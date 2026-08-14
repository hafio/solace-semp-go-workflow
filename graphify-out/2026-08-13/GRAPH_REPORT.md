# Graph Report - Solace-SEMP-Workflow-Automation  (2026-08-13)

## Corpus Check
- 254 files · ~548,912 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1550 nodes · 2957 edges · 206 communities (125 shown, 81 thin omitted)
- Extraction: 85% EXTRACTED · 15% INFERRED · 0% AMBIGUOUS · INFERRED: 430 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `8ceedb3f`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newTestClient
- driver_test.go
- naming.go
- output.go
- LoadConfig
- SEMP Workflow Automation - How-To Guide
- SEMP Workflow Automation — 操作指南
- coerceField
- NewEngine
- index.md
- testing.T
- absent
- NewTemplateError
- dev.sh
- registerObject
- dev.ps1
- objectDesc
- Parse
- `acl_profile`
- Release Checklist
- run
- Template: `app-inbound`
- Template: `app-outbound`
- `authentication_kerberos_realm`
- Solace SEMP Workflow Automation
- `authentication_oauth_profile`
- Solace SEMP Workflow Automation
- Roadmap
- `authentication_oauth_provider`
- Extract
- test.sh
- solace-semp-workflow
- `authorization_group`
- `bridge`
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
- helpers_test.go
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
- `bridge_tls_trusted_common_name`
- Doc
- `oauth_profile_client_allowed_host`
- init
- init
- init
- init
- checkLegacyMatched
- init
- init
- extract_test.go
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
- extract.go
- init
- init
- init
- init
- init
- ActionResult
- init
- `bridge_remote_msg_vpn`
- `broker_proxy`
- `cert_authority`
- `cert_matching_rule`
- `cert_matching_rule_attribute_filter`
- `client_cert_authority`
- `client_profile`
- `client_username`
- `distributed_cache`
- `distributed_cache_cluster`
- `distributed_cache_cluster_instance`
- `dmr_bridge`
- `dmr_cluster`
- `dmr_cluster_cert_matching_rule`
- `dmr_cluster_cert_matching_rule_filter`
- `dmr_cluster_link`
- `domain_cert_authority`
- `jndi_connection_factory`
- `jndi_queue`
- `jndi_topic`
- `kafka_receiver`
- `kafka_receiver_topic_binding`
- `kafka_sender`
- `kafka_sender_queue_binding`
- `mqtt_retain_cache`
- `mqtt_session`
- `mqtt_session_subscription`
- `msg_vpn`
- `oauth_profile`
- `oauth_profile_access_group_vpn_exception`
- `oauth_profile_access_level_group`
- `oauth_profile_client_authorization_parameter`
- `oauth_profile_default_vpn_exception`
- `proxy`
- `queue`
- `queue_template`
- `rdp`
- `rdp_qb`
- `rdp_qb_header`
- `rdp_qb_protected_header`
- `rdp_rc`
- `replay_log`
- `replicated_topic`
- `telemetry_profile`
- `telemetry_profile_trace_filter`
- `topic_endpoint`
- `topic_endpoint_template`
- `virtual_hostname`
- `acl_profile_client_connect_exception`
- Test Documentation
- `acl_profile_subscribe_share_name_exception`
- `acl_subscribe_exception`
- validate_test.go
- `bridge_remote_subscription`
- Load
- `cert_authority_ocsp_tls_trusted_common_name`
- `cert_matching_rule_condition`
- fakeClient
- collectionUpdate
- `distributed_cache_cluster_topic`
- `distributed_cache_home_cluster`
- `distributed_cache_home_cluster_prefix`
- `dmr_cluster_cert_matching_rule_condition`
- `dmr_cluster_link_attribute`
- `dmr_cluster_link_remote_address`
- `dmr_cluster_link_tls_trusted_common_name`
- newTestEngine
- Path
- `oauth_profile_resource_server_required_claim`
- `q_sub`
- `rdp_rc_jwt_claim`
- WorkflowResult
- `replay_log_topic_filter_subscription`
- `sequenced_topic`
- `telemetry_profile_receiver_acl_exception`
- `telemetry_profile_trace_filter_subscription`
- Stringify
- buildAttribute
- checkValue
- `client_cert_authority_ocsp_cn`
- `acl_profile_publish_exception`
- `acl_publish_exception`
- `authentication_oauth_client_claim`
- `rdp_rc_tls_cn`
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
2. `absent()` - 43 edges
3. `exec()` - 40 edges
4. `runCLI()` - 25 edges
5. `queueLike()` - 24 edges
6. `NewEngine()` - 24 edges
7. `newTestEngine()` - 23 edges
8. `objectDesc` - 21 edges
9. `newTestClient()` - 21 edges
10. `ctx()` - 21 edges

## Surprising Connections (you probably didn't know these)
- `TestIntegrationEngineLifecycle()` --calls--> `run()`  [INFERRED]
  internal/engine/integration_test.go → tools/semp-gen/main.go
- `NewEngine()` --calls--> `FS()`  [EXTRACTED]
  internal/engine/engine.go → templates/embed.go
- `init()` --calls--> `registerObject()`  [INFERRED]
  internal/modules/gen_acl_profile.go → internal/modules/registry.go
- `init()` --calls--> `registerObject()`  [INFERRED]
  internal/modules/gen_acl_profile_client_connect_exception.go → internal/modules/registry.go
- `init()` --calls--> `registerObject()`  [INFERRED]
  internal/modules/gen_acl_profile_publish_exception.go → internal/modules/registry.go

## Import Cycles
- None detected.

## Communities (206 total, 81 thin omitted)

### Community 0 - "newTestClient"
Cohesion: 0.06
Nodes (50): net/http.Client, net/http.HandlerFunc, net/http/httptest.Server, net/http.Request, net/http.Response, net/http.ResponseWriter, net/http.RoundTripper, time.Duration (+42 more)

### Community 1 - "driver_test.go"
Cohesion: 0.07
Nodes (57): buildParams(), brokerObj(), constDesc(), deepObj(), desc(), globalObj(), objectDesc, queueLike() (+49 more)

### Community 2 - "naming.go"
Cohesion: 0.24
Nodes (11): DefaultPrefix(), friendlyName(), indefiniteArticle(), Prefix(), splitCamel(), TestDefaultPrefix(), TestFriendlyNamePreservesInitialisms(), TestIndefiniteArticle() (+3 more)

### Community 3 - "output.go"
Cohesion: 0.06
Nodes (68): main(), bytes.Buffer, embed.FS, github.com/spf13/cobra.Command, classifyExit(), Execute(), joinComma(), newInitCmd() (+60 more)

### Community 4 - "LoadConfig"
Cohesion: 0.08
Nodes (45): ActionSpec, Engine, io/fs.FS, cfgBool(), cfgInt(), cfgStr(), AppConfig, SempConfig (+37 more)

### Community 5 - "SEMP Workflow Automation - How-To Guide"
Cohesion: 0.04
Nodes (49): 10. Common Scenario Examples, 11. Troubleshooting, 1. Introduction, 2. Prerequisites, 3. Getting Started, 4. Project Structure, 5.1 SEMP Connection, 5.2 Global Variables (global_vars) (+41 more)

### Community 6 - "SEMP Workflow Automation — 操作指南"
Cohesion: 0.05
Nodes (42): 10. 常見情境範例, 11. 故障排除, 1. 簡介, 2. 系統需求, 3. 開始使用, 4. 專案結構, 5.1 SEMP 連線設定, 5.2 全域變數（global_vars） (+34 more)

### Community 7 - "coerceField"
Cohesion: 0.38
Nodes (8): coerceField(), fieldNames(), TestCoerceFieldIntErrorNamesTheAttribute(), TestCoerceFieldNestedObject(), TestCoerceFieldNestedObjectInnerError(), TestCoerceFieldNestedObjectWrongShape(), TestCoerceFieldScalars(), threshold()

### Community 8 - "NewEngine"
Cohesion: 0.15
Nodes (30): github.com/nikolalohinski/gonja/v2/config.Config, TestRenderMalformedTemplateError(), TestRenderMapRecursionError(), TestRenderSliceRecursionError(), TestRenderUndefinedIndexBranch(), TestValidateInputsIntegerCoercionVariants(), TestValidateInputsOptionalSkipped(), TestValidateInputsUnknownTypePassthrough() (+22 more)

### Community 9 - "index.md"
Cohesion: 0.10
Nodes (15): `acl_profile_subscribe_exception`, `acl_profile_subscribe_exception.add`, `acl_profile_subscribe_exception.delete`, `authentication_oauth_rs_claim`, `authentication_oauth_rs_claim.add`, `authentication_oauth_rs_claim.delete`, `broker`, `broker.update` (+7 more)

### Community 10 - "testing.T"
Cohesion: 0.17
Nodes (31): testing.T, runCLI(), TestInitCopiesBundledTemplates(), TestListModules(), TestListModulesRemovesStaleReferencePages(), TestListModulesWritesFile(), TestMissingConfigFlagIsUsageError(), TestRunConfigError() (+23 more)

### Community 11 - "absent"
Cohesion: 0.14
Nodes (37): absent(), existing(), sempErr(), exec(), Client, TestAclProfileAddDelete(), TestClientProfileAddCoercion(), TestClientProfileUpdateNoFields() (+29 more)

### Community 12 - "NewTemplateError"
Cohesion: 0.11
Nodes (17): ConfigError, SEMPError, TemplateError, ValidationError, WorkflowError, TestClassifyExit(), TestWorkflowAndValidationErrorMessages(), IsWorkflowError() (+9 more)

### Community 13 - "dev.sh"
Cohesion: 0.20
Nodes (27): CGO_ENABLED, load_local_env(), main(), NO_COLOR, _report_only(), _run(), run_logged(), dev.sh script (+19 more)

### Community 14 - "registerObject"
Cohesion: 0.09
Nodes (12): init(), init(), init(), init(), init(), init(), init(), init() (+4 more)

### Community 15 - "dev.ps1"
Cohesion: 0.24
Nodes (22): Get-BinPath(), Have(), Import-LocalEnv(), Invoke-Logged(), Invoke-Task(), Ok(), Step(), Task-All() (+14 more)

### Community 16 - "objectDesc"
Cohesion: 0.14
Nodes (15): applyMsgDefaults(), objectDesc, objectDesc, hopKeys(), orDefault(), addMode, collectionAdd, collectionDelete (+7 more)

### Community 17 - "Parse"
Cohesion: 0.19
Nodes (15): Parsed, cells(), Flags, Parse(), readTable(), splitList(), TestParse_NoTablesIsEmptyNotError(), TestParse_RealSpecificationHasPlausibleRowCounts() (+7 more)

### Community 18 - "`acl_profile`"
Cohesion: 0.40
Nodes (4): `acl_profile`, `acl_profile.add`, `acl_profile.delete`, `acl_profile.update`

### Community 19 - "Release Checklist"
Cohesion: 0.17
Nodes (12): 1. Run the test suite, 2. Run vet, lint, and the vulnerability scan, 3. Regenerate the descriptors and the module reference, 4. Build the release binary, 5. Commit the release, 6. Tag the release, 7. The tag pipeline publishes the release automatically, Common Commands (+4 more)

### Community 20 - "run"
Cohesion: 0.06
Nodes (61): go/ast.File, strings.Builder, Kind, ParentHop, Platform, TestUnionAttributesTagsThePlatformSpecificOneAndNotesItsDescription(), TestUnionEnumKeepsTheWiderSetsOrder(), TestUnionTagsObjectsByWhichPlatformDefinesThem() (+53 more)

### Community 21 - "Template: `app-inbound`"
Cohesion: 0.18
Nodes (11): Action Sequence, `delete` (7 actions), Delete the same resources, Dry-run then execute, Example Workflow Entry, Inputs, `new-seq` / `new-non-seq` (14 actions), Optional (with defaults) (+3 more)

### Community 22 - "Template: `app-outbound`"
Cohesion: 0.18
Nodes (11): Action Sequence, `delete` (6 actions), Delete the same resources, Dry-run then execute, Example Workflow Entry, Inputs, `new-seq` / `new-non-seq` (11 actions), Optional (with defaults) (+3 more)

### Community 23 - "`authentication_kerberos_realm`"
Cohesion: 0.40
Nodes (4): `authentication_kerberos_realm`, `authentication_kerberos_realm.add`, `authentication_kerberos_realm.delete`, `authentication_kerberos_realm.update`

### Community 24 - "Solace SEMP Workflow Automation"
Cohesion: 0.25
Nodes (7): Backward compatibility, Build & test (run from the repo root), Documentation rules, Naming, Project conventions, Solace SEMP Workflow Automation, Specs

### Community 25 - "`authentication_oauth_profile`"
Cohesion: 0.40
Nodes (4): `authentication_oauth_profile`, `authentication_oauth_profile.add`, `authentication_oauth_profile.delete`, `authentication_oauth_profile.update`

### Community 26 - "Solace SEMP Workflow Automation"
Cohesion: 0.25
Nodes (8): Build from source (requires Go 1.26+), Documentation, How It Works, Install a prebuilt binary (recommended), Quick Start, Solace SEMP Workflow Automation, Status, What It Does

### Community 28 - "Roadmap"
Cohesion: 0.29
Nodes (7): Deferred: SEMP v1, Design decisions, Naming, Open work, Roadmap, Stages, Why

### Community 29 - "`authentication_oauth_provider`"
Cohesion: 0.40
Nodes (4): `authentication_oauth_provider`, `authentication_oauth_provider.add`, `authentication_oauth_provider.delete`, `authentication_oauth_provider.update`

### Community 30 - "Extract"
Cohesion: 0.23
Nodes (13): Extract(), specPath(), TestCheckPrefixesAcceptsAnAliasedNameOverBudget(), TestCheckPrefixesReportsACollision(), TestCheckPrefixesReportsAnUnaliasedNameOverBudget(), TestExtractAppliesEveryLegacyOverlay(), TestExtractOnTheRealSpecifications(), TestKindRuntimeConstMatchesTheRuntimeIdentifiers() (+5 more)

### Community 34 - "`authorization_group`"
Cohesion: 0.40
Nodes (4): `authorization_group`, `authorization_group.add`, `authorization_group.delete`, `authorization_group.update`

### Community 35 - "`bridge`"
Cohesion: 0.40
Nodes (4): `bridge`, `bridge.add`, `bridge.delete`, `bridge.update`

### Community 50 - "helpers_test.go"
Cohesion: 0.13
Nodes (20): TestCoerceBoolDefaultBranch(), CheckNameLength(), CleanPayload(), CoerceBool(), Enc(), isUnreserved(), TestAttributesThatDoNotInheritTheObviousLimit(), TestCheckNameLength() (+12 more)

### Community 72 - "`bridge_tls_trusted_common_name`"
Cohesion: 0.50
Nodes (3): `bridge_tls_trusted_common_name`, `bridge_tls_trusted_common_name.add`, `bridge_tls_trusted_common_name.delete`

### Community 73 - "Doc"
Cohesion: 0.22
Nodes (12): Definition, Info, Schema, attributes(), extractBroker(), TestAttributesExcludesPathKeysAndReadOnlyProperties(), TestExtractBrokerRequiresAPatchOperation(), Doc (+4 more)

### Community 74 - "`oauth_profile_client_allowed_host`"
Cohesion: 0.50
Nodes (3): `oauth_profile_client_allowed_host`, `oauth_profile_client_allowed_host.add`, `oauth_profile_client_allowed_host.delete`

### Community 79 - "checkLegacyMatched"
Cohesion: 0.33
Nodes (4): legacyOverlay, checkLegacyMatched(), TestCheckLegacyMatchedErrorsWhenAnOverlayPrefixMatchesNoObject(), LegacyPrefixes()

### Community 82 - "extract_test.go"
Cohesion: 0.26
Nodes (14): describe(), extractOne(), bodyParam(), TestApplyLegacyLeavesAnObjectWithNoOverlayUntouched(), TestApplyLegacyPinsOverlayFieldsAndDropsSkippedAttributes(), TestDescribeAppendsTheOperationalNoteForEachFlag(), TestDescribeTrimsBoilerplateAfterTheFirstBlankLine(), TestExtractCollectionHasUpdateComesFromTheItemPathOnly() (+6 more)

### Community 104 - "extract.go"
Cohesion: 0.47
Nodes (8): extractCollection(), firstSentence(), isKeySegment(), itemPaths(), keyNames(), ownKeyNames(), parentDefinition(), pathSegments()

### Community 110 - "ActionResult"
Cohesion: 0.28
Nodes (11): ActionResult, NewActionResult(), dryrun(), failed(), ok(), sempCodeOf(), skipped(), capitalizeFirst() (+3 more)

### Community 114 - "`bridge_remote_msg_vpn`"
Cohesion: 0.40
Nodes (4): `bridge_remote_msg_vpn`, `bridge_remote_msg_vpn.add`, `bridge_remote_msg_vpn.delete`, `bridge_remote_msg_vpn.update`

### Community 115 - "`broker_proxy`"
Cohesion: 0.40
Nodes (4): `broker_proxy`, `broker_proxy.add`, `broker_proxy.delete`, `broker_proxy.update`

### Community 116 - "`cert_authority`"
Cohesion: 0.40
Nodes (4): `cert_authority`, `cert_authority.add`, `cert_authority.delete`, `cert_authority.update`

### Community 117 - "`cert_matching_rule`"
Cohesion: 0.40
Nodes (4): `cert_matching_rule`, `cert_matching_rule.add`, `cert_matching_rule.delete`, `cert_matching_rule.update`

### Community 118 - "`cert_matching_rule_attribute_filter`"
Cohesion: 0.40
Nodes (4): `cert_matching_rule_attribute_filter`, `cert_matching_rule_attribute_filter.add`, `cert_matching_rule_attribute_filter.delete`, `cert_matching_rule_attribute_filter.update`

### Community 119 - "`client_cert_authority`"
Cohesion: 0.40
Nodes (4): `client_cert_authority`, `client_cert_authority.add`, `client_cert_authority.delete`, `client_cert_authority.update`

### Community 120 - "`client_profile`"
Cohesion: 0.40
Nodes (4): `client_profile`, `client_profile.add`, `client_profile.delete`, `client_profile.update`

### Community 121 - "`client_username`"
Cohesion: 0.40
Nodes (4): `client_username`, `client_username.add`, `client_username.delete`, `client_username.update`

### Community 122 - "`distributed_cache`"
Cohesion: 0.40
Nodes (4): `distributed_cache`, `distributed_cache.add`, `distributed_cache.delete`, `distributed_cache.update`

### Community 123 - "`distributed_cache_cluster`"
Cohesion: 0.40
Nodes (4): `distributed_cache_cluster`, `distributed_cache_cluster.add`, `distributed_cache_cluster.delete`, `distributed_cache_cluster.update`

### Community 124 - "`distributed_cache_cluster_instance`"
Cohesion: 0.40
Nodes (4): `distributed_cache_cluster_instance`, `distributed_cache_cluster_instance.add`, `distributed_cache_cluster_instance.delete`, `distributed_cache_cluster_instance.update`

### Community 125 - "`dmr_bridge`"
Cohesion: 0.40
Nodes (4): `dmr_bridge`, `dmr_bridge.add`, `dmr_bridge.delete`, `dmr_bridge.update`

### Community 126 - "`dmr_cluster`"
Cohesion: 0.40
Nodes (4): `dmr_cluster`, `dmr_cluster.add`, `dmr_cluster.delete`, `dmr_cluster.update`

### Community 127 - "`dmr_cluster_cert_matching_rule`"
Cohesion: 0.40
Nodes (4): `dmr_cluster_cert_matching_rule`, `dmr_cluster_cert_matching_rule.add`, `dmr_cluster_cert_matching_rule.delete`, `dmr_cluster_cert_matching_rule.update`

### Community 128 - "`dmr_cluster_cert_matching_rule_filter`"
Cohesion: 0.40
Nodes (4): `dmr_cluster_cert_matching_rule_filter`, `dmr_cluster_cert_matching_rule_filter.add`, `dmr_cluster_cert_matching_rule_filter.delete`, `dmr_cluster_cert_matching_rule_filter.update`

### Community 129 - "`dmr_cluster_link`"
Cohesion: 0.40
Nodes (4): `dmr_cluster_link`, `dmr_cluster_link.add`, `dmr_cluster_link.delete`, `dmr_cluster_link.update`

### Community 130 - "`domain_cert_authority`"
Cohesion: 0.40
Nodes (4): `domain_cert_authority`, `domain_cert_authority.add`, `domain_cert_authority.delete`, `domain_cert_authority.update`

### Community 131 - "`jndi_connection_factory`"
Cohesion: 0.40
Nodes (4): `jndi_connection_factory`, `jndi_connection_factory.add`, `jndi_connection_factory.delete`, `jndi_connection_factory.update`

### Community 132 - "`jndi_queue`"
Cohesion: 0.40
Nodes (4): `jndi_queue`, `jndi_queue.add`, `jndi_queue.delete`, `jndi_queue.update`

### Community 133 - "`jndi_topic`"
Cohesion: 0.40
Nodes (4): `jndi_topic`, `jndi_topic.add`, `jndi_topic.delete`, `jndi_topic.update`

### Community 134 - "`kafka_receiver`"
Cohesion: 0.40
Nodes (4): `kafka_receiver`, `kafka_receiver.add`, `kafka_receiver.delete`, `kafka_receiver.update`

### Community 135 - "`kafka_receiver_topic_binding`"
Cohesion: 0.40
Nodes (4): `kafka_receiver_topic_binding`, `kafka_receiver_topic_binding.add`, `kafka_receiver_topic_binding.delete`, `kafka_receiver_topic_binding.update`

### Community 136 - "`kafka_sender`"
Cohesion: 0.40
Nodes (4): `kafka_sender`, `kafka_sender.add`, `kafka_sender.delete`, `kafka_sender.update`

### Community 137 - "`kafka_sender_queue_binding`"
Cohesion: 0.40
Nodes (4): `kafka_sender_queue_binding`, `kafka_sender_queue_binding.add`, `kafka_sender_queue_binding.delete`, `kafka_sender_queue_binding.update`

### Community 138 - "`mqtt_retain_cache`"
Cohesion: 0.40
Nodes (4): `mqtt_retain_cache`, `mqtt_retain_cache.add`, `mqtt_retain_cache.delete`, `mqtt_retain_cache.update`

### Community 139 - "`mqtt_session`"
Cohesion: 0.40
Nodes (4): `mqtt_session`, `mqtt_session.add`, `mqtt_session.delete`, `mqtt_session.update`

### Community 140 - "`mqtt_session_subscription`"
Cohesion: 0.40
Nodes (4): `mqtt_session_subscription`, `mqtt_session_subscription.add`, `mqtt_session_subscription.delete`, `mqtt_session_subscription.update`

### Community 141 - "`msg_vpn`"
Cohesion: 0.40
Nodes (4): `msg_vpn`, `msg_vpn.add`, `msg_vpn.delete`, `msg_vpn.update`

### Community 142 - "`oauth_profile`"
Cohesion: 0.40
Nodes (4): `oauth_profile`, `oauth_profile.add`, `oauth_profile.delete`, `oauth_profile.update`

### Community 143 - "`oauth_profile_access_group_vpn_exception`"
Cohesion: 0.40
Nodes (4): `oauth_profile_access_group_vpn_exception`, `oauth_profile_access_group_vpn_exception.add`, `oauth_profile_access_group_vpn_exception.delete`, `oauth_profile_access_group_vpn_exception.update`

### Community 144 - "`oauth_profile_access_level_group`"
Cohesion: 0.40
Nodes (4): `oauth_profile_access_level_group`, `oauth_profile_access_level_group.add`, `oauth_profile_access_level_group.delete`, `oauth_profile_access_level_group.update`

### Community 145 - "`oauth_profile_client_authorization_parameter`"
Cohesion: 0.40
Nodes (4): `oauth_profile_client_authorization_parameter`, `oauth_profile_client_authorization_parameter.add`, `oauth_profile_client_authorization_parameter.delete`, `oauth_profile_client_authorization_parameter.update`

### Community 146 - "`oauth_profile_default_vpn_exception`"
Cohesion: 0.40
Nodes (4): `oauth_profile_default_vpn_exception`, `oauth_profile_default_vpn_exception.add`, `oauth_profile_default_vpn_exception.delete`, `oauth_profile_default_vpn_exception.update`

### Community 147 - "`proxy`"
Cohesion: 0.40
Nodes (4): `proxy`, `proxy.add`, `proxy.delete`, `proxy.update`

### Community 148 - "`queue`"
Cohesion: 0.40
Nodes (4): `queue`, `queue.add`, `queue.delete`, `queue.update`

### Community 149 - "`queue_template`"
Cohesion: 0.40
Nodes (4): `queue_template`, `queue_template.add`, `queue_template.delete`, `queue_template.update`

### Community 150 - "`rdp`"
Cohesion: 0.40
Nodes (4): `rdp`, `rdp.add`, `rdp.delete`, `rdp.update`

### Community 151 - "`rdp_qb`"
Cohesion: 0.40
Nodes (4): `rdp_qb`, `rdp_qb.add`, `rdp_qb.delete`, `rdp_qb.update`

### Community 152 - "`rdp_qb_header`"
Cohesion: 0.40
Nodes (4): `rdp_qb_header`, `rdp_qb_header.add`, `rdp_qb_header.delete`, `rdp_qb_header.update`

### Community 153 - "`rdp_qb_protected_header`"
Cohesion: 0.40
Nodes (4): `rdp_qb_protected_header`, `rdp_qb_protected_header.add`, `rdp_qb_protected_header.delete`, `rdp_qb_protected_header.update`

### Community 154 - "`rdp_rc`"
Cohesion: 0.40
Nodes (4): `rdp_rc`, `rdp_rc.add`, `rdp_rc.delete`, `rdp_rc.update`

### Community 155 - "`replay_log`"
Cohesion: 0.40
Nodes (4): `replay_log`, `replay_log.add`, `replay_log.delete`, `replay_log.update`

### Community 156 - "`replicated_topic`"
Cohesion: 0.40
Nodes (4): `replicated_topic`, `replicated_topic.add`, `replicated_topic.delete`, `replicated_topic.update`

### Community 157 - "`telemetry_profile`"
Cohesion: 0.40
Nodes (4): `telemetry_profile`, `telemetry_profile.add`, `telemetry_profile.delete`, `telemetry_profile.update`

### Community 158 - "`telemetry_profile_trace_filter`"
Cohesion: 0.40
Nodes (4): `telemetry_profile_trace_filter`, `telemetry_profile_trace_filter.add`, `telemetry_profile_trace_filter.delete`, `telemetry_profile_trace_filter.update`

### Community 159 - "`topic_endpoint`"
Cohesion: 0.40
Nodes (4): `topic_endpoint`, `topic_endpoint.add`, `topic_endpoint.delete`, `topic_endpoint.update`

### Community 160 - "`topic_endpoint_template`"
Cohesion: 0.40
Nodes (4): `topic_endpoint_template`, `topic_endpoint_template.add`, `topic_endpoint_template.delete`, `topic_endpoint_template.update`

### Community 161 - "`virtual_hostname`"
Cohesion: 0.40
Nodes (4): `virtual_hostname`, `virtual_hostname.add`, `virtual_hostname.delete`, `virtual_hostname.update`

### Community 162 - "`acl_profile_client_connect_exception`"
Cohesion: 0.50
Nodes (3): `acl_profile_client_connect_exception`, `acl_profile_client_connect_exception.add`, `acl_profile_client_connect_exception.delete`

### Community 163 - "Test Documentation"
Cohesion: 0.15
Nodes (13): Aggregate dev-script tasks, Application, Coverage, Generator, Integration tests (live broker required), Module Registry, Overview, Result Status Reference (+5 more)

### Community 164 - "`acl_profile_subscribe_share_name_exception`"
Cohesion: 0.50
Nodes (3): `acl_profile_subscribe_share_name_exception`, `acl_profile_subscribe_share_name_exception.add`, `acl_profile_subscribe_share_name_exception.delete`

### Community 165 - "`acl_subscribe_exception`"
Cohesion: 0.50
Nodes (3): `acl_subscribe_exception`, `acl_subscribe_exception.add`, `acl_subscribe_exception.delete`

### Community 166 - "validate_test.go"
Cohesion: 0.31
Nodes (15): objectDesc, TestExportedValidateMatchesTheVerb(), TestInvalidArgsFailWithoutIssuingAnyRequest(), TestValidateAcceptsAWellFormedCall(), TestValidateChecksInsideNestedGroups(), TestValidateEnforcesAttributeCoDependencies(), TestValidateEnforcesEnums(), TestValidateRejectsBodyArgumentsOnDelete() (+7 more)

### Community 167 - "`bridge_remote_subscription`"
Cohesion: 0.50
Nodes (3): `bridge_remote_subscription`, `bridge_remote_subscription.add`, `bridge_remote_subscription.delete`

### Community 168 - "Load"
Cohesion: 0.31
Nodes (8): Load(), RefName(), TestBodyDefinition(), TestLoadErrors(), TestLoadValidMinimalSpec(), TestPathParams(), TestRefName(), writeSpec()

### Community 169 - "`cert_authority_ocsp_tls_trusted_common_name`"
Cohesion: 0.50
Nodes (3): `cert_authority_ocsp_tls_trusted_common_name`, `cert_authority_ocsp_tls_trusted_common_name.add`, `cert_authority_ocsp_tls_trusted_common_name.delete`

### Community 170 - "`cert_matching_rule_condition`"
Cohesion: 0.50
Nodes (3): `cert_matching_rule_condition`, `cert_matching_rule_condition.add`, `cert_matching_rule_condition.delete`

### Community 173 - "`distributed_cache_cluster_topic`"
Cohesion: 0.50
Nodes (3): `distributed_cache_cluster_topic`, `distributed_cache_cluster_topic.add`, `distributed_cache_cluster_topic.delete`

### Community 174 - "`distributed_cache_home_cluster`"
Cohesion: 0.50
Nodes (3): `distributed_cache_home_cluster`, `distributed_cache_home_cluster.add`, `distributed_cache_home_cluster.delete`

### Community 175 - "`distributed_cache_home_cluster_prefix`"
Cohesion: 0.50
Nodes (3): `distributed_cache_home_cluster_prefix`, `distributed_cache_home_cluster_prefix.add`, `distributed_cache_home_cluster_prefix.delete`

### Community 176 - "`dmr_cluster_cert_matching_rule_condition`"
Cohesion: 0.50
Nodes (3): `dmr_cluster_cert_matching_rule_condition`, `dmr_cluster_cert_matching_rule_condition.add`, `dmr_cluster_cert_matching_rule_condition.delete`

### Community 177 - "`dmr_cluster_link_attribute`"
Cohesion: 0.50
Nodes (3): `dmr_cluster_link_attribute`, `dmr_cluster_link_attribute.add`, `dmr_cluster_link_attribute.delete`

### Community 178 - "`dmr_cluster_link_remote_address`"
Cohesion: 0.50
Nodes (3): `dmr_cluster_link_remote_address`, `dmr_cluster_link_remote_address.add`, `dmr_cluster_link_remote_address.delete`

### Community 179 - "`dmr_cluster_link_tls_trusted_common_name`"
Cohesion: 0.50
Nodes (3): `dmr_cluster_link_tls_trusted_common_name`, `dmr_cluster_link_tls_trusted_common_name.add`, `dmr_cluster_link_tls_trusted_common_name.delete`

### Community 180 - "newTestEngine"
Cohesion: 0.14
Nodes (23): Engine, engineFake, TestResolveTemplateNotFoundListsAvailable(), TestRunFailFastStopsAfterFailedWorkflow(), TestRunFailFastStopsOnTemplateError(), TestRunWorkflowInputRenderError(), TestRunWorkflowMissingRequiredInput(), newTestEngine() (+15 more)

### Community 181 - "Path"
Cohesion: 0.36
Nodes (6): Path(), ResolveVPN(), TestPath(), TestResolveVPN(), VPNSeg(), Seg

### Community 182 - "`oauth_profile_resource_server_required_claim`"
Cohesion: 0.50
Nodes (3): `oauth_profile_resource_server_required_claim`, `oauth_profile_resource_server_required_claim.add`, `oauth_profile_resource_server_required_claim.delete`

### Community 183 - "`q_sub`"
Cohesion: 0.50
Nodes (3): `q_sub`, `q_sub.add`, `q_sub.delete`

### Community 184 - "`rdp_rc_jwt_claim`"
Cohesion: 0.50
Nodes (3): `rdp_rc_jwt_claim`, `rdp_rc_jwt_claim.add`, `rdp_rc_jwt_claim.delete`

### Community 186 - "`replay_log_topic_filter_subscription`"
Cohesion: 0.50
Nodes (3): `replay_log_topic_filter_subscription`, `replay_log_topic_filter_subscription.add`, `replay_log_topic_filter_subscription.delete`

### Community 187 - "`sequenced_topic`"
Cohesion: 0.50
Nodes (3): `sequenced_topic`, `sequenced_topic.add`, `sequenced_topic.delete`

### Community 188 - "`telemetry_profile_receiver_acl_exception`"
Cohesion: 0.50
Nodes (3): `telemetry_profile_receiver_acl_exception`, `telemetry_profile_receiver_acl_exception.add`, `telemetry_profile_receiver_acl_exception.delete`

### Community 189 - "`telemetry_profile_trace_filter_subscription`"
Cohesion: 0.50
Nodes (3): `telemetry_profile_trace_filter_subscription`, `telemetry_profile_trace_filter_subscription.add`, `telemetry_profile_trace_filter_subscription.delete`

### Community 190 - "Stringify"
Cohesion: 0.24
Nodes (8): argStr(), AttrEqual(), numericEqual(), sliceEqual(), TestAttrEqual(), toAnySlice(), Stringify(), TestStringify()

### Community 191 - "buildAttribute"
Cohesion: 0.33
Nodes (7): applyCombos(), buildAttribute(), parseDefault(), TestBuildAttributeRejectsAnUnsupportedType(), TestBuildAttributeResolvesANestedRefAndAppliesCombosByNestedDefinition(), TestParseDefaultReadsTheProseSentence(), Combination

### Community 192 - "checkValue"
Cohesion: 0.36
Nodes (7): checkCombinations(), checkValue(), contains(), objectDesc, quoteAll(), sortedArgNames(), paramMode

### Community 193 - "`client_cert_authority_ocsp_cn`"
Cohesion: 0.50
Nodes (3): `client_cert_authority_ocsp_cn`, `client_cert_authority_ocsp_cn.add`, `client_cert_authority_ocsp_cn.delete`

### Community 194 - "`acl_profile_publish_exception`"
Cohesion: 0.50
Nodes (3): `acl_profile_publish_exception`, `acl_profile_publish_exception.add`, `acl_profile_publish_exception.delete`

### Community 195 - "`acl_publish_exception`"
Cohesion: 0.50
Nodes (3): `acl_publish_exception`, `acl_publish_exception.add`, `acl_publish_exception.delete`

### Community 196 - "`authentication_oauth_client_claim`"
Cohesion: 0.50
Nodes (3): `authentication_oauth_client_claim`, `authentication_oauth_client_claim.add`, `authentication_oauth_client_claim.delete`

### Community 197 - "`rdp_rc_tls_cn`"
Cohesion: 0.50
Nodes (3): `rdp_rc_tls_cn`, `rdp_rc_tls_cn.add`, `rdp_rc_tls_cn.delete`

## Knowledge Gaps
- **357 isolated node(s):** `solace-semp-workflow`, `InputSpec`, `NO_COLOR`, `CGO_ENABLED`, `test.sh script` (+352 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **81 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `registerObject()` connect `registerObject` to `output.go`, `objectDesc`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`, `init`?**
  _High betweenness centrality (0.130) - this node is a cross-community bridge._
- **Why does `register()` connect `output.go` to `driver_test.go`, `registerObject`?**
  _High betweenness centrality (0.077) - this node is a cross-community bridge._
- **Why does `TestRegisterPanicsOnADuplicateName()` connect `driver_test.go` to `testing.T`, `output.go`?**
  _High betweenness centrality (0.075) - this node is a cross-community bridge._
- **Are the 87 inferred relationships involving `registerObject()` (e.g. with `init()` and `init()`) actually correct?**
  _`registerObject()` has 87 INFERRED edges - model-reasoned connections that need verification._
- **Are the 41 inferred relationships involving `absent()` (e.g. with `TestAddAcceptsRequiredFalseAndZero()` and `TestAddBuildsGlobalScopedPathWithoutVPNHop()`) actually correct?**
  _`absent()` has 41 INFERRED edges - model-reasoned connections that need verification._
- **Are the 4 inferred relationships involving `exec()` (e.g. with `TestIntegrationAccessControlLifecycle()` and `TestIntegrationQueueSubscriptionLifecycle()`) actually correct?**
  _`exec()` has 4 INFERRED edges - model-reasoned connections that need verification._
- **Are the 9 inferred relationships involving `runCLI()` (e.g. with `newRootCmd()` and `TestInitMkdirAllFailure()`) actually correct?**
  _`runCLI()` has 9 INFERRED edges - model-reasoned connections that need verification._