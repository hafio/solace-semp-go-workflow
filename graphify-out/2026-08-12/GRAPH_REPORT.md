# Graph Report - Solace-SEMP-Workflow-Automation  (2026-08-12)

## Corpus Check
- 157 files · ~464,947 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1037 nodes · 2122 edges · 114 communities (36 shown, 78 thin omitted)
- Extraction: 80% EXTRACTED · 20% INFERRED · 0% AMBIGUOUS · INFERRED: 430 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- newTestClient
- driver_test.go
- extract.go
- capture
- LoadConfig
- SEMP Workflow Automation — How-To Guide
- SEMP Workflow Automation — 操作指南
- Stringify
- NewEngine
- Object
- runCLI
- Unit Tests
- NewTemplateError
- dev.sh
- registerObject
- dev.ps1
- objectDesc
- buildAttribute
- naming.go
- Release Checklist
- run
- Template: `app-inbound`
- Template: `app-outbound`
- render.go
- Solace SEMP Workflow Automation
- coerceField
- Solace SEMP Workflow Automation
- Roadmap
- Test Documentation
- test.sh
- solace-semp-workflow
- Extract
- Running the Tests
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init
- init

## God Nodes (most connected - your core abstractions)
1. `registerObject()` - 90 edges
2. `absent()` - 42 edges
3. `exec()` - 37 edges
4. `objectDesc` - 28 edges
5. `queueLike()` - 24 edges
6. `NewEngine()` - 22 edges
7. `newTestClient()` - 21 edges
8. `ctx()` - 21 edges
9. `runCLI()` - 20 edges
10. `LoadConfig()` - 20 edges

## Surprising Connections (you probably didn't know these)
- `NewEngine()` --calls--> `FS()`  [INFERRED]
  internal/engine/engine.go → templates/embed.go
- `TestIntegrationEngineLifecycle()` --calls--> `run()`  [INFERRED]
  internal/engine/integration_test.go → tools/semp-gen/main.go
- `newValidateCmd()` --calls--> `FS()`  [INFERRED]
  internal/cli/cli.go → templates/embed.go
- `newInitCmd()` --calls--> `FS()`  [INFERRED]
  internal/cli/cli.go → templates/embed.go
- `LoadTemplatesFS()` --references--> `FS()`  [EXTRACTED]
  internal/config/config.go → templates/embed.go

## Import Cycles
- None detected.

## Communities (114 total, 78 thin omitted)

### Community 0 - "newTestClient"
Cohesion: 0.07
Nodes (50): Duration, SEMPError, HandlerFunc, asSEMPError(), backoffDelay(), Request, Response, NewClient() (+42 more)

### Community 1 - "driver_test.go"
Cohesion: 0.06
Nodes (98): brokerObj(), deepObj(), desc(), globalObj(), T, queueLike(), TestAddAcceptsRequiredFalseAndZero(), TestAddBuildsGlobalScopedPathWithoutVPNHop() (+90 more)

### Community 2 - "extract.go"
Cohesion: 0.20
Nodes (19): Definition, Doc, Info, Operation, Parameter, PathItem, Property, Schema (+11 more)

### Community 3 - "capture"
Cohesion: 0.08
Nodes (49): Buffer, Command, classifyExit(), Execute(), joinComma(), newInitCmd(), newListModulesCmd(), newRootCmd() (+41 more)

### Community 4 - "LoadConfig"
Cohesion: 0.06
Nodes (67): ActionSpec, AppConfig, SempConfig, WorkflowEntry, WorkflowTemplate, Engine, Engine, engineFake (+59 more)

### Community 5 - "SEMP Workflow Automation — How-To Guide"
Cohesion: 0.05
Nodes (42): 10. Common Scenario Examples, 11. Troubleshooting, 1. Introduction, 2. Prerequisites, 3. Getting Started, 4. Project Structure, 5.1 SEMP Connection, 5.2 Global Variables (global_vars) (+34 more)

### Community 6 - "SEMP Workflow Automation — 操作指南"
Cohesion: 0.05
Nodes (42): 10. 常見情境範例, 11. 故障排除, 1. 簡介, 2. 系統需求, 3. 開始使用, 4. 專案結構, 5.1 SEMP 連線設定, 5.2 全域變數（global_vars） (+34 more)

### Community 7 - "Stringify"
Cohesion: 0.10
Nodes (26): cfgInt(), AttrEqual(), numericEqual(), sliceEqual(), T, TestAttrEqual(), toAnySlice(), CheckNameLength() (+18 more)

### Community 8 - "NewEngine"
Cohesion: 0.17
Nodes (32): Config, CoerceBool(), T, TestRenderMalformedTemplateError(), TestRenderMapRecursionError(), TestRenderSliceRecursionError(), TestRenderUndefinedIndexBranch(), TestValidateInputsIntegerCoercionVariants() (+24 more)

### Community 9 - "Object"
Cohesion: 0.24
Nodes (12): Attribute, Key, Kind, Object, ParentHop, Platform, Scope, ownKeyNames() (+4 more)

### Community 10 - "runCLI"
Cohesion: 0.18
Nodes (27): T, runCLI(), TestInitCopiesBundledTemplates(), TestListModules(), TestListModulesWritesFile(), TestMissingConfigFlagIsUsageError(), TestRunConfigError(), TestRunTemplateNotFoundExit1() (+19 more)

### Community 11 - "Unit Tests"
Cohesion: 0.15
Nodes (13): `internal/cli` (`cli_test.go`, 10 tests), `internal/config` (`config_test.go`, 13 tests), `internal/engine` (`engine_test.go`, 9 tests), `internal/errors` (`errors_test.go`, 3 tests), `internal/models` (`models_test.go`, 3 tests), `internal/modules` (`modules_test.go`, 35 tests), `internal/output` (`output_test.go`, 8 tests), `internal/semp` — client (`client_test.go`, 11 tests) (+5 more)

### Community 12 - "NewTemplateError"
Cohesion: 0.12
Nodes (17): ConfigError, TemplateError, ValidationError, WorkflowError, TestClassifyExit(), T, TestWorkflowAndValidationErrorMessages(), IsWorkflowError() (+9 more)

### Community 13 - "dev.sh"
Cohesion: 0.20
Nodes (27): CGO_ENABLED, load_local_env(), main(), NO_COLOR, _report_only(), _run(), run_logged(), dev.sh script (+19 more)

### Community 14 - "registerObject"
Cohesion: 0.10
Nodes (11): init(), init(), init(), init(), init(), init(), init(), init() (+3 more)

### Community 15 - "dev.ps1"
Cohesion: 0.24
Nodes (22): Get-BinPath(), Have(), Import-LocalEnv(), Invoke-Logged(), Invoke-Task(), Ok(), Step(), Task-All() (+14 more)

### Community 16 - "objectDesc"
Cohesion: 0.06
Nodes (43): NewActionResult(), T, TestNewActionResult(), TestWorkflowResultCounts(), TestWorkflowResultNoFailures(), argStr(), dryrun(), failed() (+35 more)

### Community 17 - "buildAttribute"
Cohesion: 0.24
Nodes (12): Combination, Flags, Parsed, applyCombos(), buildAttribute(), describe(), parseDefault(), cells() (+4 more)

### Community 18 - "naming.go"
Cohesion: 0.27
Nodes (12): DefaultPrefix(), friendlyName(), Prefix(), splitCamel(), T, TestDefaultPrefix(), TestFriendlyNamePreservesInitialisms(), TestLengthAliasesAreShorterThanTheMechanicalName() (+4 more)

### Community 19 - "Release Checklist"
Cohesion: 0.17
Nodes (12): 1. Run the test suite, 2. Run vet, lint, and the vulnerability scan, 3. Regenerate documentation, 4. Build the release binary, 5. Commit the release, 6. Tag the release, 7. The tag pipeline publishes the release automatically, Common Commands (+4 more)

### Community 20 - "run"
Cohesion: 0.31
Nodes (12): Spec, File, All(), Bytes(), Manifest(), actionCount(), main(), reportDrift() (+4 more)

### Community 21 - "Template: `app-inbound`"
Cohesion: 0.18
Nodes (11): Action Sequence, `delete` (7 actions), Delete the same resources, Dry-run then execute, Example Workflow Entry, Inputs, `new-seq` / `new-non-seq` (14 actions), Optional (with defaults) (+3 more)

### Community 22 - "Template: `app-outbound`"
Cohesion: 0.18
Nodes (11): Action Sequence, `delete` (6 actions), Delete the same resources, Dry-run then execute, Example Workflow Entry, Inputs, `new-seq` / `new-non-seq` (11 actions), Optional (with defaults) (+3 more)

### Community 23 - "render.go"
Cohesion: 0.40
Nodes (10): Builder, countAttrs(), describeObject(), literal(), Object(), quote(), renderAttribute(), renderKey() (+2 more)

### Community 24 - "Solace SEMP Workflow Automation"
Cohesion: 0.25
Nodes (7): Backward compatibility, Build & test (run from the repo root), Documentation rules, Naming, Project conventions, Solace SEMP Workflow Automation, Specs

### Community 25 - "coerceField"
Cohesion: 0.40
Nodes (9): coerceField(), fieldNames(), T, TestCoerceFieldIntErrorNamesTheAttribute(), TestCoerceFieldNestedObject(), TestCoerceFieldNestedObjectInnerError(), TestCoerceFieldNestedObjectWrongShape(), TestCoerceFieldScalars() (+1 more)

### Community 26 - "Solace SEMP Workflow Automation"
Cohesion: 0.25
Nodes (8): Build from source (requires Go 1.26+), Documentation, How It Works, Install a prebuilt binary (recommended), Quick Start, Solace SEMP Workflow Automation, Status, What It Does

### Community 28 - "Roadmap"
Cohesion: 0.29
Nodes (6): Deferred: SEMP v1, Design decisions, Naming, Roadmap, Stages, Why

### Community 29 - "Test Documentation"
Cohesion: 0.20
Nodes (10): Integration Tests, `internal/cli` — command level (3 tests), `internal/engine` — engine-driven fixture run (1 test), `internal/modules` — module-action lifecycles (3 tests), `internal/semp` — client level (3 tests), Module Registry, Overview, Result Status Reference (+2 more)

### Community 34 - "Extract"
Cohesion: 0.22
Nodes (7): legacyOverlay, checkLegacyMatched(), Extract(), applyLegacy(), LegacyPrefixes(), CheckPrefixes(), ValidateAliases()

### Community 35 - "Running the Tests"
Cohesion: 0.40
Nodes (5): Aggregate dev-script tasks, Coverage, Integration tests (live broker required), Running the Tests, Unit tests (no broker required)

## Knowledge Gaps
- **140 isolated node(s):** `solace-semp-workflow`, `Client`, `InputSpec`, `NO_COLOR`, `CGO_ENABLED` (+135 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **78 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `registerObject()` connect `registerObject` to `driver_test.go`, `capture`, `objectDesc`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`?**
  _High betweenness centrality (0.244) - this node is a cross-community bridge._
- **Why does `objectDesc` connect `objectDesc` to `driver_test.go`, `registerObject`, `Stringify`?**
  _High betweenness centrality (0.224) - this node is a cross-community bridge._
- **Why does `TestIntegrationEngineLifecycle()` connect `LoadConfig` to `newTestClient`, `run`, `Stringify`?**
  _High betweenness centrality (0.147) - this node is a cross-community bridge._
- **Are the 87 inferred relationships involving `registerObject()` (e.g. with `init()` and `init()`) actually correct?**
  _`registerObject()` has 87 INFERRED edges - model-reasoned connections that need verification._
- **Are the 40 inferred relationships involving `absent()` (e.g. with `TestAddAcceptsRequiredFalseAndZero()` and `TestAddBuildsGlobalScopedPathWithoutVPNHop()`) actually correct?**
  _`absent()` has 40 INFERRED edges - model-reasoned connections that need verification._
- **What connects `solace-semp-workflow`, `Client`, `InputSpec` to the rest of the system?**
  _140 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `newTestClient` be split into smaller, more focused modules?**
  _Cohesion score 0.0675990675990676 - nodes in this community are weakly interconnected._