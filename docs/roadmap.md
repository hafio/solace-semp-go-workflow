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
| M2 | `tools/semp-gen`: spec to intermediate representation to generated descriptors, all 225 actions | Next |
| M3 | Generated per-family docs and module manifest, curated HOWTO tables, per-family integration fixtures | Planned |
| M4 | Cutover: verify every existing template against the new binary, then retire the old repository | Planned |

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
- **The 10 original action names are preserved**, along with their parameter
  names, defaults and result wording, so existing templates and configs run
  unchanged. `internal/modules/modules_test.go` was carried over from the old
  repository to enforce that.

## Naming

Registry prefixes derive mechanically from the create `operationId`: strip
`create`, split CamelCase, drop a leading `Msg`+`Vpn`, lower-snake-join. A
committed 24-entry alias table overrides that for the 10 legacy names, the one
genuine collision (`Proxy` and `MsgVpnProxy` both reduce to `proxy`, resolved as
`broker_proxy` and `proxy`), and the 13 names that would otherwise exceed 45
characters. The other 62 take the mechanical name, so they are predictable from
the SEMP object without consulting the table.

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
