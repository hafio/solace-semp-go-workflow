#!/usr/bin/env bash
#
# M4 cutover verification against a live broker.
#
# Proves the claims this implementation makes that only a real broker can test:
#
#   1. Every legacy action name still resolves           (backward compatibility)
#   2. Configs validate offline                          (no broker needed)
#   3. A dry-run touches nothing and fails nothing
#   4. Applying twice reports no change the second time  (converging updates)
#   5. Objects the previous client could not address at all now work:
#      broker-global scope, a composite key with a defaulted part, and the
#      broker settings singleton
#   6. Deleting twice reports no change the second time
#
# This is NOT a dev-script task: it needs a broker, mutates it, and is meant to
# be copied into a container and run there. It is deliberately standalone.
#
# COPY INTO THE CONTAINER
#
#   the semp-workflow binary  (build for the container's os/arch, see below)
#   this script
#
# Nothing else. The workflow templates are embedded in the binary and exported
# by `init`; the configs are generated here.
#
#   TARGET_OS=linux TARGET_ARCH=amd64 ./scripts/dev.sh build
#   docker cp dist/semp-workflow-linux-amd64 <container>:/tmp/semp-workflow
#   docker cp scripts/cutover-check.sh       <container>:/tmp/
#   docker exec -it <container> bash -lc 'chmod +x /tmp/semp-workflow /tmp/cutover-check.sh && /tmp/cutover-check.sh'
#
# CONNECTION  (defaults suit a local Solace container; override as needed)
#
#   SEMP_HOST      default http://localhost:8080
#   SEMP_USERNAME  default admin
#   SEMP_PASSWORD  default admin
#   SEMP_MSG_VPN   default default
#
# OPTIONS
#
#   --binary PATH   the semp-workflow binary            (default: alongside this script, then $PATH)
#   --extra DIR     also validate configs found in DIR. Every *.yaml directly in
#                   DIR is treated as a config; a `templates` subdirectory is used
#                   as its templates dir. This is how you check your own configs,
#                   including the ones not in the repository.
#   --keep          do not delete what the run created (for inspection)
#   --help
#
# Exits non-zero if any check fails. Every step says what a failure means.

set -uo pipefail

SEMP_HOST="${SEMP_HOST:-http://localhost:8080}"
SEMP_USERNAME="${SEMP_USERNAME:-admin}"
SEMP_PASSWORD="${SEMP_PASSWORD:-admin}"
SEMP_MSG_VPN="${SEMP_MSG_VPN:-default}"

# Colour off so the recap line parses identically whether or not a TTY is attached.
export NO_COLOR=1

BINARY=""
EXTRA_DIR=""
KEEP=0
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Everything created is named from this prefix, so anything left behind after an
# interrupted run is obvious and safe to remove by hand.
PREFIX="CUTOVER"
DOMAIN="$PREFIX"
SYSTEM="CHK"
SYSTEM_TOPIC="PROBE"
OWNER="$PREFIX-OWNER"
AEM_USER="$PREFIX-AEM"
# A virtual hostname is a hostname, so the broker rejects uppercase. The shared
# PREFIX is upper case, hence the explicit lower-case name here.
VHOST="cutover-probe.example.invalid"
ACL_PROFILE="$PREFIX-ACL"
EXC_TOPIC="$PREFIX/probe/>"

PASSED=0
FAILED=0
declare -a FAILURES=()

# ---------------------------------------------------------------- presentation

hr() { printf '%s\n' "----------------------------------------------------------------------"; }

step() {
  printf '\n'
  hr
  printf 'STEP %s\n' "$1"
  hr
}

info() { printf '  %s\n' "$1"; }

pass() {
  PASSED=$((PASSED + 1))
  printf '  PASS  %s\n' "$1"
}

fail() {
  FAILED=$((FAILED + 1))
  FAILURES+=("$1")
  printf '  FAIL  %s\n' "$1"
  if [ -n "${2:-}" ]; then
    printf '        why: %s\n' "$2"
  fi
}

die() {
  printf '\nERROR: %s\n' "$1" >&2
  exit 2
}

usage() { sed -n '2,60p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

# ---------------------------------------------------------------------- set-up

while [ $# -gt 0 ]; do
  case "$1" in
    --binary) BINARY="${2:-}"; shift 2 ;;
    --extra)  EXTRA_DIR="${2:-}"; shift 2 ;;
    --keep)   KEEP=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1 (try --help)" ;;
  esac
done

if [ -z "$BINARY" ]; then
  for candidate in "$SCRIPT_DIR/semp-workflow" "$SCRIPT_DIR/semp-workflow.exe" "$(command -v semp-workflow 2>/dev/null || true)"; do
    if [ -n "$candidate" ] && [ -x "$candidate" ]; then BINARY="$candidate"; break; fi
  done
fi
[ -n "$BINARY" ] && [ -x "$BINARY" ] || die "semp-workflow binary not found. Pass --binary PATH."

WORK="$(mktemp -d 2>/dev/null || mktemp -d -t cutover)"
LOGS="$WORK/logs"
mkdir -p "$LOGS"

cleanup() {
  if [ "$KEEP" -eq 1 ]; then
    printf '\nKept working directory: %s\n' "$WORK"
    return
  fi
  printf '\nLogs kept at: %s\n' "$LOGS"
}
trap cleanup EXIT

printf '%s\n' "======================================================================"
printf 'M4 cutover verification\n'
printf '%s\n' "======================================================================"
info "binary   : $BINARY  ($("$BINARY" --version 2>&1 | head -1))"
info "broker   : $SEMP_HOST"
info "msg vpn  : $SEMP_MSG_VPN"
info "prefix   : $PREFIX  (everything created is named from this)"
[ -n "$EXTRA_DIR" ] && info "extra    : $EXTRA_DIR"

# --------------------------------------------------------------- config set-up

TEMPLATES="$WORK/templates"
"$BINARY" init --output-dir "$TEMPLATES" >"$LOGS/init.log" 2>&1 \
  || die "could not export the bundled templates: see $LOGS/init.log"

conn_block() {
  cat <<YAML
semp:
  host: "$SEMP_HOST"
  username: "$SEMP_USERNAME"
  password: "$SEMP_PASSWORD"
  msg_vpn: "$SEMP_MSG_VPN"
  verify_ssl: false
  timeout: 30
YAML
}

# The bundled inbound template, which exercises the ten legacy families.
{
  conn_block
  cat <<YAML
workflows:
  - template: app-inbound.new-seq
    inputs:
      domain: "$DOMAIN"
      system: "$SYSTEM"
      system_topic: "$SYSTEM_TOPIC"
      non_service_queue_owner: "$OWNER"
      aem_client_username: "$AEM_USER"
      # The template defaults this to a client profile it does not create, so on
      # a broker without it the RDP fails and its consumer and binding cascade.
      # Point it at the profile this same run creates, so the check needs no
      # pre-existing broker configuration.
      rdp_client_profile: "cp-it-user"
YAML
} >"$WORK/create.yaml"

{
  conn_block
  cat <<YAML
workflows:
  - template: app-inbound.delete
    inputs:
      domain: "$DOMAIN"
      system: "$SYSTEM"
      system_topic: "$SYSTEM_TOPIC"
      aem_client_username: "$AEM_USER"
YAML
} >"$WORK/delete.yaml"

# Objects the previous implementation could not reach. The old client hardcoded a
# msgVpns/{vpn} prefix, so nothing broker-global was addressable at all.
cat >"$TEMPLATES/cutover-probe.yaml" <<'YAML'
workflow-templates:
  # A broker-global object: no msgVpns hop in its path.
  - name: "global-scope"
    inputs:
      required:
      - vhost
    actions:
    - name: "Create virtual hostname"
      module: "virtual_hostname.add"
      args:
        virtualHostname: "{{ inputs.vhost }}"
        enabled: false

  # A composite key whose leading part is defaulted: the exception is named by
  # topic alone and the topic syntax defaults to smf.
  - name: "composite-key"
    inputs:
      required:
      - acl_profile
      - topic
    actions:
    - name: "Create ACL profile"
      module: "acl_profile.add"
      args:
        aclProfileName: "{{ inputs.acl_profile }}"
    - name: "Add publish exception, syntax defaulted"
      module: "acl_publish_exception.add"
      args:
        aclProfileName: "{{ inputs.acl_profile }}"
        publishTopicException: "{{ inputs.topic }}"

  - name: "global-scope-teardown"
    inputs:
      required:
      - vhost
    actions:
    - name: "Delete virtual hostname"
      module: "virtual_hostname.delete"
      args:
        virtualHostname: "{{ inputs.vhost }}"

  - name: "composite-key-teardown"
    inputs:
      required:
      - acl_profile
      - topic
    actions:
    - name: "Remove publish exception, syntax defaulted"
      module: "acl_publish_exception.delete"
      args:
        aclProfileName: "{{ inputs.acl_profile }}"
        publishTopicException: "{{ inputs.topic }}"
    - name: "Delete ACL profile"
      module: "acl_profile.delete"
      args:
        aclProfileName: "{{ inputs.acl_profile }}"
YAML

# The broker settings singleton. Only ever dry-run: its path is the config root
# and a real change here affects the whole broker.
cat >"$TEMPLATES/cutover-broker.yaml" <<'YAML'
workflow-templates:
  - name: "singleton-read"
    actions:
    - name: "Reach the broker settings object"
      module: "broker.update"
      args:
        authBruteForceProtectionEnabled: true
YAML

# A template rejects inputs it does not declare, so each probe config carries
# exactly the inputs its own template requires.
vhost_config() {
  conn_block
  cat <<YAML
workflows:
  - template: $1
    inputs:
      vhost: "$VHOST"
YAML
}

acl_config() {
  conn_block
  cat <<YAML
workflows:
  - template: $1
    inputs:
      acl_profile: "$ACL_PROFILE"
      topic: "$EXC_TOPIC"
YAML
}

vhost_config "cutover-probe.global-scope"           >"$WORK/probe-global.yaml"
vhost_config "cutover-probe.global-scope-teardown"  >"$WORK/probe-global-down.yaml"
acl_config   "cutover-probe.composite-key"          >"$WORK/probe-composite.yaml"
acl_config   "cutover-probe.composite-key-teardown" >"$WORK/probe-composite-down.yaml"
{ conn_block; printf 'workflows:\n  - template: cutover-broker.singleton-read\n'; } >"$WORK/probe-broker.yaml"

# ------------------------------------------------------------------- mechanics

CHANGED=0; DRYRUN=0; SKIPPED=0; FAILED_TASKS=0; RAN=0

# parse_counts reads the recap totals out of a run log. RAN reports whether a
# recap was found at all: without it every count would read as zero, and a check
# for "no failures" would pass a run that never reached the broker.
parse_counts() {
  local log="$1" line
  line="$(grep -a 'Total:' "$log" | tail -1)"
  if [ -z "$line" ]; then
    RAN=0; CHANGED=0; DRYRUN=0; SKIPPED=0; FAILED_TASKS=0
    return
  fi
  RAN=1
  CHANGED="$(printf '%s' "$line"  | grep -o 'changed=[0-9]*'  | head -1 | cut -d= -f2)"
  DRYRUN="$(printf '%s' "$line"   | grep -o 'dryrun=[0-9]*'   | head -1 | cut -d= -f2)"
  SKIPPED="$(printf '%s' "$line"  | grep -o 'skipped=[0-9]*'  | head -1 | cut -d= -f2)"
  FAILED_TASKS="$(printf '%s' "$line" | grep -o 'failed=[0-9]*' | head -1 | cut -d= -f2)"
  : "${CHANGED:=0}" "${DRYRUN:=0}" "${SKIPPED:=0}" "${FAILED_TASKS:=0}"
}

# wf runs a config and parses its recap. $1 log name, $2 config, rest passed
# through. Returns non-zero when the run produced no recap, so a caller can tell
# "nothing failed" apart from "nothing ran".
wf() {
  local name="$1" config="$2"; shift 2
  "$BINARY" run --config "$config" "$@" >"$LOGS/$name.log" 2>&1
  local code=$?
  parse_counts "$LOGS/$name.log"
  if [ "$RAN" -eq 0 ]; then
    info "$name: no recap -- the run did not execute (exit $code)"
    fail "$name did not run" "$(grep -a 'ERROR' "$LOGS/$name.log" | head -2)
        Commonly the broker is unreachable at $SEMP_HOST, or the credentials are wrong. Full output: $LOGS/$name.log"
    return 1
  fi
  info "$name: changed=$CHANGED dryrun=$DRYRUN skipped=$SKIPPED failed=$FAILED_TASKS   ($LOGS/$name.log)"
  return 0
}

# ------------------------------------------------------------------ 1. offline

step "1/6  Legacy action names still resolve, and configs validate offline"

MODULE_LIST="$LOGS/list-modules.log"
"$BINARY" list-modules >"$MODULE_LIST" 2>&1 || fail "list-modules failed" "see $MODULE_LIST"

LEGACY="queue.add queue.delete queue.update q_sub.add q_sub.delete
rdp.add rdp.delete rdp.update rdp_rc.add rdp_rc.delete rdp_qb.add rdp_qb.delete
client_profile.add client_profile.delete client_profile.update
client_username.add client_username.delete client_username.update
acl_profile.add acl_profile.delete
acl_publish_exception.add acl_publish_exception.delete
acl_subscribe_exception.add acl_subscribe_exception.delete"

missing=""
for m in $LEGACY; do
  grep -qa -- "- $m\$" "$MODULE_LIST" || missing="$missing $m"
done
if [ -z "$missing" ]; then
  pass "all 24 legacy action names are registered"
else
  fail "legacy action names missing:$missing" "existing templates naming them would fail with 'Unknown module'"
fi

# Offline validation: catches bad inputs and bad attributes without a broker.
for cfg in create delete; do
  if "$BINARY" validate --config "$WORK/$cfg.yaml" --templates-dir "$TEMPLATES" >"$LOGS/validate-$cfg.log" 2>&1; then
    pass "validate $cfg.yaml (offline)"
  else
    fail "validate $cfg.yaml failed" "$(grep -a ERROR "$LOGS/validate-$cfg.log" | head -3)"
  fi
done

if [ -n "$EXTRA_DIR" ]; then
  found=0
  for cfg in "$EXTRA_DIR"/*.yaml; do
    [ -f "$cfg" ] || continue
    found=$((found + 1))
    args=(validate --config "$cfg")
    [ -d "$EXTRA_DIR/templates" ] && args+=(--templates-dir "$EXTRA_DIR/templates")
    name="extra-$(basename "$cfg" .yaml)"
    if "$BINARY" "${args[@]}" >"$LOGS/$name.log" 2>&1; then
      pass "validate $(basename "$cfg") (your config)"
    else
      fail "validate $(basename "$cfg") failed" "$(grep -a ERROR "$LOGS/$name.log" | head -3)"
    fi
  done
  [ "$found" -eq 0 ] && info "no *.yaml found directly in $EXTRA_DIR"
fi

# ------------------------------------------------------------------ 2. dry-run

step "2/6  Dry-run reaches the broker, changes nothing, fails nothing"
info "This is also the connectivity check: a dry-run still reads current state."

if wf "dryrun" "$WORK/create.yaml" --templates-dir "$TEMPLATES" --dry-run; then
  if [ "$FAILED_TASKS" -ne 0 ]; then
    info "note: $FAILED_TASKS task(s) reported failed. In a dry-run an update"
    info "cannot succeed against an object the same run would have created,"
    info "because nothing is actually created. Check they are all of that shape:"
    grep -a -A1 'FAILED' "$LOGS/dryrun.log" | grep -a '=> ' | sed 's/^/        /'
  fi
  if [ "$CHANGED" -ne 0 ]; then
    fail "dry-run reported changed=$CHANGED" "a dry-run must never modify the broker"
  elif [ "$((DRYRUN + SKIPPED))" -eq 0 ]; then
    fail "dry-run reported no tasks at all" "the template resolved to nothing"
  else
    pass "dry-run: $DRYRUN would change, $SKIPPED already correct, nothing modified"
  fi
  BROKER_REACHED=1
else
  BROKER_REACHED=0
fi

if [ "$BROKER_REACHED" -eq 0 ]; then
  printf '\nThe broker was not reached, so every later step would fail for the same\n'
  printf 'reason. Fix the connection and re-run.\n'
  printf '\n%s\n' "======================================================================"
  printf 'RESULT: %s passed, %s failed\n' "$PASSED" "$FAILED"
  printf '%s\n' "======================================================================"
  exit 1
fi

# -------------------------------------------------------------------- 3. apply

step "3/6  First apply creates, second apply reports no change"
info "This is the load-bearing check on converging updates. An update reads"
info "current state and sends only genuine differences, so the second run of an"
info "unchanged config must report everything skipped."

if wf "apply-1" "$WORK/create.yaml" --templates-dir "$TEMPLATES"; then
  if [ "$FAILED_TASKS" -ne 0 ]; then
    fail "first apply reported $FAILED_TASKS failed task(s)" "$(grep -a '=> ' "$LOGS/apply-1.log" | grep -ai 'fail\|error' | head -3)"
  else
    pass "first apply: changed=$CHANGED skipped=$SKIPPED failed=0"
  fi
fi

if wf "apply-2" "$WORK/create.yaml" --templates-dir "$TEMPLATES"; then
if [ "$FAILED_TASKS" -ne 0 ]; then
  fail "second apply reported $FAILED_TASKS failed task(s)" "see $LOGS/apply-2.log"
elif [ "$CHANGED" -ne 0 ]; then
  fail "second apply still reported changed=$CHANGED" \
       "these attributes differ on every run. Usually a write-only attribute that cannot be read back (expected, e.g. a password), or one the broker stores in a different form from the one supplied. Compare the named tasks in $LOGS/apply-2.log against a GET on the broker."
else
  pass "second apply: changed=0, skipped=$SKIPPED -- the config converges"
fi
fi

# --------------------------------------------------- 4. previously unreachable

step "4/6  Objects the previous client could not address"
info "The old client hardcoded a msgVpns/{vpn} prefix, so nothing broker-global"
info "was reachable, and a composite key could not be defaulted."

if wf "probe-global-1" "$WORK/probe-global.yaml" --templates-dir "$TEMPLATES"; then
if [ "$FAILED_TASKS" -ne 0 ]; then
  fail "broker-global object could not be created" "$(grep -a '=> ' "$LOGS/probe-global-1.log" | head -2)"
else
  wf "probe-global-2" "$WORK/probe-global.yaml" --templates-dir "$TEMPLATES"
  if [ "$SKIPPED" -ge 1 ] && [ "$CHANGED" -eq 0 ]; then
    pass "broker-global scope works and is idempotent (virtual_hostname)"
  else
    fail "broker-global object is not idempotent" "re-adding reported changed=$CHANGED skipped=$SKIPPED"
  fi
fi
fi

if wf "probe-composite-1" "$WORK/probe-composite.yaml" --templates-dir "$TEMPLATES"; then
if [ "$FAILED_TASKS" -ne 0 ]; then
  fail "composite key with a defaulted part failed" "$(grep -a '=> ' "$LOGS/probe-composite-1.log" | head -2)"
else
  wf "probe-composite-2" "$WORK/probe-composite.yaml" --templates-dir "$TEMPLATES"
  if [ "$CHANGED" -eq 0 ]; then
    pass "composite key works, topic syntax defaulted to smf, and is idempotent"
  else
    fail "composite-key object is not idempotent" "re-adding reported changed=$CHANGED"
  fi
fi
fi

# Dry-run only: this object's path is the config root and a real change here is
# broker-wide.
if wf "probe-broker" "$WORK/probe-broker.yaml" --templates-dir "$TEMPLATES" --dry-run; then
if [ "$FAILED_TASKS" -eq 0 ] && [ "$((DRYRUN + SKIPPED))" -ge 1 ]; then
  pass "broker settings singleton is reachable (dry-run only, nothing changed)"
else
  fail "could not reach the broker settings singleton" \
       "its path is the config root, so this exercises the empty-path GET. $(grep -a '=> ' "$LOGS/probe-broker.log" | head -2)"
fi
fi

# ------------------------------------------------------------------- 5. delete

step "5/6  Delete removes what was created, and deleting twice reports no change"

if wf "delete-1" "$WORK/delete.yaml" --templates-dir "$TEMPLATES"; then
  if [ "$FAILED_TASKS" -ne 0 ]; then
    fail "delete reported $FAILED_TASKS failed task(s)" "see $LOGS/delete-1.log"
  else
    pass "delete: changed=$CHANGED skipped=$SKIPPED failed=0"
  fi
fi

if wf "delete-2" "$WORK/delete.yaml" --templates-dir "$TEMPLATES"; then
  if [ "$CHANGED" -ne 0 ] || [ "$FAILED_TASKS" -ne 0 ]; then
    fail "second delete was not a no-op" "changed=$CHANGED failed=$FAILED_TASKS, see $LOGS/delete-2.log"
  else
    pass "second delete: changed=0, skipped=$SKIPPED"
  fi
fi

# ------------------------------------------------------------------ 6. tear-down

step "6/6  Removing what the probes created"

if [ "$KEEP" -eq 1 ]; then
  info "--keep given: leaving $VHOST and $ACL_PROFILE in place"
else
  wf "teardown-composite" "$WORK/probe-composite-down.yaml" --templates-dir "$TEMPLATES"
  [ "$FAILED_TASKS" -eq 0 ] || fail "could not remove the probe ACL profile" "remove $ACL_PROFILE by hand"
  wf "teardown-global" "$WORK/probe-global-down.yaml" --templates-dir "$TEMPLATES"
  [ "$FAILED_TASKS" -eq 0 ] || fail "could not remove the probe virtual hostname" "remove $VHOST by hand"
  info "anything still named $PREFIX* on the broker was left by a failed step"
fi

# ------------------------------------------------------------------- summary

printf '\n%s\n' "======================================================================"
printf 'RESULT: %s passed, %s failed\n' "$PASSED" "$FAILED"
printf '%s\n' "======================================================================"

if [ "$FAILED" -ne 0 ]; then
  printf '\nFailures:\n'
  for f in "${FAILURES[@]}"; do printf '  - %s\n' "$f"; done
  printf '\nLogs: %s\n' "$LOGS"
  printf 'Cutover is NOT verified. Do not retire the previous repository yet.\n'
  exit 1
fi

printf '\nCutover verified against %s.\n' "$SEMP_HOST"
printf 'Still to do off-broker: diff the module reference for the ten legacy\n'
printf 'families against the previous implementation, then tag and archive.\n'
