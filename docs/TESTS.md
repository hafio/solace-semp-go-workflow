# Test Documentation

## Overview

The Go implementation ships **183 automated tests** — **173 unit tests** and
**10 integration tests** — built on Go's standard `testing` package plus
[`stretchr/testify`](https://github.com/stretchr/testify) for assertions.

Tests live **beside the code they cover** as `*_test.go` files, mirroring the
`internal/` package layout (Go's convention — no separate `tests/` tree). They
are table-driven where a behavior has many input variants. Each package also has
a `coverage_test.go` that targets the error and edge-case branches its main
suite does not reach (delete/update failures, malformed input, request/JSON
error paths), keeping line coverage high. Three kinds of test double are used:

- **Module tests** inject an in-memory fake that satisfies the `modules.Client`
  interface (`internal/modules/fake_test.go`) — no network, no broker.
- **SEMP client tests** run against `net/http/httptest` servers that return
  canned SEMP response bodies, so retry/timeout/`responseCode` logic is exercised
  without a broker.
- **Integration tests** hit a real Solace broker, gated behind the `integration`
  build tag and skipped unless connection env vars are set.

---

## Running the Tests

All commands run from the repo root (module `semp-workflow`).

### Unit tests (no broker required)

```bash
go test ./...                     # all packages
./scripts/dev.sh test             # same, via the dev script (logs to scripts/logs/test.log)
```

On Windows use `.\scripts\dev.ps1 test`.

```bash
go test -v ./...                          # verbose (per-test output)
go test ./internal/modules/...            # a single package
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

### Integration tests (live broker required)

Integration tests are compiled only with the `integration` build tag and skip
themselves at runtime unless **all four** connection variables are set:

```bash
export SEMP_HOST="https://broker.example.com:943"
export SEMP_USERNAME="admin"
export SEMP_PASSWORD="admin"
export SEMP_MSG_VPN="default"

go test -tags integration ./...                 # all four integration packages
go test -tags integration ./internal/modules/... # just the module lifecycles
```

Or via the dev script. Unlike the raw `go test` above, the dev-script task reads
the broker connection **only** from `scripts/local.env` (there is no
environment fallback), so copy the template and fill it in first:

```bash
cp scripts/local.env.example scripts/local.env   # then edit in real values
./scripts/dev.sh integration      # .\scripts\dev.ps1 integration on Windows
```

`local.env` is **required** for this task and is its only source: bash `source`s
it and PowerShell parses it (both strip surrounding quotes and export the values
to the `go test` child). A **missing** file fails the task with instructions to
copy `local.env.example` — it never runs a skip-only suite that could be mistaken
for real coverage. `local.env` is **gitignored** — it may hold the broker
password and must never be committed; only the `.example` template is tracked.
Because bash sources the file, quote values containing spaces or shell
metacharacters (prefer single quotes).

`integration` is opt-in — it is deliberately **not** part of `all`/`full`, so
broker-free CI and local runs still pass. (`SEMP_VERIFY_SSL=true` enables TLS
cert verification; it defaults to `false`.)

Integration tests span four packages — `internal/semp` (client), `internal/modules`
(module-action lifecycles), `internal/engine` (`Engine.Run()` over a fixture
template), and `internal/cli` (run/validate/list-modules). Every resource they
create is prefixed **`TEST-SEMP-WF-`** (the engine and CLI suites use the
`TEST-SEMP-WF-ENG`/`TEST-SEMP-WF-CLI` variants) and removed with `t.Cleanup`
(and pre-deleted for a clean slate), so a re-run is safe. If any variable is
unset the tests call `t.Skip` rather than fail — so a plain
`go test -tags integration ./...` with no env set reports `ok` with skips.

### Aggregate dev-script tasks

```bash
./scripts/dev.sh all              # build vet test               (CI runs `all scan`)
./scripts/dev.sh scan             # govulncheck (image scan N/A -- no Dockerfile)
./scripts/dev.sh full             # all + cov + image + scan + graphify
```

---

## Test Layout

| Package | Test file(s) | Tests | Covers |
|---|---|---|---|
| `internal/semp` | `helpers_test.go`, `client_test.go`, `coverage_test.go` | 30 | Encoding/coercion helpers; HTTP client `responseCode` logic, retry, timeout; request/JSON error paths |
| `internal/semp` | `integration_test.go` | 3 (integration) | Live-broker connection + client-level queue lifecycle |
| `internal/models` | `models_test.go` | 3 | `ActionResult`/`WorkflowResult`, status counts, `HasFailures()` |
| `internal/errors` | `errors_test.go`, `coverage_test.go` | 4 | Error types, messages, `errors.As` unwrapping |
| `internal/templating` | `templating_test.go`, `coverage_test.go` | 20 | gonja rendering, StrictUndefined, `ValidateInputs`, type coercion; recursion/undefined error paths |
| `internal/modules` | `fake_test.go` (fixtures), `modules_test.go`, `coverage_test.go` | 49 | All 24 actions across 10 families + registry; add/delete/update error branches |
| `internal/modules` | `integration_test.go` | 3 (integration) | Module-action lifecycles (queue+subscription, RDP, access-control) against a live broker |
| `internal/config` | `config_test.go`, `coverage_test.go` | 21 | YAML loading, schema validation, template source resolution; malformed-input skips |
| `internal/engine` | `engine_test.go`, `coverage_test.go` | 16 | Workflow orchestration, two-pass render, fail-fast, dry-run; constructor errors |
| `internal/engine` | `integration_test.go` | 1 (integration) | `Engine.Run()` over the fixture template: dry-run → create → rerun-skip → delete → rerun-skip |
| `internal/cli` | `cli_test.go`, `coverage_test.go` | 15 | Command wiring, exit-code classification, file output; filesystem-error paths |
| `internal/cli` | `integration_test.go` | 3 (integration) | `run`/`validate`/`list-modules` against a live broker |
| `internal/output` | `output_test.go`, `coverage_test.go` | 14 | Console formatting, recap counts, Markdown doc generation, banners |
| `templates` | `embed_test.go` | 1 | Embedded bundled-template `FS()` accessor |

**Shared fixtures** (`internal/modules/fake_test.go`): a `fakeClient`
implementing the `modules.Client` interface, plus helpers `existing()`,
`absent()`, and `sempErr()` used to script "resource present / absent / broker
error" scenarios across every module test. The engine tests use an inline
`engineFake`; the SEMP client tests use `newTestClient` over `httptest`.

---

## Unit Tests

### `internal/semp` — helpers (`helpers_test.go`, 6 tests)

| Test | Verifies |
|---|---|
| `TestStringify` | `Stringify` renders values to their canonical string form (bools -> `True`/`False`, ints, floats, nil -> `None`) |
| `TestEnc` | `Enc` percent-encodes every reserved char, leaving the unreserved set `A-Za-z0-9-._~` intact |
| `TestCoerceBool` | bool passthrough; string `true`/`yes`/`1` → true; else truthiness |
| `TestCoerceInt` | int/float/string coercion; rejects non-numeric strings |
| `TestCleanPayload` | drops nil and blank/whitespace-only strings; keeps `0`, `false`, empty collections |
| `TestCheckNameLength` | enforces `NameMaxLengths` per resource (queueName 200, restConsumerName 32, etc.) |

### `internal/semp` — client (`client_test.go`, 11 tests)

| Test | Verifies |
|---|---|
| `TestExistsFound` | GET → `(true, data)` when the resource exists |
| `TestExistsNotFoundBySempCode` | SEMP `responseCode` with `sempCode == 6` → `(false, nil)`, no error |
| `TestExistsPropagatesOtherErrors` | a non-NotFound SEMP error is returned, not swallowed |
| `TestCreateSuccess` | POST success decided by `body.meta.responseCode == 200`, returns `body.data` |
| `TestCreateAlreadyExists` | `sempCode == 10` surfaces as a `SEMPError` with that code |
| `TestUpdateAndDelete` | PATCH and DELETE happy paths |
| `TestConnectionFailed` | connection error → `SEMPError` with `StatusCode == 0` |
| `TestRetryOnServerError` | 502/503/504 are retried (total 3, 0.5s backoff) |
| `TestNoRetryOnClientError` | 4xx is **not** retried |
| `TestTimeout` | request timeout surfaces as a connection-class `SEMPError` |
| `TestTestConnection` | `TestConnection` reports reachability without raising |

### `internal/semp` — error paths (`coverage_test.go`, 13 tests)

| Test | Verifies |
|---|---|
| `TestRequestBuildErrors` | an unmarshalable payload / invalid method → "Request failed", `StatusCode == 0` |
| `TestRequestBodyReadError` | a truncated response body → "Connection failed" |
| `TestRequestMalformedJSON` | invalid JSON → "Invalid JSON response" carrying the HTTP status |
| `TestRequestMissingMeta` | a body with no `meta` block → `responseCode` falls back to the HTTP status, `data` returned |
| `TestRequestErrorWithoutDescription` | a non-200 with an `error.code` but no description → raw body used as the message |
| `TestRequestErrorEmptyBody` | a non-200 with an empty body → "Unknown SEMP error" |
| `TestTestConnectionTransportError` | a transport error → `TestConnection` returns false |
| `TestTestConnectionRequestBuildError` | an invalid host → `TestConnection` returns false |
| `TestTrimTrailingSlashMultiple` | multiple trailing slashes are stripped from the host |
| `TestRetryTransportBodyReadError` | the retry transport surfaces a request-body read error |
| `TestRetryTransportBackoffSleep` | the retry loop sleeps on a nonzero backoff before retrying |
| `TestBackoffDelayValues` | `backoffDelay` zero (attempt < 1) and exponential branches |
| `TestCoerceBoolDefaultBranch` | `CoerceBool` default arm — a non-nil unhandled type is truthy |

### `internal/models` (`models_test.go`, 3 tests)

| Test | Verifies |
|---|---|
| `TestWorkflowResultCounts` | `OKCount`/`SkippedCount`/`DryRunCount`/`FailedCount` tally task statuses |
| `TestWorkflowResultNoFailures` | `HasFailures()` is false when no task is `FAILED` |
| `TestNewActionResult` | `NewActionResult` constructs a result with the expected fields |

### `internal/errors` (`errors_test.go`, 3 tests)

| Test | Verifies |
|---|---|
| `TestIsWorkflowError` | `ConfigError`/`TemplateError`/`SEMPError` are all recognized as `WorkflowError` |
| `TestErrorMessages` | each type formats an actionable message (incl. `SEMPError` status/sempCode) |
| `TestErrorsAsConcreteTypes` | `errors.As` unwraps a wrapped error back to its concrete type |

`coverage_test.go` (1 test): `TestWorkflowAndValidationErrorMessages` exercises
`WorkflowError.Error()` and `ValidationError.Error()` directly — the base suite
reaches them only through the `IsWorkflowError` marker check.

### `internal/templating` (`templating_test.go`, 13 tests)

| Test | Verifies |
|---|---|
| `TestRenderPassthrough` | non-string values (bool/int/nil) pass through untouched, keeping their type |
| `TestRenderString` | `{{ inputs.x }}` renders against context |
| `TestRenderDefaultFilter` | the `default('fallback')` filter supplies a value when the variable is undefined |
| `TestRenderUndefinedIsTemplateError` | an undefined variable (StrictUndefined) → `TemplateError` |
| `TestRenderRecursesMapsAndSlices` | maps and slices are rendered element-by-element |
| `TestValidateInputsProvidedWins` | a provided value overrides the template default |
| `TestValidateInputsUsesDefault` | a missing input falls back to its rendered default |
| `TestValidateInputsRequiredMissing` | a missing required input → `TemplateError` |
| `TestValidateInputsUnexpected` | an input not in the schema → `TemplateError` |
| `TestValidateInputsCoercionToString` | default type is `string`; values are stringified |
| `TestValidateInputsIntegerType` | `integer`-typed inputs coerce/validate |
| `TestValidateInputsBooleanType` | `boolean`-typed inputs coerce |
| `TestValidateInputsUnresolvableDefaultKeptRaw` | a default referencing not-yet-available inputs is kept raw for the engine's second pass |

**Edge & error paths (`coverage_test.go`, 7 tests):**
`TestRenderMapRecursionError` / `TestRenderSliceRecursionError` (a render failure
inside a map value or slice element aborts `Render` → `TemplateError`);
`TestRenderMalformedTemplateError` (an unparseable template → "Template rendering
error", not "Undefined variable"); `TestRenderUndefinedIndexBranch`
(`{{ inputs[] }}` under StrictUndefined → "Undefined variable");
`TestValidateInputsOptionalSkipped` (an optional input with no default is
omitted); `TestValidateInputsUnknownTypePassthrough` (an unrecognized declared
type passes the value through unchanged); `TestValidateInputsIntegerCoercionVariants`
(`toInt` across bool / int / int64 / float and an uncoercible slice).

### `internal/modules` (`modules_test.go`, 35 tests)

**Registry (3):** `TestRegistryCompleteness` asserts exactly the **24** expected
action keys are registered; `TestGetUnknownModule` checks the
`Unknown module ...` error; `TestInfoHasParams` verifies `Info()` exposes
ordered param specs for doc generation.

**Queue (11):** `TestQueueAddCreatesWithDerivedFields`,
`TestQueueAddRedeliveryEnabledDerivation` (the `maxRedeliveryCount == -1` → 0 +
`redeliveryEnabled=false` rule), `TestQueueAddSkippedWhenExists`,
`TestQueueAddDryRun`, `TestQueueAddMissingName`, `TestQueueAddNameTooLong`,
`TestQueueAddExistsError`, `TestQueueAddCreateError`, `TestQueueAddBadIntFails`,
`TestQueueDelete`, `TestQueueUpdate`.

**Queue subscription (6):** `TestSubscriptionAddPostsDirectly` (POSTs without a
pre-check), `TestSubscriptionAddAlreadyExists` (`sempCode 10` → SKIPPED),
`TestSubscriptionAddCreateError`, `TestSubscriptionAddDryRun`,
`TestSubscriptionAddMissingArgs`, `TestSubscriptionDelete`.

**RDP / REST consumer / queue binding (8):** `TestRdpAddAndUpdate`,
`TestRdpDelete`, `TestRestConsumerAdd`, `TestRestConsumerAddBadPort`,
`TestRestConsumerNameTooLong`, `TestQueueBindingAddPreCheck`,
`TestQueueBindingAddCreates`, `TestQueueBindingAddAlreadyExistsFallback` (the
pre-check **and** ALREADY_EXISTS fallback).

**ACL (4):** `TestAclProfileAddDelete`,
`TestPublishExceptionAddCompositePath` (composite `{enc(syntax)},{enc(topic)}`
path), `TestPublishExceptionMissingArgs`, `TestSubscribeExceptionAdd`.

**Client profile / username (3):** `TestClientProfileAddCoercion`,
`TestClientProfileUpdateNoFields` (empty update → SKIPPED "No fields to
update"), `TestClientUsernameLifecycle`.

**Add, delete & update error branches (`coverage_test.go`, 14 tests):** the five
delete actions with no prior direct test — `TestPublishExceptionDelete`,
`TestSubscribeExceptionDelete` (composite `{enc(syntax)},{enc(topic)}` path),
`TestClientProfileDelete`, `TestQueueBindingDelete`, `TestRestConsumerDelete` —
each across missing-args→FAILED, absent→SKIPPED, dry-run→DRYRUN, present→OK, and
`Delete` error→FAILED; plus `TestClientUsernameUpdateAppliesFields` (PATCH issued
with the name key dropped, asserting both `clientProfileName` **and**
`aclProfileName` associations land in the body), `TestSubscriptionDeleteBranches`,
`TestAclProfileDeleteBranches`, `TestQueueDeleteAndUpdateErrorBranches`, and
`TestRdpDeleteAndUpdateErrorBranches` (Exists / Delete / Update error → FAILED).

The four **add-path** branch tests bring the add actions to full branch
coverage -- each covers missing-args->FAILED (where applicable),
SKIPPED-when-exists, DRYRUN, Exists-error->FAILED, and Create-error->FAILED:
`TestRestConsumerAddBranches` (plus the `outgoingConnectionCount` int coercion),
`TestQueueBindingAddBranches` (plus name-too-long at limit 200),
`TestPublishExceptionAddBranches`, and `TestSubscribeExceptionAddBranches` (plus
the default `subscribeTopicExceptionSyntax == "smf"` assertion and the composite
create path).

### `internal/config` (`config_test.go`, 13 tests)

| Test | Verifies |
|---|---|
| `TestLoadConfigValid` | a well-formed config loads with defaults applied |
| `TestLoadConfigTemplatesDirOnDisk` | `templates_dir` resolved relative to the config file when it exists on disk |
| `TestLoadConfigOverrides` | `verify_ssl`/`timeout` overrides are honored |
| `TestLoadConfigMissingFile` | a missing config file → `ConfigError` |
| `TestLoadConfigNotAMapping` | a non-mapping top-level document → `ConfigError` |
| `TestLoadConfigMissingSemp` | absent `semp:` block → `ConfigError` |
| `TestLoadConfigMissingSempKey` | a missing `host`/`username`/`password`/`msg_vpn` → `ConfigError` |
| `TestLoadConfigWorkflowsNotList` | `workflows` not a list → `ConfigError` |
| `TestLoadConfigWorkflowMissingTemplate` | a workflow entry without `template` → `ConfigError` (1-based index) |
| `TestLoadTemplatesFS` | templates load from a filesystem dir; registry key = `<file>.<name>` |
| `TestLoadTemplatesMissingModule` | an action referencing an unknown module is reported |
| `TestLoadTemplatesNonListSkipped` | a non-list `workflow-templates` document is skipped, not fatal |
| `TestLoadTemplatesDirNotFound` | a missing templates dir → `ConfigError` |

**Edge & error paths (`coverage_test.go`, 8 tests):** `TestLoadConfigReadError`
(a path that stats but fails to read → `ConfigError`); `TestLoadConfigParseErrors`
(malformed YAML; a workflow entry that is not a map); `TestLoadConfigEdgeDefaults`
(a workflow without inputs, an explicit `templates_dir`, a non-string scalar, an
unparseable `timeout` falling back to the default); `TestLoadTemplatesDirSuccess`
(loading via `os.DirFS`); `TestLoadTemplatesReadError`; `TestLoadTemplatesParseError`;
`TestLoadTemplatesSkippedEntries` (non-mapping document, non-map template/action
entry, template with no name — all skipped, not fatal); `TestLoadTemplatesActionNoArgs`
(an action with nil args defaults to an empty map).

### `internal/engine` (`engine_test.go`, 9 tests)

| Test | Verifies |
|---|---|
| `TestRunWorkflowRendersArgs` | action args are rendered before dispatch |
| `TestRunWorkflowTwoPassRender` | `provided → global_var → inputs.X` chains resolve within two passes |
| `TestRunWorkflowCircularReference` | a circular reference leaves an unresolved marker → `WorkflowError` |
| `TestResolveTemplateNotFound` | an unknown template reference → `TemplateError` |
| `TestRunActionUnknownModule` | an unknown module becomes a `FAILED` task (no crash) |
| `TestRunActionTemplateError` | a render error becomes a `FAILED` task |
| `TestRunWorkflowFailFastStopsActions` | `--fail-fast` halts on the first `FAILED` action |
| `TestRunDryRun` | dry-run yields `DRYRUN` results and calls no mutating client method |
| `TestRunTemplateNotFoundSynthesizesFailure` | a top-level `WorkflowError` becomes a synthetic `FAILED` "Template Resolution" task |

**Constructor & fail-fast paths (`coverage_test.go`, 7 tests):**
`TestNewEngineBundledTemplates` (the `NewEngine` success path loads the embedded
templates and builds the client); `TestNewEngineMissingTemplatesDir` (a missing
non-bundled dir → error, nil engine); `TestRunWorkflowMissingRequiredInput` and
`TestRunWorkflowInputRenderError` (the `ValidateInputs` and input-render error
branches → `WorkflowError`); `TestRunFailFastStopsOnTemplateError` and
`TestRunFailFastStopsAfterFailedWorkflow` (both `--fail-fast` break points);
`TestResolveTemplateNotFoundListsAvailable` (the not-found error lists the
available template names).

### `internal/cli` (`cli_test.go`, 10 tests)

| Test | Verifies |
|---|---|
| `TestListModules` | `list-modules` prints the registered actions |
| `TestListModulesWritesFile` | `--output` writes the Markdown reference to a file |
| `TestValidateBundledTemplateOK` | `validate` against bundled templates exits 0 |
| `TestValidateTemplateNotFound` | a bad template reference → exit 2 |
| `TestValidateConfigError` | a config error → exit 2 |
| `TestRunConfigError` | `run` with a config error → exit 2 |
| `TestRunTemplateNotFoundExit1` | a run-time resolution failure → exit 1 |
| `TestInitCopiesBundledTemplates` | `init` writes embedded templates to a directory |
| `TestMissingConfigFlagIsUsageError` | omitting the required `-c/--config` → usage error, exit 2 |
| `TestClassifyExit` | `classifyExit` maps error types to codes (0/1/2/130) |

**Filesystem-error paths (`coverage_test.go`, 5 tests):**
`TestRunTemplatesDirNotFound` and `TestValidateTemplatesDirNotFound` (a bad
`--templates-dir` → engine/load error → exit 2); `TestListModulesWriteFailure`
(a failed `-o` write is reported but leaves the exit code 0);
`TestInitMkdirAllFailure` and `TestInitWriteFailure` (`init` directory-create and
file-write failures → exit 2).

### `internal/output` (`output_test.go`, 8 tests)

| Test | Verifies |
|---|---|
| `TestPrintTaskResultLabels` | status → label mapping (`changed`/`dryrun`/`skipped`/`FAILED`) |
| `TestPrintTaskResultMessage` | a failure message renders as `=> <message>` |
| `TestPrintTaskResultFallsBackToModuleName` | with no task name, the module name is shown |
| `TestPrintRecapCountsAndReturn` | recap tallies `changed`/`skipped`/`failed`; returns "had failures" |
| `TestPrintRecapAllSuccess` | an all-success recap prints the success line, returns false |
| `TestPrintModuleList` | `list-modules` output groups actions by object family |
| `TestPrintError` | `PrintError` writes to stderr with an `ERROR:` prefix |
| `TestRenderModuleDocsMDStructure` | generated Markdown has the title, idempotency note, TOC anchors, section headers, and param-table header |

**Banners & doc edge cases (`coverage_test.go`, 6 tests):** `TestPrintBanner`,
`TestPrintDryRunBanner`, and `TestPrintValidationOK` (banner / dry-run /
validation-summary text); `TestPrintWorkflowHeaderWithInputs` and
`TestPrintWorkflowHeaderNoInputs` (the `PLAY` header — inputs rendered sorted, and
the `Inputs:` line omitted when empty); `TestRenderModuleDocsMDEnumAndNoParams`
(enum values appended to a param description, and the `_No parameters._`
placeholder).

### `templates` (`embed_test.go`, 1 test)

| Test | Verifies |
|---|---|
| `TestFSContainsBundledTemplates` | `FS()` exposes the embedded bundled templates — `app-inbound.yaml` reads back non-empty |

---

## Integration Tests

**Location:** four `integration_test.go` files (build tag `//go:build integration`),
one per layer — `internal/semp`, `internal/modules`, `internal/engine`,
`internal/cli` — plus the shared fixture template
`testdata/integration/test-artifacts.yaml`, which provisions every supported
artifact type (`acl_profile`, `client_profile`, `client_username`, `queue`,
`q_sub`, `rdp`, `rdp_rc`, `rdp_qb`) in a `create` workflow and removes them in
reverse dependency order in a `delete` workflow.

**Prerequisites:** a reachable Solace broker and these environment variables —
`SEMP_HOST`, `SEMP_USERNAME`, `SEMP_PASSWORD`, `SEMP_MSG_VPN` (plus optional
`SEMP_VERIFY_SSL`, default `false`). When any of the four is unset the tests call
`t.Skip`, so the suite never fails for a missing broker.

**Run:** `go test -tags integration ./...` (all four packages), or a single
package, e.g. `go test -tags integration ./internal/modules/...`.

**Resource hygiene:** every resource is prefixed `TEST-SEMP-WF-` (the engine and
CLI suites use the `TEST-SEMP-WF-ENG`/`TEST-SEMP-WF-CLI` variants), pre-deleted
before use, and removed via `t.Cleanup` after — so re-runs are safe.

### `internal/semp` — client level (3 tests)

| Test | Scenario | Asserts |
|---|---|---|
| `TestIntegrationConnection` | `TestConnection` against the live broker | broker is reachable with the provided credentials |
| `TestIntegrationQueueLifecycle` | create → read → re-create → update → delete a queue | absent before create; present after; second create → `SEMPError` with `AlreadyExists` (sempCode 10); update & delete succeed; absent again |
| `TestIntegrationExistsNotFound` | `Exists` on a non-existent queue | returns `(false, nil)` with no error |

### `internal/modules` — module-action lifecycles (3 tests)

Drive the real action modules end-to-end via the `exec()` helper against a live
broker (`*semp.Client` satisfies the `modules.Client` interface).

| Test | Scenario | Asserts |
|---|---|---|
| `TestIntegrationQueueSubscriptionLifecycle` | `queue.add`→OK / re-add→SKIPPED / `queue.update`→OK / `q_sub.add`→OK / re-add→SKIPPED / `q_sub.delete`→OK / re-delete→SKIPPED / `queue.delete`→OK / re-delete→SKIPPED | idempotency at every step; `queue.update` on an absent queue → FAILED (never creates on update) |
| `TestIntegrationRdpLifecycle` | `rdp.add` / `rdp_rc.add` (host, port 443, TLS) / `queue.add` / `rdp_qb.add` (post target), then teardown in dependency order binding→consumer→RDP→queue | each step → OK; ordered teardown succeeds |
| `TestIntegrationAccessControlLifecycle` | table over `acl_profile` / `client_profile` / `client_username`: dry-run-add / add→OK / re-add→SKIPPED / dry-run-delete / delete→OK / re-delete→SKIPPED | dry-run add does **not** create; dry-run delete does **not** remove (verified via `Exists`); idempotency otherwise |

### `internal/engine` — engine-driven fixture run (1 test)

| Test | Scenario | Asserts |
|---|---|---|
| `TestIntegrationEngineLifecycle` | `Engine.Run()` over the fixture template: dry-run create → create → rerun → delete → rerun-delete | dry-run yields only `DRYRUN` and creates nothing; create yields `OK`/`SKIPPED` and every artifact exists; rerun all `SKIPPED`; delete yields `OK`/`SKIPPED` and every artifact is gone; rerun-delete all `SKIPPED` |

### `internal/cli` — command level (3 tests)

| Test | Scenario | Asserts |
|---|---|---|
| `TestIntegrationCLIListModules` | `list-modules` | exit 0; output contains every registered `object.verb` name |
| `TestIntegrationCLIValidate` | `validate` with a valid config + real template (via `-t`), then a bad template ref | valid → exit 0 + "Validation passed!"; unknown template → non-zero + "not found" |
| `TestIntegrationCLIRun` | `run --dry-run` then a real `run` over the fixture config | dry-run → exit 0, queue not created; real run → exit 0, queue and RDP exist afterward |

---

## Result Status Reference

Every module's `Execute` method returns a `models.ActionResult` whose `Status`
is one of the `models.ResultStatus` constants:

| Constant | String | Meaning |
|---|---|---|
| `StatusOK` | `ok` | Action was performed and succeeded (resource created/deleted/updated) |
| `StatusSkipped` | `skipped` | No action needed — resource already in the desired state (idempotent) |
| `StatusDryRun` | `dryrun` | Dry-run mode: the action would have been performed but was not |
| `StatusFailed` | `failed` | Action failed due to a SEMP error, validation error, or unexpected error |

`WorkflowResult.HasFailures()` returns `true` only when at least one task has
`StatusFailed`. Console labels differ from the raw status: `StatusOK` prints as
`changed`, `StatusFailed` as `FAILED` (see `internal/output`).

---

## Module Registry

All 24 actions registered and their SEMP resource paths:

| Module | HTTP method | SEMP path |
|---|---|---|
| `queue.add` | `POST` | `queues` |
| `queue.delete` | `DELETE` | `queues/<name>` |
| `queue.update` | `PATCH` | `queues/<name>` |
| `q_sub.add` | `POST` | `queues/<queue>/subscriptions` |
| `q_sub.delete` | `DELETE` | `queues/<queue>/subscriptions/<topic>` |
| `rdp.add` | `POST` | `restDeliveryPoints` |
| `rdp.delete` | `DELETE` | `restDeliveryPoints/<name>` |
| `rdp.update` | `PATCH` | `restDeliveryPoints/<name>` |
| `rdp_rc.add` | `POST` | `restDeliveryPoints/<rdp>/restConsumers` |
| `rdp_rc.delete` | `DELETE` | `restDeliveryPoints/<rdp>/restConsumers/<rc>` |
| `rdp_qb.add` | `POST` | `restDeliveryPoints/<rdp>/queueBindings` |
| `rdp_qb.delete` | `DELETE` | `restDeliveryPoints/<rdp>/queueBindings/<queue>` |
| `acl_profile.add` | `POST` | `aclProfiles` |
| `acl_profile.delete` | `DELETE` | `aclProfiles/<name>` |
| `acl_publish_exception.add` | `POST` | `aclProfiles/<name>/publishTopicExceptions` |
| `acl_publish_exception.delete` | `DELETE` | `aclProfiles/<name>/publishTopicExceptions/<syntax>,<topic>` |
| `acl_subscribe_exception.add` | `POST` | `aclProfiles/<name>/subscribeTopicExceptions` |
| `acl_subscribe_exception.delete` | `DELETE` | `aclProfiles/<name>/subscribeTopicExceptions/<syntax>,<topic>` |
| `client_profile.add` | `POST` | `clientProfiles` |
| `client_profile.delete` | `DELETE` | `clientProfiles/<name>` |
| `client_profile.update` | `PATCH` | `clientProfiles/<name>` |
| `client_username.add` | `POST` | `clientUsernames` |
| `client_username.delete` | `DELETE` | `clientUsernames/<name>` |
| `client_username.update` | `PATCH` | `clientUsernames/<name>` |
