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

## Naming

Operator-facing names match the **Solace Terraform provider** (`SolaceProducts/solacebroker`),
so one spelling works across both tools.

- **Action names** are `snake_case(DefinitionName).verb` -- `MsgVpnQueue` -> `msg_vpn_queue.add`,
  which is the provider's `solacebroker_msg_vpn_queue` minus its provider prefix. There is no
  alias table: all 86 prefixes are mechanical, so a name is predictable from the SEMP object.
- **Attribute names** are the snake_case form of the SEMP attribute: `egressEnabled` ->
  `egress_enabled`. `Params()`, `docs/modules/` and every error message use this spelling.

Two names therefore exist for each attribute, and the distinction is load-bearing:

| Field | Meaning | Used for |
|---|---|---|
| `fieldDesc.Name` / `keyDesc.Name` | the SEMP spelling | payloads, GET responses, the diff, `NameMaxLengths` |
| `.canonical()` (`Canon`, else `Name`) | snake_case | `Params()`, docs, all error text |

**Never build a payload or read a broker response through `canonical()`, and never present
`Name` to an operator.** SEMP requires camelCase on the wire; snake_case is presentation only.
`Canon` is emitted only where it differs, so an already-lower-case name ("owner") carries none.

Generation-time guards, all in `tools/semp-gen/internal/ir`:

- `CheckSnakeRoundTrip` fails if any name does not survive snake_case and back. The scheme
  relies on no SEMP name containing two adjacent capitals (Solace writes `Tls`, not `TLS`);
  that is an observed property of the specs, not a promise, so it is asserted every run.
- `CheckPrefixes` fails on a prefix collision.
- `checkLegacyMatched` fails if a `legacyOverlays` entry matches no object.

## Backward compatibility

Both naming axes accept the previous implementation's spelling, so existing templates and
customer configs run unchanged. Both are tested, and both are described once for operators
under "Upgrading from an earlier version" in `docs/HOWTO.md` -- but neither appears in
`list-modules` or the generated module reference, which stay canonical-only.

- **Attributes**: `objectDesc.normalizeArgs` (`internal/modules/canon.go`) rewrites canonical
  keys to the SEMP spelling at one choke point per verb. Because `Name` *is* the SEMP name,
  a camelCase key already matches and costs nothing. Supplying both spellings of one attribute
  is a hard error -- map order is random, so picking one would be non-deterministic.
- **Action names**: the 10 carried-over prefixes (`queue`, `q_sub`, `rdp`, `rdp_rc`, `rdp_qb`,
  `client_profile`, `client_username`, `acl_profile`, `acl_publish_exception`,
  `acl_subscribe_exception`) resolve via `legacyRegistry` in `internal/modules/registry.go`.
  `Get` consults it; `List` and `Info` do not, so the documented surface stays canonical-only.

`internal/modules/modules_test.go` was carried over from the old repo and drives the legacy
names with camelCase args -- treat a failure there as a driver bug, not a test to update.

Two maps would silently stop applying if re-keyed carelessly, so each has a guard test:
`legacyOverlays` is keyed by **SEMP definition name** (stable across naming changes), and
`overrides` by **canonical prefix**.

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
- docs/modules/ is generated. The `gen` task regenerates the descriptors and then re-renders
  the reference from the registry, so both committed outputs move together; never hand-edit
  it. `gencheck` verifies the descriptors against `specs/` in `full` and in CI. It does not
  check the rendered reference: if you change a renderer or a `Params()` shape, run `gen` and
  commit the result.
- Counts in prose (README.md, docs/HOWTO*.md, docs/TESTS.md) are hand-maintained and will
  drift. docs/TESTS.md deliberately describes what each package covers rather than listing
  individual tests, because an enumerated list of 325 goes stale immediately.
- docs/template-*.md: capture template workflow details, inputs, step by step workflow, and
  example usage.
