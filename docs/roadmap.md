# Roadmap

This repository replaces the hand-written module layer of the earlier
`SEMP-Workflow_Automation` implementation with modules generated from the
committed SEMP schemas in [`specs/`](../specs/MANIFEST.yaml).

## Why

The SEMP v2 config API exposes **85 creatable object types plus the Broker
singleton**, which is **225 actions** (85 add, 54 update, 85 delete, 1
broker.update) over 2,054 attributes. The previous implementation covered 24
actions across 10 object families, all of them hand-written. Writing the
remaining 200 by hand was not viable, and would have gone stale on the next
broker release.

Two facts from the schemas shape the design:

- The appliance and software specs are **not** subsets of one another. Software
  adds 10 paths (Kafka senders and receivers with their bindings, Kerberos
  realms); the appliance adds 10 attributes and one wider enum. The generator
  therefore consumes the union of both.
- Per-attribute metadata (identifying, const, required, read-only, write-only,
  requires-disable, auto-disable, deprecated, opaque) exists **only** as Markdown
  tables inside each operation's description, so the extractor parses those
  rather than reading a structured field.

## Stages

| Stage | Content | State |
|---|---|---|
| M0 | Fork the engine, client, templating, config, output and CLI packages; rewrite the module path; commit the specs; make the client root-relative; add the composite-key path helper; mask secrets in output | Landed |
| M1 | Table-driven driver, spec-driven coercion, and the 10 legacy object families re-expressed as descriptors, proving the driver against the carried-over test suite | Landed |
| M2 | `tools/semp-gen`: spec to intermediate representation to generated descriptors, all 225 actions, with a drift gate in `full` and CI | Landed |
| M3 | Generated per-family module reference; local argument validation consumed by `validate`; curated name and topic length limits; curated HOWTO tables | Landed |
| M3a | Per-family integration fixtures, platform-gated | Open |
| M4 | Cutover: verify every existing template against the new binary, then retire the old repository | Open |

### Open work

- **Per-family integration fixtures.** `testdata/integration/test-artifacts.yaml` exercises
  the 6 legacy families against a live broker. The other 80 have no live coverage, and
  appliance-only and software-only objects cannot both be exercised on one broker, so the
  suite needs splitting and platform-gating.
- **Length limits for name-valued body attributes.** 59 identifying attributes have curated
  limits. Around 36 name-shaped body attributes do not -- `dead_msg_queue`, `localTopic`, the
  various claim and LDAP identifiers. An over-long value is still rejected by the broker;
  the cost is a round trip rather than a local error.
- **SEMP v1 fallback**, deferred -- see below.

## Design decisions

- **Three drivers, not 225 modules.** `collectionAdd`, `collectionUpdate` and
  `collectionDelete` in `internal/modules/driver.go` implement every action,
  parameterised by one data-only `objectDesc`. Generated files contain
  descriptors and nothing else, so a fix to key encoding, payload filtering or
  idempotency is made once.
- **Update converges.** An update reads current state and sends only genuinely
  differing attributes, reporting `skipped` when the object already matches. The
  previous implementation always issued the PATCH and always reported `changed`.
  Write-only and opaque attributes cannot be read back, so they are always
  re-sent.
- **PUT is not exposed.** SEMP's PUT resets every attribute the request omits to
  its default, which is a footgun for a tool whose contract is "change what I
  named and leave the rest alone".
- **Requires-Disable is documented, not orchestrated.** It occurs exactly once in
  the whole specification, on the Broker object, for the two SEMP listener ports.
  Disabling those to apply a change could strand the client issuing it, so the
  broker's own error is surfaced instead.
- **The 10 original action names still resolve**, along with their parameter
  names, defaults and result wording, so existing templates and configs run
  unchanged. `internal/modules/modules_test.go` was carried over from the old
  repository to enforce that.

## Naming

Operator-facing names match the Solace Terraform provider
(`SolaceProducts/solacebroker`), so one spelling serves both tools.

A registry prefix is the snake_case form of the SEMP definition name --
`MsgVpnQueue` gives `msg_vpn_queue` -- which is the provider's resource name
without its `solacebroker_` prefix. Attribute names follow the same rule, so
`egressEnabled` is written `egress_enabled`.

There is no alias table. All 86 prefixes are mechanical, and the two reasons the
earlier scheme needed exceptions are both gone: retaining the VPN scope separates
`Proxy` (`proxy`) from `MsgVpnProxy` (`msg_vpn_proxy`), which used to collide, and
there is no length budget now that Terraform's own lengths are the target -- the
longest is 73 characters.

The descriptors carry both spellings. `Name` is the SEMP attribute, which is what
goes on the wire and what a GET returns; `Canon` is the snake_case name shown to
operators. Keeping `Name` as the wire identity is what leaves the payload builder,
the diff and the curated length limits untouched by the rename, and what makes
accepting the old camelCase spelling free rather than a second code path.

A generation-time round-trip check asserts that every name survives snake_case and
back. The scheme depends on no SEMP name containing two adjacent capitals (Solace
writes `Tls`, not `TLS`); that holds across both specifications today, but it is a
property of the published schema rather than a guarantee, so it is verified on
every run instead of assumed.

## Deferred: SEMP v1

SEMP v2 config does not reach broker and system administration: broker-level
`authentication` (LDAP profiles), `redundancy`, broker-level `replication`,
`logging`, `syslog`, `system`, `console`, CLI `username`, `router-name`,
`schedule`, `memory-event`, `compression`, the appliance-only `hardware`,
`interface`, `ip`, `dns`, `hostname`, `clock` and `snmp-server` trees, and the
parts of `message-spool`, `service`, `ssl`, `config-sync` and `web-manager` not
surfaced as Broker attributes.

A v1 fallback is deliberately out of scope for now. The v1 request and reply
schemas are committed alongside the v2 specs so the work can start from evidence
when it is picked up.
