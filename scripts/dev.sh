#!/usr/bin/env bash
#
# dev.sh -- developer / CI task runner for the Go implementation of
# SEMP Workflow Automation. Mirrors dev.ps1 exactly (same tasks, same flags).
#
# Tasks:
#   build   go build -> dist/semp-workflow-<os>-<arch>[.exe]   (fatal)
#   vet     go vet ./...                                       (fatal)
#   lint    golangci-lint run                                  (fatal)
#   test    go test ./... (unit)                               (fatal)
#   integration  go test -tags integration ./...               (fatal; needs live broker)
#   cov     coverage profile -> func total + HTML              (fatal)
#   scan    govulncheck ./... (image scan N/A -- no Dockerfile)(fatal)
#   image   container image build                    (warn-skip: no Dockerfile here)
#   graphify  python -m graphify update .          (local only; report-only)
#   size    build + go-size-analyzer size breakdown            (report-only)
# Aggregates:
#   all     build vet test                       (fast inner loop; CI runs `all scan`)
#   full    all + cov + image + scan + graphify   (pre-tag sweep)
# (integration is opt-in -- never part of all/full, so broker-free runs pass.)
#
# Every task closes with a footer line -- echoed to the console and appended to
# that task's log:  <ISO-8601 w/ offset> | <task> | <duration>s | OK|FAILED (exit N)
#
# Runs from any cwd. Logs land in <scriptdir>/logs/<task>.log (plain text).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"          # the Go module root (= repo root)
PROJECT_ROOT="$GO_ROOT"                          # repo root (graphify graphs this)
LOG_DIR="$SCRIPT_DIR/logs"
DIST_DIR="$GO_ROOT/dist"
BIN_NAME="semp-workflow"                          # base name; build appends -<os>-<arch>[.exe]
COVERAGE_OUT="$GO_ROOT/coverage.out"
COVERAGE_HTML="$GO_ROOT/coverage.html"

VERSION="${SEMP_WF_VERSION:-$(git describe --tags --exact-match 2>/dev/null || echo 0.0.0-dev)}"

# Tools that honor NO_COLOR emit clean text into the logs; static build by default.
export NO_COLOR=1
export CGO_ENABLED="${CGO_ENABLED:-0}"

# Toolchain parity with CI: when go.mod carries a `toolchain` directive it is the
# single pin, and GOTOOLCHAIN makes any go binary honour it exactly. Set only
# when unset, so an exported value wins.
if [ -z "${GOTOOLCHAIN:-}" ] && [ -f "$GO_ROOT/go.mod" ]; then
  t="$(sed -n 's/^toolchain //p' "$GO_ROOT/go.mod")"
  [ -n "$t" ] && export GOTOOLCHAIN="$t"
fi

mkdir -p "$LOG_DIR"

# ---- console helpers (colored; not captured to logs) ----------------------
c_reset=$'\033[0m'; c_blue=$'\033[1;34m'; c_green=$'\033[1;32m'
c_yellow=$'\033[1;33m'; c_red=$'\033[1;31m'
step() { printf '%s==>%s %s\n' "$c_blue" "$c_reset" "$*"; }
ok()   { printf '%s OK %s %s\n' "$c_green" "$c_reset" "$*"; }
warn() { printf '%s ! %s %s\n' "$c_yellow" "$c_reset" "$*"; }
die()  { printf '%s XX %s %s\n' "$c_red" "$c_reset" "$*" >&2; exit 1; }

# run_logged <task> <workdir> <cmd...>
# Truncates the task log with a header, then tees combined stdout+stderr while
# cleaning it so the log stays readable plain ASCII: strips CSI escape sequences
# (spinner/color) and folds the Unicode box-drawing characters and micro sign a
# tool may emit (e.g. go-size-analyzer's table + "us" timings) to ASCII (-,|,+,u).
# LC_ALL=C makes sed operate byte-wise so the \xHH box-drawing sequences match on
# both GNU and BSD sed. The folds are inert for ASCII-only tools. Returns the
# command's exit code (not sed/tee's), via PIPESTATUS.
run_logged() {
  local task="$1" workdir="$2"; shift 2
  local log="$LOG_DIR/$task.log"
  {
    printf '=== %s ===\n' "$task"
    printf 'dir: %s\n' "$workdir"
    printf 'cmd: %s\n\n' "$*"
  } > "$log"
  ( cd "$workdir" && "$@" 2>&1 ) \
    | LC_ALL=C sed -E $'s/\x1b\\[[0-9;?]*[a-zA-Z]//g; s/\xe2\x94\x80/-/g; s/\xe2\x94\x82/|/g; s/\xe2[\x94\x95][\x80-\xbf]/+/g; s/\xc2\xb5/u/g' \
    | tee -a "$log"
  return "${PIPESTATUS[0]}"
}

# _run <task> executes task_<task>, then closes it with the S7 footer -- echoed
# to the console and appended to that task's log -- whatever the outcome. Fatal
# tasks abort the whole run on a non-zero code; report-only tasks (graphify,
# size) record FAILED in the footer but let the run continue. Task functions
# therefore `return` a code instead of exiting, so the footer always lands.
_run() {
  local task="$1" start code dur ts status footer
  start="$(date +%s)"
  "task_$task"
  code=$?
  dur=$(( $(date +%s) - start ))
  ts="$(date +%Y-%m-%dT%H:%M:%S%z)"
  if [ "$code" -eq 0 ]; then status="OK"; else status="FAILED (exit $code)"; fi
  footer="$ts | $task | ${dur}s | $status"
  printf '%s\n' "$footer"
  printf '%s\n' "$footer" >> "$LOG_DIR/$task.log"
  if [ "$code" -ne 0 ] && ! _report_only "$task"; then
    exit "$code"
  fi
  return 0
}

# _report_only names the tasks that record FAILED but never abort the run.
_report_only() { case "$1" in graphify|size) return 0 ;; *) return 1 ;; esac; }

# _bin_path <os> <arch> echoes the dist/ output path for a target: the base name
# plus -<os>-<arch> plus the platform extension (.exe on Windows). The target is
# always in the name because the release job merges every cross-compile leg into
# one directory with merge-multiple -- a bare name would let one arch overwrite
# another and ship a single architecture under several names.
_bin_path() {
  local os="$1" arch="$2" ext=""
  [ "$os" = "windows" ] && ext=".exe"
  printf '%s/%s-%s-%s%s' "$DIST_DIR" "$BIN_NAME" "$os" "$arch" "$ext"
}

# load_local_env sources the gitignored scripts/local.env -- the ONLY source of the
# broker connection for the `integration` task (no environment fallback). `set -a`
# (allexport) marks every sourced assignment for export so the values reach the
# `go test` child process; a plain source would leave them as non-exported shell
# vars. The file is required: absent it we fail (return non-zero) with instructions
# rather than run a skip-only suite that looks like a pass -- and, because this
# returns instead of exiting, task_integration still emits its footer. Because bash
# sources the file, values with spaces or shell metacharacters must be quoted in it.
load_local_env() {
  local file="$SCRIPT_DIR/local.env"
  if [ ! -f "$file" ]; then
    warn "scripts/local.env not found -- copy scripts/local.env.example to scripts/local.env and fill in the broker connection"
    return 1
  fi
  set -a
  # shellcheck source=/dev/null
  . "$file"
  set +a
}

# ---- tasks ----------------------------------------------------------------
# task_build compiles the CLI to dist/<name>-<os>-<arch>[.exe]. TARGET_OS /
# TARGET_ARCH (set by CI's per-leg binaries matrix) select the target; unset
# means the host's own os/arch, so local and CI output are the same shape. The
# target is always in the name -- see _bin_path.
task_build() {
  local os arch out
  os="${TARGET_OS:-$(go env GOOS)}"
  arch="${TARGET_ARCH:-$(go env GOARCH)}"
  out="$(_bin_path "$os" "$arch")"
  step "build -> $out"
  mkdir -p "$DIST_DIR"
  run_logged build "$GO_ROOT" \
    env GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
    -o "$out" ./cmd/semp-workflow \
    || return
  ok "build -> $out"
}

task_gen() {
  step "gen (module descriptors from specs/, then the module reference)"
  run_logged gen "$GO_ROOT" go generate ./internal/modules/... || return
  # The reference is rendered from the registry, so it has to run after the
  # descriptors are regenerated and compiled. Both outputs are committed.
  run_logged gen "$GO_ROOT" go run ./cmd/semp-workflow list-modules --output-dir docs/modules || return
  ok "gen"
}

task_gencheck() {
  step "gencheck (descriptors match specs/)"
  run_logged gencheck "$GO_ROOT" go run ./tools/semp-gen     -software specs/software/10.26.0.8755/semp-v2-config.json     -appliance specs/appliance/10.26.0.8755/semp-v2-config.json     -out internal/modules -check || return
  ok "gencheck"
}

task_vet() {
  step "vet"
  run_logged vet "$GO_ROOT" go vet ./... || return
  ok "vet"
}

task_lint() {
  step "lint (golangci-lint)"
  if ! command -v golangci-lint >/dev/null 2>&1; then
    warn "golangci-lint not installed; skipping (install: https://golangci-lint.run)"
    return 0
  fi
  # golangci-lint v2 dropped --no-color; NO_COLOR=1 (exported above) disables color.
  run_logged lint "$GO_ROOT" golangci-lint run ./... || return
  ok "lint"
}

task_test() {
  step "test (unit)"
  # -count=1 disables Go's test result cache so every run re-executes the whole
  # suite -- a cached PASS can otherwise hide a regression or stale coverage.
  run_logged test "$GO_ROOT" go test -count=1 ./... || return
  ok "test"
}

# task_integration runs the //go:build integration suites (module/engine/cli
# lifecycles) against a live broker. Connection values come solely from the
# gitignored scripts/local.env, which load_local_env sources (and requires --
# it fails the task if the file is absent). Building with the tag always compiles
# the integration files, so this catches compile errors even against an
# unreachable broker.
task_integration() {
  step "integration (-tags integration; live broker)"
  load_local_env || return
  # -count=1: never cache. -tags integration compiles+runs the broker-gated suites.
  run_logged integration "$GO_ROOT" go test -count=1 -tags integration ./... || return
  ok "integration"
}

task_cov() {
  step "cov"
  # -count=1: never cache -- coverage must reflect a real run of every package.
  run_logged cov "$GO_ROOT" \
    go test -count=1 -covermode=atomic -coverprofile="$COVERAGE_OUT" ./... || return
  ( cd "$GO_ROOT" && go tool cover -func="$COVERAGE_OUT" | tee -a "$LOG_DIR/cov.log" | tail -1 )
  ( cd "$GO_ROOT" && go tool cover -html="$COVERAGE_OUT" -o "$COVERAGE_HTML" )
  ok "cov -> $COVERAGE_HTML"
}

# task_scan is the single security-scan task (S7): the dependency scanner always,
# plus an image scan only where the project ships an image. This project ships
# binaries only (no Dockerfile), so scan is govulncheck alone -- fatal on findings.
# Invoked as `go tool govulncheck` so it resolves through go.mod's tool + toolchain
# pins (never `go run pkg@version`, which is module-less and ignores both).
task_scan() {
  step "scan (govulncheck; image scan N/A -- no Dockerfile)"
  run_logged scan "$GO_ROOT" go tool govulncheck ./... || return
  ok "scan"
}

# task_image builds the project's container image. This repo ships no Dockerfile
# (binaries only), so the task warn-and-skips -- shipping no image is a valid
# project shape (S7), not a misconfiguration. If a Dockerfile is added, the local
# single-arch build below runs; CI's tag.yml owns the multi-arch push.
task_image() {
  step "image"
  if [ ! -f "$GO_ROOT/Dockerfile" ]; then
    warn "no Dockerfile; skipping image build (binaries-only project)"
    return 0
  fi
  run_logged image "$GO_ROOT" docker build --progress=plain -t "$BIN_NAME:$VERSION" . || return
  ok "image"
}

# task_graphify refreshes the local knowledge graph (AST-only, no API cost). It is
# a developer artifact, so it is local-only: it warn-skips when CI is set (belt and
# braces -- CI runs `all scan`, which never reaches graphify anyway) and rides only
# in `full`, never in `all`.
task_graphify() {
  step "graphify (update graph at $PROJECT_ROOT)"
  if [ -n "${CI:-}" ]; then
    warn "CI detected; skipping graphify (local-only developer artifact)"
    return 0
  fi
  if ! command -v python >/dev/null 2>&1 && ! command -v python3 >/dev/null 2>&1; then
    warn "python not found; skipping graphify"
    return 0
  fi
  local py; py="$(command -v python || command -v python3)"
  run_logged graphify "$PROJECT_ROOT" "$py" -m graphify update . || return
  ok "graphify"
}

# task_size builds a fresh binary (identical flags to task_build, so the measured
# artifact equals what ships) and reports its size breakdown by package/module via
# go-size-analyzer (gsa). gsa attributes bytes AFTER dead-code elimination, so it
# reflects the linked binary, not pre-link archives. Pinned to a release tag and
# run via `go run` so nothing is installed globally. Report-only: a gsa failure
# only warns. `env GOEXPERIMENT=jsonv2` is required by gsa (it uses
# encoding/json/v2) and is scoped to this one command via `env` so it never leaks
# into other tasks. Fallback without gsa: `go tool nm -size -sort=size <binary>`.
task_size() {
  local os arch out
  os="${TARGET_OS:-$(go env GOOS)}"
  arch="${TARGET_ARCH:-$(go env GOARCH)}"
  out="$(_bin_path "$os" "$arch")"
  task_build || return
  step "size (go-size-analyzer)"
  run_logged size "$GO_ROOT" \
    env GOEXPERIMENT=jsonv2 go run github.com/Zxilly/go-size-analyzer/cmd/gsa@v1.13.0 \
    -f text "$out" \
    || return
  ok "size"
}

task_all()  { _run build; _run vet; _run test; }
task_full() { task_all; _run gencheck; _run cov; _run image; _run scan; _run graphify; }

usage() {
  cat <<'EOF'
Usage: dev.sh <task> [task...]

Tasks:
  build     Compile the CLI to dist/semp-workflow-<os>-<arch>[.exe]
            (TARGET_OS / TARGET_ARCH cross-compile; unset = host)
  gen       Regenerate internal/modules/gen_*.go + manifest from specs/
  gencheck  Fail if the checked-in descriptors no longer match specs/
  vet       go vet ./...
  lint      golangci-lint run
  test      go test ./... (unit)
  integration  go test -tags integration ./...  (needs a live broker; requires scripts/local.env)
  cov       Coverage profile -> printed total + coverage.html
  scan      govulncheck ./...                 (image scan N/A -- no Dockerfile)
  image     Container image build             (warn-skips -- no Dockerfile here)
  graphify  python -m graphify update .        (local only; report-only)
  size      build + go-size-analyzer breakdown (report-only)

Aggregates:
  all       build vet test                    (CI runs `all scan`)
  full      all + gencheck + cov + image + scan + graphify
  (integration is opt-in and never part of all/full)

Every task closes with a footer echoed and appended to logs/<task>.log:
  <ISO-8601 w/ offset> | <task> | <duration>s | OK|FAILED (exit N)

Env:
  SEMP_WF_VERSION   override the stamped version (unset: exact git tag, else 0.0.0-dev)
  TARGET_OS/TARGET_ARCH   cross-compile target for `build` (default: host)

  The 'integration' task reads the broker connection ONLY from a gitignored
  scripts/local.env (copy scripts/local.env.example) -- SEMP_HOST, SEMP_USERNAME,
  SEMP_PASSWORD, SEMP_MSG_VPN, and optional SEMP_VERIFY_SSL (TLS cert verify,
  default false). The task fails if the file is missing.
EOF
}

main() {
  [ "$#" -eq 0 ] && { usage; exit 0; }
  for arg in "$@"; do
    case "$arg" in
      -h|--help|help) usage; exit 0 ;;
      build|vet|lint|test|integration|cov|scan|image|graphify|size|gen|gencheck) _run "$arg" ;;
      all)      task_all ;;
      full)     task_full ;;
      *) die "unknown task: $arg (run 'dev.sh help')" ;;
    esac
  done
}

main "$@"
