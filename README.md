# Solace SEMP Workflow Automation

Ansible-style workflow automation for [Solace](https://solace.com/) brokers via the SEMP v2 REST API.

Define reusable, parameterised workflows in YAML and run them against your broker -- every action is idempotent, so re-running the same workflow is always safe.

---

## Status

This repository supersedes the earlier `SEMP-Workflow_Automation` implementation, which
covered 24 hand-written actions across 10 object families. Here the action modules are
**generated from the committed SEMP schemas**, so coverage tracks the API rather than
whatever happened to be written by hand.

The 10 original action names and their parameters are preserved, so existing workflow
templates and configs run unchanged.

Build-out is staged; see [docs/roadmap.md](docs/roadmap.md) for what has landed.

---

## What It Does

- Configure any object the SEMP v2 config API exposes -- queues, topic endpoints, REST
  Delivery Points, subscriptions, ACL profiles, client profiles and usernames, bridges,
  DMR clusters, message VPNs, OAuth profiles, certificate authorities, telemetry profiles,
  Kafka senders and receivers, and broker-level settings
- Preview changes with dry-run mode before touching the broker
- Use Jinja2 templates and global variables to build dynamic, reusable workflows
- Run multiple workflows in a single execution with shared configuration

---

## How It Works

1. You write a **config file** (`config.yaml`) with your broker connection and a list of workflows
2. You write **template files** (`.yaml`) that describe what actions to perform (create queue, add subscription, and so on)
3. You run the tool -- it connects to the broker, checks current state, and only makes the changes needed

Every action is idempotent. An `add` whose object already exists is skipped; an `update`
compares the broker's current state and reports no change when it is already correct. A
second run of the same config reports nothing changed.

---

## Quick Start

A single self-contained binary -- the workflow templates are embedded, so there are no runtime dependencies.

### Install a prebuilt binary (recommended)

Every [release](https://github.com/hafio/Solace-SEMP-Workflow-Automation/releases) attaches a binary for each supported platform -- `semp-workflow-{linux,darwin,windows}-{amd64,arm64}` (Windows adds `.exe`) -- plus a `SHA256SUMS.txt` file. Download the asset for your platform, verify it, and put it on your `PATH`:

```bash
# Linux/macOS -- adjust the asset name for your OS/arch
base=https://github.com/hafio/Solace-SEMP-Workflow-Automation/releases/latest/download
curl -LO "$base/semp-workflow-linux-amd64"
curl -LO "$base/SHA256SUMS.txt"
sha256sum --ignore-missing -c SHA256SUMS.txt  # -> semp-workflow-linux-amd64: OK
install -m 0755 semp-workflow-linux-amd64 /usr/local/bin/semp-workflow
```

On Windows, download `semp-workflow-windows-amd64.exe` and verify it against `SHA256SUMS.txt`.

### Build from source (requires Go 1.26+)

```bash
./scripts/dev.sh build        # -> dist/semp-workflow-<os>-<arch>[.exe]   (Windows: .\scripts\dev.ps1 build)
```

Then run it:

```bash
# See available commands
semp-workflow --help

# Export bundled templates for customisation
semp-workflow init --output-dir templates

# Validate config without connecting to the broker
semp-workflow validate --config config.yaml

# Preview what would change
semp-workflow run --config config.yaml --dry-run

# Execute
semp-workflow run --config config.yaml
```

---

## Documentation

For full technical details -- configuration reference, template authoring, module parameters, examples, and troubleshooting -- see the **[How-To Guide](docs/HOWTO.md)**.

| Document | Contents |
|---|---|
| [docs/HOWTO.md](docs/HOWTO.md) | Full technical guide: configuration, templates, modules, and troubleshooting |
| [docs/HOWTO-zh.md](docs/HOWTO-zh.md) | Chinese translation of the How-To guide |
| Module reference | Generated on demand: `semp-workflow list-modules --output all-modules.md`. A committed per-family reference under `docs/modules/` lands with M3 |
| [docs/roadmap.md](docs/roadmap.md) | Build-out stages and what has landed |
| [docs/TESTS.md](docs/TESTS.md) | Test suite overview, layout, and module registry |
| [docs/template-app-inbound.md](docs/template-app-inbound.md) | `app-inbound` template: inputs, action sequence, examples |
| [docs/template-app-outbound.md](docs/template-app-outbound.md) | `app-outbound` template: inputs, action sequence, examples |
| [docs/release.md](docs/release.md) | Release process and versioning scheme |
| [specs/MANIFEST.yaml](specs/MANIFEST.yaml) | Provenance of the committed SEMP schemas |
