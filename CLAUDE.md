# Solace SEMP Workflow Automation

Go module `solace-semp-workflow`, code at the repository root. Run all Go commands from
the repo root.

This repository supersedes `SEMP-Workflow_Automation` (24 hand-written action modules).
Here the modules are **generated from the committed SEMP schemas** in `specs/`. The old
repo is frozen and will be retired; do not backport to it.

## Project conventions

- **Never hand-edit `internal/modules/gen_*.go`.** They carry a `DO NOT EDIT` header and are
  regenerated from `specs/` by `tools/semp-gen`. Per-object logic that a descriptor cannot
  express goes in `internal/modules/overrides.go`, which is hand-written and never generated.
- Action modules are not written individually. Three generic drivers in
  `internal/modules/driver.go` (`collectionAdd`, `collectionUpdate`, `collectionDelete`)
  implement every action; each generated file supplies one `objectDesc` data literal and
  calls `registerObject`. A bug fix belongs in the driver, once -- not in 225 places.
- Registry keys are `object.verb`. Verbs are derived from the spec: `add` where the
  collection has POST, `update` where the item path has PATCH, `delete` where it has DELETE.
- Every action is idempotent and reports exactly one of four states: OK (changed),
  SKIPPED (already in the desired state), FAILED, DRYRUN. `update` diffs against the
  broker's current state and reports SKIPPED when nothing differs; Write-Only and Opaque
  attributes cannot be read back, so they are always re-sent.
- Paths passed to `semp.Client` are relative to `/SEMP/v2/config`. VPN-scoped objects
  prepend their own hop via `semp.VPNSeg(vpn)`; broker-global objects address the root
  directly; the Broker singleton uses the empty path. Build paths with `semp.Path`/`semp.Seg`
  so composite keys are comma-joined with each part encoded independently -- never with
  string concatenation.
- Secrets are never echoed. Mark any credential-bearing `ParamSpec` `Sensitive: true`;
  workflow inputs are additionally masked by name via `output.IsSensitiveName`.
- New modules need no bespoke tests. The driver is tested once per *shape* in
  `internal/modules/driver_test.go` using the shared `fakeClient`; the generator is tested
  by golden file in `tools/semp-gen/internal/render`.

## Backward compatibility

These 10 registry prefixes and their parameter names are fixed, so existing templates and
customer configs keep working: `queue`, `q_sub`, `rdp`, `rdp_rc`, `rdp_qb`,
`client_profile`, `client_username`, `acl_profile`, `acl_publish_exception`,
`acl_subscribe_exception`. They are pinned in the generator's alias table and asserted by
`internal/modules/modules_test.go`, which was carried over from the old repo -- treat a
failure there as a driver bug, not a test to update.

## Naming

Prefixes derive mechanically from the create `operationId`: strip `create`, split CamelCase,
drop a leading `Msg`+`Vpn`, lower-snake-join. A committed 24-entry alias table in
`tools/semp-gen/internal/ir/aliases.go` overrides that for the 10 legacy names, the one
genuine collision (`Proxy` vs `MsgVpnProxy` -> `broker_proxy` and `proxy`), and the 13 names
over 45 characters. Everything else takes the mechanical name, so it is predictable from the
SEMP object without consulting the table. The generator fails if two objects resolve to one
prefix or if an alias key matches no real `operationId`.

## Specs

`specs/<platform>/<version>/` holds the SEMP v2 swagger and v1 XSDs per platform, with
provenance and checksums in `specs/MANIFEST.yaml`. Both platforms are merged as a **union**:
the appliance spec is not a subset of the software one. The shipped binary never reads these
files -- only `tools/semp-gen` does, at generation time.

To adopt a new broker version: add `specs/<platform>/<new-version>/`, update
`specs/MANIFEST.yaml`, run the `gen` task, and review the diff under `internal/modules/gen_*.go`.

## Build & test (run from the repo root)

- Build: `./scripts/dev.sh build` (`.\scripts\dev.ps1 build` on Windows) ->
  `dist/semp-workflow-<os>-<arch>[.exe]` (host os/arch unless `TARGET_OS`/`TARGET_ARCH` are set)
- Generate: `./scripts/dev.sh gen` -> rewrites `internal/modules/gen_*.go`, the module
  manifest and `docs/modules/`. Must leave a clean `git diff`; CI enforces that.
- Tests: `go test ./...` (or `./scripts/dev.sh test`)
- Coverage: `./scripts/dev.sh cov` -> printed total + `coverage.html`
- Security scan: `./scripts/dev.sh scan` -> `go tool govulncheck ./...` (govulncheck pinned as
  a go.mod `tool` directive; the `toolchain` directive pins Go, so it resolves through the
  module -- never `go run pkg@version`). Fatal on a fixable CVE.
- Integration tests (need a live broker): `go test -tags integration ./...` with
  `SEMP_HOST/SEMP_USERNAME/SEMP_PASSWORD/SEMP_MSG_VPN` set. Platform-gated: appliance-only
  and software-only objects cannot both be exercised on one broker.
- Validate examples: `semp-workflow validate -c examples/config.yaml -t examples/templates`
- Version is stamped at build time via `-ldflags "-X main.version=..."` -- no reinstall step.

## Documentation rules

- README.md: brief overview only, no technical details
- docs/HOWTO.md, docs/HOWTO-zh.md: full technical guide, use the `semp-workflow` binary
  exclusively. Their module tables cover only the modules the bundled templates use; the
  full reference is the generated `docs/modules/`. Keep English and Chinese in sync.
- docs/modules/ is generated by `semp-workflow list-modules --output-dir docs/modules`.
  Never hand-edit it, and never hand-maintain module counts in prose -- the `docscheck` task
  regenerates and fails on a diff.
- docs/template-*.md: capture template workflow details, inputs, step by step workflow, and
  example usage.
