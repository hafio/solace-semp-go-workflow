# Test Documentation

## Overview

The suite is **325 automated tests** -- **315 unit tests** and **10 integration
tests** -- built on Go's standard `testing` package plus
[`stretchr/testify`](https://github.com/stretchr/testify) for assertions.

Tests live **beside the code they cover** as `*_test.go` files, mirroring the
package layout (Go's convention -- no separate `tests/` tree). They are
table-driven where a behaviour has many input variants. Most packages also have a
`coverage_test.go` targeting the error and edge-case branches the main suite does
not reach.

This document deliberately does **not** enumerate individual tests. At 325 and
growing with each broker schema, such a list is wrong within a week; the tables
below describe what each package covers and the test names themselves say the
rest.

Four kinds of test double are used:

- **Module tests** inject an in-memory fake satisfying the `modules.Client`
  interface (`internal/modules/fake_test.go`) -- no network, no broker.
- **SEMP client tests** run against `net/http/httptest` servers returning canned
  SEMP response bodies, so retry, timeout and `responseCode` logic is exercised
  without a broker.
- **Generator tests** use small hand-built specification fixtures, plus a golden
  file for the renderer and the real committed schemas where testing against the
  actual input is the point.
- **Integration tests** hit a real Solace broker, gated behind the `integration`
  build tag and skipped unless connection variables are set.

---

## What the tests are guarding

Two properties matter more than any individual assertion, and both have dedicated
coverage:

**Backward compatibility.** `internal/modules/modules_test.go` was carried over
unchanged from the previous implementation, which hand-wrote its 24 actions. It
still passes against the generated descriptors, which is what proves existing
workflow templates and customer configs keep working. A failure there is a driver
defect, not a test to update.

**Generated output matches the schemas.** `tools/semp-gen` has its own suite, and
the `gencheck` task re-derives every descriptor from `specs/` and fails if the
committed output differs. The generator also fails loudly rather than guessing:
an unrecognised attribute flag column, an unsupported attribute type, a stale
alias, a prefix collision, or a legacy overlay entry matching no object all stop
generation.

---

## Running the Tests

All commands run from the repo root (module `solace-semp-workflow`).

### Unit tests (no broker required)

```bash
go test ./...                     # all packages
./scripts/dev.sh test             # same, via the dev script (logs to scripts/logs/test.log)
```

On Windows use `.\scripts\dev.ps1 test`.

```bash
go test -v ./...                                      # verbose (per-test output)
go test ./internal/modules/...                        # a single package
go test -run TestQueueAddDryRun ./internal/modules/   # a single test
```

### Coverage

```bash
./scripts/dev.sh cov              # prints total %, writes coverage.html
```

Equivalent raw commands:

```bash
go test -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out        # printed per-func + total
go tool cover -html=coverage.out -o coverage.html
```

The floor is the last recorded total. An unexplained drop is a failed gate, not a
rounding error -- deleting dead code or a weak test can legitimately lower it, but
say which number moved and why.

### Integration tests (live broker required)

Integration tests are compiled only with the `integration` build tag and skip
themselves at runtime unless **all four** connection variables are set:

```bash
export SEMP_HOST="https://broker.example.com:943"
export SEMP_USERNAME="admin"
export SEMP_PASSWORD="admin"
export SEMP_MSG_VPN="default"

go test -tags integration ./...                  # all four integration packages
go test -tags integration ./internal/modules/... # just the module lifecycles
```

Or via the dev script. Unlike the raw `go test` above, the dev-script task reads
the broker connection **only** from `scripts/local.env` (there is no environment
fallback), so copy the template and fill it in first:

```bash
cp scripts/local.env.example scripts/local.env   # then edit in real values
./scripts/dev.sh integration      # .\scripts\dev.ps1 integration on Windows
```

`local.env` is **required** for this task and is its only source: bash `source`s
it and PowerShell parses it (both strip surrounding quotes and export the values
to the `go test` child). A **missing** file fails the task with instructions to
copy `local.env.example` -- it never runs a skip-only suite that could be mistaken
for real coverage. `local.env` is **gitignored** -- it may hold the broker
password and must never be committed; only the `.example` template is tracked.
Because bash sources the file, quote values containing spaces or shell
metacharacters (prefer single quotes).

`integration` is opt-in -- deliberately **not** part of `all`/`full`, so
broker-free CI and local runs still pass. (`SEMP_VERIFY_SSL=true` enables TLS
certificate verification; it defaults to `false`.)

Every resource the integration tests create is prefixed **`TEST-SEMP-WF-`** (the
engine and CLI suites use the `TEST-SEMP-WF-ENG` / `TEST-SEMP-WF-CLI` variants)
and removed with `t.Cleanup`, and pre-deleted for a clean slate, so a re-run is
safe. If any variable is unset the tests call `t.Skip` rather than fail, so a
plain `go test -tags integration ./...` with no environment set reports `ok` with
skips.

**Coverage caveat.** The fixtures exercise the object families the bundled
templates use. The other 80 families have no live coverage, and appliance-only and
software-only objects cannot both be exercised against one broker, so no single
run can cover everything. Splitting the fixtures per family and gating them by
platform is open work -- see [roadmap.md](roadmap.md).

### Aggregate dev-script tasks

```bash
./scripts/dev.sh all              # build vet test               (CI runs `all gencheck scan`)
./scripts/dev.sh gen              # regenerate descriptors + module reference
./scripts/dev.sh gencheck         # fail if the committed output no longer matches specs/
./scripts/dev.sh scan             # govulncheck
./scripts/dev.sh full             # all + gencheck + cov + image + scan + graphify
```

---

## Test Layout

### Application

| Package | Test files | Unit | Integration | Covers |
|---|---|---|---|---|
| `internal/modules` | `driver_test.go`, `modules_test.go`, `validate_test.go`, `coerce_test.go`, `fake_test.go` (fixtures), `integration_test.go` | 101 | 3 | The three generic drivers once per structural shape (simple key, composite keys, nesting, singleton, global scope); the 24 legacy actions via the carried-over suite; argument validation; spec-driven coercion; live queue, RDP and access-control lifecycles |
| `internal/semp` | `client_test.go`, `helpers_test.go`, `path_test.go`, `compare_test.go`, `coverage_test.go`, `integration_test.go` | 37 | 3 | HTTP client `responseCode` logic, retry, timeout; path building and composite-key encoding; attribute comparison; length limits; live connection and client-level queue lifecycle |
| `internal/cli` | `cli_test.go`, `coverage_test.go`, `integration_test.go` | 20 | 3 | Command wiring, exit-code classification, the split module reference and its stale-page cleanup, offline validation; live `run`/`validate`/`list-modules` |
| `internal/engine` | `engine_test.go`, `coverage_test.go`, `integration_test.go` | 20 | 1 | Workflow orchestration, multi-pass rendering, fail-fast, dry-run, offline workflow validation; `Engine.Run()` over the fixture template (dry-run, create, rerun-skip, delete, rerun-skip) |
| `internal/output` | `output_test.go`, `coverage_test.go` | 21 | -- | Console formatting, recap counts, secret masking, the single-file and per-family Markdown renderers |
| `internal/config` | `config_test.go`, `coverage_test.go` | 21 | -- | YAML loading, schema validation, template source resolution; malformed input |
| `internal/templating` | `templating_test.go`, `coverage_test.go` | 20 | -- | gonja rendering, StrictUndefined, `ValidateInputs`, type coercion; recursion and undefined-variable paths |
| `internal/errors` | `errors_test.go`, `coverage_test.go` | 4 | -- | Error types, messages, `errors.As` unwrapping |
| `internal/models` | `models_test.go` | 3 | -- | `ActionResult`/`WorkflowResult`, status counts, `HasFailures()` |
| `templates` | `embed_test.go` | 1 | -- | Embedded bundled-template `FS()` accessor |

### Generator

| Package | Test files | Unit | Covers |
|---|---|---|---|
| `tools/semp-gen/internal/ir` | `extract_test.go`, `guards_test.go`, `naming_test.go` | 34 | Path and scope extraction, composite keys, parent chains, flag merging, prose defaults, description trimming, platform union, the legacy overlay, CamelCase splitting and naming, and the guard rails that fail generation |
| `tools/semp-gen` | `run_test.go`, `main_test.go` | 11 | Generate-then-check reports no drift; the three ways drift is detected (missing, out of date, stale); stale descriptors removed and hand-written files left alone; mismatched schema versions refused |
| `tools/semp-gen/internal/mdtable` | `mdtable_test.go` | 9 | The Markdown flag and combination tables that are the only source of per-attribute metadata, including the fail-loud path for an unrecognised column, and a pass over every operation in the real schema |
| `tools/semp-gen/internal/render` | `render_test.go` | 8 | Byte-exact golden file across every descriptor shape, output parses as valid Go, manifest ordering, duplicate-filename detection, string escaping |
| `tools/semp-gen/internal/specmodel` | `spec_test.go` | 5 | Schema loading and its error paths, body-definition and path-parameter extraction |

**Shared fixtures** (`internal/modules/fake_test.go`): a `fakeClient` implementing
the `modules.Client` interface, plus `existing()`, `absent()` and `sempErr()` to
script "resource present / absent / broker error" across every module test. The
engine tests use an inline `engineFake`; the SEMP client tests use `newTestClient`
over `httptest`.

---

## Result Status Reference

Every module's `Execute` returns a `models.ActionResult` whose `Status` is one of
the `models.ResultStatus` constants:

| Constant | String | Meaning |
|---|---|---|
| `StatusOK` | `ok` | The broker was modified |
| `StatusSkipped` | `skipped` | No action needed -- already in the desired state |
| `StatusDryRun` | `dryrun` | Dry-run: the action would have been performed but was not |
| `StatusFailed` | `failed` | Rejected locally by validation, or refused by the broker |

`WorkflowResult.HasFailures()` returns `true` only when at least one task has
`StatusFailed`. Console labels differ from the raw status: `StatusOK` prints as
`changed` and `StatusFailed` as `FAILED` (see `internal/output`).

An `update` reads current state and sends only differing attributes, so it
reports `skipped` when the object already matches. Attributes the broker never
returns -- write-only and opaque ones such as passwords -- cannot be compared and
are always re-sent, so an action supplying one reports `changed` every run.

---

## Module Registry

The registry is **225 actions across 86 object families**, generated from the
schemas in `specs/`. The full list with SEMP paths and parameter tables is
generated rather than duplicated here:

- [docs/modules/index.md](modules/index.md) -- committed reference, one page per family
- `semp-workflow list-modules` -- the list from the binary you have
- `internal/modules/manifest_generated.go` -- `ExpectedModules`, which
  `TestRegistryCompleteness` asserts the registry matches exactly

The 24 actions carried over from the previous implementation are asserted
separately from the generated total, so one of them disappearing fails loudly
regardless of what the schema surface does.
