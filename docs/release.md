# Release Process

How to cut a new release of `semp-workflow` (Go implementation).

---

## Versioning Scheme

The version is **not** stored in a source file — it is stamped into the binary
at build time via linker flags:

```bash
go build -ldflags "-X main.version=0.4.0+20260520" ./cmd/semp-workflow
```

`main.version` defaults to `0.0.0-dev` for unversioned `go run` / plain builds.
The `build` task stamps `SEMP_WF_VERSION` when it is set; otherwise it derives
the version from the exact git tag (`git describe --tags --exact-match`), so a
tag-triggered CI build stamps the tag automatically, and falls back to
`0.0.0-dev` when the checkout is not on a tag.

Format: `MAJOR.MINOR.PATCH+YYYYMMDD`

- `MAJOR.MINOR.PATCH` follows [Semantic Versioning](https://semver.org/):
  - **MAJOR** — breaking changes to config/template format or CLI
  - **MINOR** — new modules, templates, or backwards-compatible features
  - **PATCH** — bug fixes, documentation
- `+YYYYMMDD` is the build date. Bump it on any re-build, even without a version bump.

Git tags use the format `vMAJOR.MINOR.PATCH` (no date, no `+` suffix):

```
v0.2.1
v0.2.2
v0.3.0
```

---

## Release Checklist

All commands run from the repo root unless noted.

### 1. Run the test suite

```bash
go test ./...
# or: ./scripts/dev.sh test
```

All tests must pass. If a broker is available, also run the integration suite:

```bash
SEMP_HOST=... SEMP_USERNAME=... SEMP_PASSWORD=... SEMP_MSG_VPN=... \
  go test -tags integration ./...
```

### 2. Run vet, lint, and the vulnerability scan

```bash
./scripts/dev.sh vet lint scan
```

### 3. Regenerate documentation

If any modules or parameters changed, regenerate the module reference from the
binary:

```bash
go run ./cmd/semp-workflow list-modules --output docs/all-modules.md
```

Also update counts in:
- [README.md](../README.md) -- "All N built-in modules"
- [docs/HOWTO.md](HOWTO.md) + [docs/HOWTO-zh.md](HOWTO-zh.md) — module tables
- [docs/TESTS.md](TESTS.md) — test count, module count, registry table

### 4. Build the release binary

```bash
SEMP_WF_VERSION="0.4.0+20260520" ./scripts/dev.sh build
# host build -> dist/semp-workflow-<os>-<arch>[.exe], e.g. dist/semp-workflow-linux-amd64
# (Windows: .\scripts\dev.ps1 build -> dist\semp-workflow-windows-amd64.exe)
```

Smoke-test the binary (the name carries the host os/arch -- adjust the suffix):

```bash
./dist/semp-workflow-linux-amd64 --version
# semp-workflow, version 0.4.0+20260520
./dist/semp-workflow-linux-amd64 list-modules
```

### 5. Commit the release

```bash
git add docs/all-modules.md README.md docs/HOWTO.md docs/HOWTO-zh.md docs/TESTS.md
git commit -m "Release v0.4.0"
```

### 6. Tag the release

```bash
git tag -a v0.4.0 -m "Release v0.4.0"
git push origin main
git push origin v0.4.0
```

### 7. The tag pipeline publishes the release automatically

Do NOT create the release or attach binaries by hand. Pushing the tag triggers
[.github/workflows/tag.yml](../.github/workflows/tag.yml). It runs a `plan` job
to resolve the project shape, then the gates
([.github/workflows/ci.yml](../.github/workflows/ci.yml): `./scripts/dev.sh all
scan` -- build, vet, test, and the `govulncheck` scan) on ubuntu-24.04 and
windows-2025, then cross-compiles one binary per entry in the `BUILD_TARGETS`
repo variable. Only if every gate passed and at least one binary was produced
does the final job create the release, attach the binaries and a
`SHA256SUMS.txt`, and generate release notes. It is all-or-nothing: a failing
gate -- a test, vet, or a known, reachable vulnerability in the tagged source --
leaves no release and nothing public to clean up; the run's `logs-*` artifact is
the evidence.

The target list lives in a repo variable, not the workflow -- set it once
before the first tag, or the `plan` job fails fast:

```bash
gh variable set BUILD_TARGETS --body '[{"os":"linux","arch":"amd64"},{"os":"linux","arch":"arm64"},{"os":"darwin","arch":"amd64"},{"os":"darwin","arch":"arm64"},{"os":"windows","arch":"amd64"},{"os":"windows","arch":"arm64"}]'
```

`build` stamps `main.version` from the exact git tag when `SEMP_WF_VERSION` is
unset, so a released binary reports its version as e.g. `v0.4.0` -- distinct
from the local `MAJOR.MINOR.PATCH+YYYYMMDD` convention above, which applies only
to hand-built binaries with `SEMP_WF_VERSION` set. Downloaders verify integrity
with `sha256sum --ignore-missing -c SHA256SUMS.txt`.

To build a target locally -- offline, or to sanity-check before tagging -- set
`TARGET_OS`/`TARGET_ARCH` on the `build` task (unset builds for the host). This
is the same task CI drives per matrix leg; it uploads nothing. One target per
invocation -- loop for the full set:

```bash
TARGET_OS=linux   TARGET_ARCH=arm64 SEMP_WF_VERSION="0.4.0+20260520" ./scripts/dev.sh build
# -> dist/semp-workflow-linux-arm64
TARGET_OS=windows TARGET_ARCH=amd64 SEMP_WF_VERSION="0.4.0+20260520" ./scripts/dev.sh build
# -> dist/semp-workflow-windows-amd64.exe
```

To reproduce the CI gate locally before tagging, run `./scripts/dev.sh all scan`
(build, vet, test, govulncheck -- exactly what the gates job runs on both OSes),
or `./scripts/dev.sh full` for the complete pre-tag sweep (adds cov, image, and
graphify; image warn-skips with no Dockerfile). Run `./scripts/dev.sh lint` too
if golangci-lint is installed.

---

## Tag Naming Conventions

- Correct: `v0.4.0`
- Incorrect: `0.4.0`, `v.0.4.0`, `release-0.4.0`

One past tag (`v.0.2.2`) uses an unusual `v.` prefix — leave it as-is for historical continuity, but use `v0.X.Y` going forward.

---

## Common Commands

| Purpose | Command |
|---|---|
| List all tags | `git tag --list` |
| Show tag details | `git show v0.4.0` |
| Delete a local tag (mistyped) | `git tag -d v0.4.0` |
| Delete a remote tag | `git push origin --delete v0.4.0` |
| Check built version | `./dist/semp-workflow-<os>-<arch> --version` |
| Rebuild (host) | `SEMP_WF_VERSION="0.4.0+20260520" ./scripts/dev.sh build` |
| Cross-compile one target | `TARGET_OS=linux TARGET_ARCH=arm64 ./scripts/dev.sh build` |
