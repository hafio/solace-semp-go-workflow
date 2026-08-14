# SEMP Workflow Automation - How-To Guide

> Platform: Solace PubSub+ (SEMP v2)
> Action modules generated from the SEMP 10.26.0.8755 schemas in `specs/`

---

## Table of Contents

1. [Introduction](#1-introduction)
2. [Prerequisites](#2-prerequisites)
3. [Getting Started](#3-getting-started)
4. [Project Structure](#4-project-structure)
5. [Configuration File](#5-configuration-file)
6. [Workflow Templates](#6-workflow-templates)
7. [CLI Commands](#7-cli-commands)
8. [Built-in Template Reference](#8-built-in-template-reference)
9. [Available Modules](#9-available-modules)
10. [Common Scenario Examples](#10-common-scenario-examples)
11. [Troubleshooting](#11-troubleshooting)

---

## 1. Introduction

SEMP Workflow Automation is an Ansible-like CLI tool for Solace PubSub+ message brokers. Using declarative YAML configuration files, you can batch-create, delete, or update queues, REST Delivery Points (RDPs), and any other object the SEMP v2 config API exposes, without manual UI interaction.

**Key features:**

- **Complete SEMP v2 config coverage**: 225 actions across 86 object types, generated from the broker's own schemas rather than written by hand. That includes broker-level objects as well as message-VPN ones: message VPNs themselves, DMR clusters and links, OAuth profiles, certificate authorities, virtual hostnames, and the broker settings object.
- **Idempotent operations**: every action checks current state before acting, so re-running is safe. An `add` whose object exists is skipped; an `update` compares the broker's current values and reports no change when it is already correct.
- **Validated before it leaves your machine**: a misspelled attribute, a value outside an enum, a missing required attribute or an attribute that cannot be changed after creation is reported locally, naming the workflow and task, instead of travelling to the broker to be refused there.
- **Jinja2 variable rendering**: supports dynamic naming and cross-variable references.
- **Dry-run mode**: preview changes without executing them.

**Two behaviours worth knowing if you used an earlier version:**

- An `update` now reports `skipped` when the object already holds the values you asked for, rather than always reporting `changed`. Attributes the broker never returns, such as passwords, cannot be compared and so are always re-sent.
- Arguments are checked against each object's full attribute set. An argument that is not an attribute of that object is an error rather than something passed through to the broker.

---

## 2. Prerequisites

| Requirement | Version |
|---|---|
| Go | 1.26 or higher (only needed to build from source; the prebuilt binary is self-contained) |
| Solace PubSub+ Broker | Any version supporting SEMP v2 API |
| Network | Access to the broker's SEMP management port (default `8080` / `943`) |

---

## 3. Getting Started

`semp-workflow` is a single self-contained binary — the Go runtime and all
dependencies are compiled in, and the workflow templates are embedded via
`//go:embed`, so no interpreter or package install is required to run it.

### Install a prebuilt binary (recommended)

Every [GitHub release](https://github.com/hafio/Solace-SEMP-Workflow-Automation/releases) attaches a binary for each supported platform — `semp-workflow-{linux,darwin,windows}-{amd64,arm64}` (Windows adds `.exe`) — plus a `SHA256SUMS.txt` file. Download the asset for your platform, verify its checksum, and put it on your `PATH`:

```bash
base=https://github.com/hafio/Solace-SEMP-Workflow-Automation/releases/latest/download
curl -LO "$base/semp-workflow-linux-amd64"     # adjust for your OS/arch
curl -LO "$base/SHA256SUMS.txt"
sha256sum --ignore-missing -c SHA256SUMS.txt    # → semp-workflow-linux-amd64: OK
install -m 0755 semp-workflow-linux-amd64 /usr/local/bin/semp-workflow
```

A released binary reports the release tag as its version: running
`semp-workflow --version` prints e.g. `semp-workflow, version v0.4.0`.

### Build from source (Go 1.26+)

```bash
./scripts/dev.sh build      # from the repo root; Windows: .\scripts\dev.ps1 build
# → produces dist/semp-workflow-<os>-<arch>[.exe] for the host (e.g. dist/semp-workflow-linux-amd64)
```

Or build/run directly with the Go toolchain:

```bash
go build ./cmd/semp-workflow          # or: go run ./cmd/semp-workflow
```

To inspect what drives the binary size, run the report-only `size` task. It
builds a fresh binary and prints a breakdown by package/module via
[go-size-analyzer](https://github.com/Zxilly/go-size-analyzer):

```bash
./scripts/dev.sh size       # Windows: .\scripts\dev.ps1 size
# needs network on first run to fetch the tool (nothing is installed globally)
# no-tool fallback: go tool nm -size -sort=size dist/semp-workflow-<os>-<arch>
```

Then run it:

```bash
semp-workflow --help
```

On first use, export the embedded templates to a local directory for customisation:

```bash
semp-workflow init --output-dir templates
```

---

## 4. Project Structure

```
project-dir/
├── semp-workflow(.exe)       # The tool (single self-contained binary)
├── config.yaml               # Main config (connection + workflow list)
└── templates/                # Optional — only after `init` or when overriding the embedded templates
```

- **`semp-workflow`**: the single self-contained binary (`.exe` on Windows); workflow templates are embedded via `//go:embed`
- **`config.yaml`**: defines the SEMP connection, shared `global_vars`, and the list of workflows to execute
- **`templates/`** (optional): the bundled templates are embedded in the binary; run `init` to export them. Each YAML file holds one or more named templates, each defining input variables and an action sequence

---

## 5. Configuration File

`config.yaml` has three main sections:

### 5.1 SEMP Connection

```yaml
semp:
  host: "https://broker.example.com:943"
  username: "admin"
  password: "admin"
  msg_vpn: "default"
  verify_ssl: false   # set false for self-signed certificates
  timeout: 30         # seconds
```

**Explanation:** The `host` must be the full SEMP v2 management URL including protocol and port. The tool uses HTTP Basic Auth with the provided username/password. All operations are scoped to the specified `msg_vpn`.

| Field | Required | Default | Description |
|---|---|---|---|
| `host` | Yes | -- | Full base URL including port, e.g. `https://broker:8943` |
| `username` | Yes | -- | SEMP admin username |
| `password` | Yes | -- | SEMP admin password |
| `msg_vpn` | Yes | -- | Message VPN to operate on |
| `verify_ssl` | No | `false` | Verify the broker's TLS certificate |
| `timeout` | No | `30` | HTTP request timeout in seconds |

### 5.2 Global Variables (global_vars)

Global variables are available across all workflow inputs via `{{ global_vars.variable_name }}`, providing a single place to manage shared values.

```yaml
global_vars:
  topic_prefix: "SITEA/APP/AIF"
  default_queue_owner: "svc-app-client"
  default_rc_remote_host: "my-backend.example.com"
```

**Explanation:** Global variables are resolved in the first Jinja2 rendering pass. They can also contain `{{ inputs.X }}` references, which get resolved in the second pass. This makes them ideal for naming conventions shared by many workflows.

### 5.3 Workflow List (workflows)

Each workflow entry references a template and provides input values:

```yaml
workflows:
  - template: "app-outbound.new-seq"    # format: filename.TemplateName
    inputs:
      domain: "CENTRAL"                       # required inputs
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-EVENT"
      # optional overrides (uncomment to override template defaults):
      #service_queue_owner: "{{ global_vars.default_queue_owner }}"
```

> **Note**: Template references use the format `filename.TemplateName` — e.g. `app-outbound.new-seq` refers to the template named `new-seq` inside `app-outbound.yaml`.

**Explanation:** Workflows are executed top-to-bottom in the order listed. Each workflow entry is independent — you can mix different templates and inputs freely. Commented-out inputs show available optional overrides; uncomment them to change from the template's default values.

### 5.4 Full Config Example

```yaml
semp:
  host: "https://broker-host:8943"
  username: "admin"
  password: "admin"
  msg_vpn: "default"
  verify_ssl: false
  timeout: 30

global_vars:
  topic_prefix:    "myapp/events"
  default_owner:   "my-client"
  queue_ttl:       1296000

templates_dir: "templates"

workflows:
  - template: "my-queues.create"
    inputs:
      queue_name: "MY-QUEUE"
      sub_topic:  "{{ global_vars.topic_prefix }}/>"

  - template: "my-queues.create"
    inputs:
      queue_name: "MY-OTHER-QUEUE"
      sub_topic:  "other/topic/>"
```

---

## 6. Workflow Templates

Template files define reusable workflows with the following structure:

```yaml
workflow-templates:
  - name: "my-template"

    inputs:
      required:           # required — error if not provided
      - domain
      - system

      optional:           # optional — with default values
        queue_name: "Q-{{ inputs.domain }}-{{ inputs.system }}"
        queue_owner: ""

    actions:
    - name: "Create Queue"
      module: "msg_vpn_queue.add"
      args:
        queue_name: "{{ inputs.queue_name }}"
        owner: "{{ inputs.queue_owner }}"
```

**Explanation:** The template defines a contract: which inputs are required, which are optional (with their defaults), and what actions to perform. Actions execute in order and each one maps to a built-in module (like `msg_vpn_queue.add` or `msg_vpn_rest_delivery_point.delete`). The `args` section supports full Jinja2 expressions, so you can compose dynamic values from inputs and global vars.

### Input Schema

| Key | Format | Description |
|---|---|---|
| `required` | List of strings | Inputs that must be supplied by the caller |
| `optional` | Map of `name: default` | Optional inputs; default can be a literal value or a Jinja2 expression |

An optional input with `null` as its default is included in the schema but has no default value — it is omitted from the resolved context if not provided by the caller.

### Variable Rendering Rules

| Syntax | Usage |
|---|---|
| `{{ inputs.variable_name }}` | Reference an input variable |
| `{{ global_vars.variable_name }}` | Reference a global variable (in defaults only) |
| `{{ inputs.a }}-{{ inputs.b }}` | Compose multiple variables |

### Two-Pass Rendering

Templates are rendered in two passes:

1. **First pass:** `global_vars` context is available. Defaults like `"{{ global_vars.topic_prefix }}"` are resolved.
2. **Second pass:** The full `inputs` dict is available. Defaults like `"DMQ/{{ inputs.queue_name }}"` are resolved.

This two-pass design allows global vars to define naming patterns that reference inputs (e.g. `queue_name_tpl: "Q-{{ inputs.domain }}-{{ inputs.system }}"`). The global var is expanded in the first pass to that raw Jinja string, then the inputs references are resolved in the second pass.

### YAML Anchor Support

Templates support YAML anchors and aliases to share input definitions or action lists:

```yaml
workflow-templates:
  - name: "create-seq"
    inputs:
      required: &required-inputs
        - queue_name
        - sub_topic
      optional: &optional-inputs
        access_type: exclusive
    actions: &create-actions
      - name: "Create Queue"
        module: "msg_vpn_queue.add"
        args:
          queue_name: "{{ inputs.queue_name }}"

  - name: "create-non-seq"
    inputs:
      required: *required-inputs
      optional:
        <<: *optional-inputs
        access_type: non-exclusive  # override one field
    actions: *create-actions
```

---

## 7. CLI Commands

### Top-Level Help

```
$ semp-workflow --help

SEMP Workflow Automation - Ansible-like playbooks for Solace SEMP.

Usage:
  semp-workflow [command]

Available Commands:
  help          Help about any command
  init          Copy bundled workflow templates to a local directory.
  list-modules  List all available action modules.
  run           Execute workflows defined in a config file.
  validate      Validate config and templates without executing.

Flags:
  -h, --help      help for semp-workflow
      --version   version for semp-workflow

Use "semp-workflow [command] --help" for more information about a command.
```

---

### 7.1 Run Workflows

```
$ semp-workflow run --help

Execute workflows defined in a config file.

Usage:
  semp-workflow run [flags]

Flags:
      --check                  Alias for --dry-run.
  -c, --config string          Path to config YAML file.
      --dry-run                Show what would be done without making changes.
  -f, --fail-fast              Stop execution on first failure.
  -h, --help                   help for run
  -t, --templates-dir string   Override the workflow templates directory.
  -v, --verbose                Enable verbose/debug logging.
```

Note: `-c/--config` is required — omitting it errors with `required flag(s) "config" not set` (exit code `2`). `-v/--verbose` is accepted for compatibility, but the Go modules emit no extra debug output.

**Exit codes:**

| Code | Meaning |
|---|---|
| `0` | All workflows completed with no failures |
| `1` | One or more workflow actions failed |
| `2` | Configuration or template error (nothing was executed) |
| `130` | Interrupted by user (`Ctrl+C`) |

**Examples:**
```bash
# Dry-run (no changes made)
semp-workflow run -c config.yaml --dry-run

# Stop on first error, with verbose output
semp-workflow run -c config.yaml --fail-fast --verbose

# Use a custom templates directory
semp-workflow run -c config.yaml --templates-dir ./my-templates
```

---

### 7.2 Validate Configuration

```
$ semp-workflow validate --help

Validate config and templates without executing.

Usage:
  semp-workflow validate [flags]

Flags:
  -c, --config string          Path to config YAML file.
  -h, --help                   help for validate
  -t, --templates-dir string   Override the workflow templates directory.
```

**Explanation:** Checks the whole config offline, without connecting to the broker, so it runs instantly. It verifies that:

1. the config file and every template parse;
2. every `template` reference in the `workflows` list matches a real template;
3. every required input is supplied, and every input resolves (a Jinja2 expression that cannot be resolved, or refers to itself in a cycle, is reported);
4. every action names a registered module; and
5. every action's arguments are valid for that module -- no unknown attributes, values within their allowed set, required attributes present, no create-only attribute in an update, no attribute exceeding its length limit, and no attribute combination the broker forbids.

Every problem found is listed in one run, so a config is fixed in a single pass rather than one failure at a time.

**Example:**
```bash
semp-workflow validate -c config.yaml
```

A failure names the workflow, the task and the attribute:

```
ERROR: workflow 1 (app-inbound.new-seq), task 'Create queue' [msg_vpn_queue.add]: unknown argument: acessType (see docs/modules/msg_vpn_queue.md for the accepted attributes)
```

Run this before `run` against a production broker. It costs nothing and catches the class of mistake that would otherwise be discovered halfway through applying a workflow.

---

### 7.3 List Available Modules

```
$ semp-workflow list-modules --help

List all available action modules.

Usage:
  semp-workflow list-modules [flags]

Flags:
  -h, --help                help for list-modules
  -o, --output string       Write the whole module reference to one Markdown file.
      --output-dir string   Write the module reference as an index plus one file per object family (e.g. docs/modules).
```

**Explanation:** Shows all registered action modules with their descriptions.

`--output-dir` writes the reference the way it is committed in this repository: an `index.md` listing every object family, plus one page per family with a parameter table for each of its actions. This is the form to use -- at full SEMP coverage a single document runs past ten thousand lines. Files for families that no longer exist are removed, so the directory cannot accumulate pages for objects a newer broker schema has dropped.

`--output` still writes everything to one file, which is occasionally useful for searching.

**Examples:**
```bash
semp-workflow list-modules
semp-workflow list-modules --output-dir docs/modules
```

The committed reference lives at [docs/modules/index.md](modules/index.md).

---

### 7.4 Export Bundled Templates

```
$ semp-workflow init --help

Copy bundled workflow templates to a local directory.

Usage:
  semp-workflow init [flags]

Flags:
  -f, --force               Overwrite existing files.
  -h, --help                help for init
  -o, --output-dir string   Directory to copy bundled templates into. (default "templates")
```

**Explanation:** The templates are embedded inside the binary (via `//go:embed`). The `init` command writes them to a local directory so you can customise them. Existing files are skipped unless `--force` is used.

**Examples:**
```bash
semp-workflow init
semp-workflow init --output-dir my-templates
semp-workflow init --output-dir templates --force
```

---

## 8. Built-in Template Reference

### app-outbound — APP Outbound Workflows

Message direction: Solace -> APP (broker receives and forwards to downstream)

| Template | Description |
|---|---|
| `app-outbound.new-seq` | Create a sequential-delivery queue set (Service Queue + Mirror Queue + DMQ + subscriptions) |
| `app-outbound.new-non-seq` | Create a concurrent-delivery queue set (same structure as new-seq, different default max-redelivery) |
| `app-outbound.delete` | Delete an outbound queue set |

**Required inputs:**

| Variable | Description | Example |
|---|---|---|
| `domain` | Business domain | `CENTRAL` |
| `system` | System name | `APPSYS` |
| `system_topic` | Topic identifier | `SITEB.ORDERS.ORDER-DETAIL` |

**What it creates:** Beyond queues and subscriptions, the `new-seq` and `new-non-seq` templates also provision the client profile, per-user ACL profile, client username, and publish topic exception. The `new-non-seq` variant differs only in `service_queue_max_redelivery` (5 vs 0). The `delete` template removes queues and per-user access control resources in reverse dependency order.

> See **[docs/template-app-outbound.md](template-app-outbound.md)** for the full parameter and action reference.

---

### app-inbound — APP Inbound Workflows

Message direction: APP -> Solace -> backend REST service

| Template | Description |
|---|---|
| `app-inbound.new-seq` | Create sequential inbound flow (queue set + RDP + REST Consumer + Queue Binding); subscription topic: `domain/system/topic` |
| `app-inbound.new-non-seq` | Create concurrent inbound flow; subscription topic: `topic_prefix/topic` |
| `app-inbound.delete` | Delete inbound resources (RDP first, then queues) |

**Required inputs:**

| Variable | Description | Example |
|---|---|---|
| `domain` | Business domain | `CENTRAL` |
| `system` | System name | `APP` |
| `system_topic` | Topic identifier | `SITEB.ORDERS.ORDER-DETAIL` |

**What it creates:** Beyond the queue set and access control resources (client profile, per-user ACL profile, client username, publish exception), the inbound templates also create a REST Delivery Point (RDP), REST Consumer (the HTTP endpoint), and a Queue Binding (connecting the queue to the RDP). This sets up the full message delivery pipeline from Solace to a backend REST API.

> See **[docs/template-app-inbound.md](template-app-inbound.md)** for the full parameter and action reference.

---

## 9. Available Modules

There are **225 actions across 86 object families**, one family per object type the SEMP v2 config API exposes. The full reference, with a parameter table for every action, is generated from the broker schemas and committed at **[docs/modules/index.md](modules/index.md)**. Run `semp-workflow list-modules` to list them from the binary you have.

An action is named `<family>.<verb>`:

| Verb | Available when | Behaviour |
|---|---|---|
| `add` | the object can be created | Creates it. Skipped if it already exists. |
| `update` | the object accepts changes after creation | Compares current state and sends only what differs. Skipped if already correct, fails if the object does not exist. |
| `delete` | the object can be removed | Deletes it. Skipped if it does not exist. |

Most families support all three. Some -- subscriptions, ACL exceptions, trusted common names -- are create-and-remove only, because SEMP has nothing to change on them once they exist. The broker settings object supports `update` alone, since it cannot be created or deleted.

### Modules used by the bundled templates

These are the ones the shipped `app-inbound` and `app-outbound` templates use, which is the shortest path to a working workflow. Everything else is in the generated reference.

| Module | Description |
|---|---|
| `msg_vpn_queue.add` | Create a queue. Skipped if it already exists. |
| `msg_vpn_queue.delete` | Delete a queue. Skipped if it does not exist. |
| `msg_vpn_queue_subscription.add` | Create a subscription. Skipped if it already exists. |
| `msg_vpn_rest_delivery_point.add` | Create an RDP. Skipped if it already exists. |
| `msg_vpn_rest_delivery_point.delete` | Delete an RDP. Skipped if it does not exist. |
| `msg_vpn_rest_delivery_point_rest_consumer.add` | Create a REST consumer. Skipped if it already exists. |
| `msg_vpn_rest_delivery_point_queue_binding.add` | Create a queue binding. Skipped if it already exists. |
| `msg_vpn_acl_profile.add` | Create an ACL profile. Skipped if it already exists. |
| `msg_vpn_acl_profile.delete` | Delete an ACL profile. Skipped if it does not exist. |
| `msg_vpn_acl_profile_publish_topic_exception.add` | Create a publish topic exception. Skipped if it already exists. |
| `msg_vpn_acl_profile_publish_topic_exception.delete` | Delete a publish topic exception. Skipped if it does not exist. |
| `msg_vpn_client_profile.add` | Create a client profile. Skipped if it already exists. |
| `msg_vpn_client_profile.update` | Update an existing client profile. Fails if it does not exist, skipped if it is already in the desired state. |
| `msg_vpn_client_username.add` | Create a client username. Skipped if it already exists. |
| `msg_vpn_client_username.delete` | Delete a client username. Skipped if it does not exist. |
| `msg_vpn_client_username.update` | Update an existing client username. Fails if it does not exist, skipped if it is already in the desired state. |

### Naming

Action and attribute names match the [Solace Terraform provider](https://registry.terraform.io/providers/SolaceProducts/solacebroker/latest/docs), so one spelling works across both tools.

Family names are the SEMP object name in snake_case: `MsgVpnQueue` becomes `msg_vpn_queue`, `MsgVpnTopicEndpoint` becomes `msg_vpn_topic_endpoint`, `DmrClusterLink` becomes `dmr_cluster_link`. That is the provider's resource name without its `solacebroker_` prefix, so `solacebroker_msg_vpn_queue` and `msg_vpn_queue.add` address the same object.

Attributes follow the same rule: the SEMP attribute `egressEnabled` is written `egress_enabled`, and `maxMsgSpoolUsage` is `max_msg_spool_usage`. The module reference in [docs/modules/index.md](modules/index.md) lists the exact spelling for every attribute of every action.

#### Upgrading from an earlier version

Workflows written against earlier versions keep working unchanged -- there is nothing to rewrite in order to upgrade:

- The ten original short family names still resolve: `queue`, `q_sub`, `rdp`, `rdp_rc`, `rdp_qb`, `client_profile`, `client_username`, `acl_profile`, `acl_publish_exception`, `acl_subscribe_exception`.
- camelCase attribute names are still accepted, so `egressEnabled` and `egress_enabled` are equivalent.

Both spellings work, but only the canonical one is listed: `list-modules` and the module reference show canonical names only, and that is what a new template should use. Do not supply both spellings of the same attribute in one `args` block -- that is rejected rather than resolved, because there would be no way to tell which value you meant.

### Choosing the message VPN

VPN-scoped actions use the `msg_vpn` from your config. To target a different one in a single action, pass `msg_vpn_name` explicitly. This matters when a workflow creates a VPN with `msg_vpn.add` and then populates it.

**Result states:**

| State | Meaning |
|---|---|
| `changed` | The broker was modified |
| `skipped` | Already in the desired state, nothing sent |
| `dryrun` | Dry-run mode: what would be done |
| `failed` | Rejected locally, or the broker refused it |

Because `update` compares before sending, applying the same config twice reports `changed` on the first run and `skipped` on the second. If a second run still reports `changed`, something is genuinely differing every time -- see Troubleshooting.

---

## 10. Common Scenario Examples

All examples below use the built-in APP templates (`app-outbound.yaml` and `app-inbound.yaml`).

### Scenario 1: Create a single APP outbound queue set

**Use case:** Set up a new outbound message flow (Solace -> APP) for a specific business topic.

```yaml
# config.yaml
semp:
  host: "https://broker.example.com:943"
  username: "admin"
  password: "admin"
  msg_vpn: "default"
  verify_ssl: false

global_vars:
  topic_prefix: "SITEA/APP/AIF"
  default_client_profile: "cp-it-user"
  default_acl_profile: "acl-it-user-{{ inputs.aem_client_username }}"

templates_dir: "templates"

workflows:
  - template: "app-outbound.new-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"
```

**What happens (step by step):**
1. Creates client profile `cp-it-user` (skipped if exists)
2. Updates `cp-it-user` to enable guaranteed send/receive
3. Creates ACL profile `acl-it-user-svc-app-client` with allow-connect, disallow-publish, disallow-subscribe (skipped if exists)
4. Creates client username `svc-app-client` linked to both profiles (skipped if exists)
5. Adds publish topic exception `SITEA/APP/AIF/SITEB.ORDERS.ORDER-CREATE` to the ACL profile (skipped if exists)
6. Creates service queue `TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE` (skipped if exists)
7. Creates mirror queue `MIRROR/TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE` (skipped if exists)
8. Creates dead-message queue `DMQ/TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE` (skipped if exists)
9. Subscribes the service queue to `SITEA/APP/AIF/SITEB.ORDERS.ORDER-CREATE` (skipped if exists)
10. Subscribes the mirror queue to the same topic (skipped if exists)

Re-running produces all `SKIPPED` results — no changes.

```bash
# Preview first
semp-workflow run -c config.yaml --dry-run

# Execute when ready
semp-workflow run -c config.yaml
```

---

### Scenario 2: Create a APP inbound flow (queue + RDP + REST delivery)

**Use case:** Set up an inbound message flow (APP -> Solace -> backend REST service) that delivers messages to an HTTP endpoint.

```yaml
workflows:
  - template: "app-inbound.new-seq"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "APP-AIF-CLIENT"
      rc_remote_host: "app-backend.internal"
      rc_remote_port: 443
      rc_tls_enabled: true
```

**What happens (step by step):**
1. Creates/updates client profile `cp-it-user` with guaranteed send/receive
2. Creates ACL profile `acl-it-user-APP-AIF-CLIENT` (skipped if exists)
3. Creates client username `APP-AIF-CLIENT` (skipped if exists)
4. Adds publish topic exception for the subscription topic (skipped if exists)
5. Creates service queue `FROM-CENTRAL-APP-SITEB.ORDERS.ORDER-CREATE`, owned by the RDP (skipped if exists)
6. Creates mirror queue and DMQ (skipped if exists)
7. Subscribes both queues to `CENTRAL/APP/SITEB.ORDERS.ORDER-CREATE` (skipped if exists)
8. Creates REST Delivery Point `RDP/FROM-CENTRAL-APP-SITEB.ORDERS.ORDER-CREATE` (skipped if exists)
9. Creates REST Consumer pointing at `app-backend.internal:443` with TLS (skipped if exists)
10. Binds the service queue to the RDP (skipped if exists)

The full pipeline: messages arrive on topic -> service queue (via subscription) -> RDP picks up via queue binding -> REST consumer delivers as HTTP POST to backend.

---

### Scenario 3: Batch-create multiple workflows

**Use case:** Set up multiple message flows at once during initial environment provisioning.

```yaml
workflows:
  - template: "app-outbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "ORDER.CREATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"

  - template: "app-outbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "ORDER.UPDATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"

  - template: "app-inbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "ORDER.CONFIRM"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "APP-AIF-CLIENT"
      rc_remote_host: "app-backend.internal"
```

**What happens:** All three workflows run in sequence. The first two create outbound queue sets (with shared `cp-it-user` profile and per-user ACL profiles); the third creates an inbound flow with RDP and REST consumer. Access control resources that already exist from previous workflows are skipped automatically.

```bash
semp-workflow run -c config.yaml
```

---

### Scenario 4: Centralise settings in global_vars

**Use case:** Many workflows share the same backend connection, naming conventions, and access control defaults. Define them once in `global_vars`.

```yaml
global_vars:
  topic_prefix: "SITEA/APP/AIF"
  default_client_profile: "cp-it-user"
  default_acl_profile: "acl-it-user-{{ inputs.aem_client_username }}"
  default_rc_remote_host: "app-backend.internal"
  default_rc_remote_port: 443
  default_rc_tls_enabled: true
  default_queue_owner: "svc-app-client"

workflows:
  - template: "app-inbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "SITEB.ORDERS.ORDER-DETAIL"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "APP-AIF-CLIENT"
      # optional overrides reference global_vars:
      #client_profile_name: "{{ global_vars.default_client_profile }}"
      #acl_profile_name: "{{ global_vars.default_acl_profile }}"
      rc_remote_host: "{{ global_vars.default_rc_remote_host }}"
      rc_remote_port: "{{ global_vars.default_rc_remote_port }}"
      rc_tls_enabled: "{{ global_vars.default_rc_tls_enabled }}"
```

**How it works:** `{{ global_vars.X }}` expressions are resolved at runtime. Values like `default_acl_profile` that contain `{{ inputs.aem_client_username }}` are resolved across two Jinja2 passes — first the global var is expanded, then the input reference is substituted. Change a setting in one place and all workflows pick it up.

---

### Scenario 5: Delete resources

**Use case:** Decommission a message flow and clean up all broker resources.

```yaml
workflows:
  # Delete an outbound flow
  - template: "app-outbound.delete"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      aem_client_username: "svc-app-client"

  # Delete an inbound flow
  - template: "app-inbound.delete"
    inputs:
      domain: "CENTRAL"
      system: "APP"
      system_topic: "SITEB.ORDERS.ORDER-DETAIL"
      aem_client_username: "APP-AIF-CLIENT"
```

**What happens:**

Outbound delete: removes service queue, mirror queue, DMQ, then the publish topic exception, client username, and ACL profile.

Inbound delete: removes the RDP first (cascading consumers and bindings), then the three queues, then access control resources.

Resources that don't exist are skipped. The shared client profile `cp-it-user` is intentionally **not** deleted since other workflows may use it.

> **Tip**: Always dry-run deletions first:
> ```bash
> semp-workflow run -c config.yaml --dry-run
> ```

---

### Scenario 6: Sequential vs non-sequential delivery

**Use case:** You need sequential delivery for one topic (messages processed in order, no redelivery) and concurrent delivery for another (messages can be redelivered up to 5 times).

```yaml
workflows:
  # Sequential: max_redelivery = 0 (forever, no retries to DMQ)
  - template: "app-outbound.new-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "CRITICAL.ORDER.CREATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"

  # Non-sequential: max_redelivery = 5 (retry 5 times, then route to DMQ)
  - template: "app-outbound.new-non-seq"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "BATCH.STATUS.UPDATE"
      service_queue_owner: "svc-app-client"
      non_service_queue_owner: "ADMIN-USER"
      aem_client_username: "svc-app-client"
```

**Difference:** Both templates create identical resources (queues, subscriptions, access control). The only difference is `service_queue_max_redelivery`: `0` for sequential (messages stay in queue forever until consumed) vs `5` for non-sequential (failed messages are moved to the DMQ after 5 retries).

---

## 11. Troubleshooting

### Issue: Template not found

```
TemplateError: Template 'app-outbound.new-seq' not found.
```

**Causes and fixes:**
- Verify `templates_dir` in `config.yaml` points to the correct path (relative to the config file)
- Confirm `app-outbound.yaml` exists in that directory
- The tool uses its embedded templates when no `templates_dir` / `--templates-dir` is set; run `semp-workflow init` to export the bundled templates for customisation

---

### Issue: Required input not provided

```
TemplateError: Required input 'domain' not provided
```

**Fix:** Add the missing required variable to the `inputs:` block of the workflow entry in `config.yaml`.

---

### Issue: Unexpected inputs

```
TemplateError: Unexpected inputs: my_typo_var
```

**Fix:** Check spelling — input variable names must exactly match those defined in the template's `optional:` block.

---

### Issue: Unknown argument

```
unknown argument: acessType (see docs/modules/msg_vpn_queue.md for the accepted attributes)
```

**Cause:** The action passed an argument that is not an attribute of that object. Arguments are checked against each object's full attribute set from the broker schema, so this is almost always a misspelling.

**Fix:** Open the family's page under [docs/modules/](modules/index.md) and check the spelling against the parameter table. Note SEMP attribute names are camelCase, and read-only attributes are not accepted as input because the broker will not accept them either.

---

### Issue: An attribute cannot be set on an update

```
'flavour' can only be set when the object is created, so it cannot appear in an update
```

**Cause:** SEMP marks some attributes as fixed after creation. Earlier versions dropped them silently, which reported success for a change that had not happened.

**Fix:** Remove the attribute from the `update` action. If the value genuinely must change, the object has to be deleted and recreated -- put that in the workflow explicitly so it is visible.

---

### Issue: Value too long

```
'queue_name' value is 203 bytes but the broker limit is 200: '...'
```

**Cause:** The broker enforces a length limit on names and topic strings. The limit is checked locally so the request is not sent.

**Fix:** Shorten the value. Limits are in **bytes**, not characters, so a name with accented or non-Latin characters reaches its limit sooner than its visible length suggests.

---

### Issue: A second run still reports `changed`

An `update` compares current state and should report `skipped` when the object already matches, so applying the same config twice should report nothing changed on the second run.

**Causes:**
- **Attributes the broker never returns.** Passwords and other write-only or opaque attributes cannot be read back for comparison, so they are re-sent every time and the action reports `changed`. This is expected.
- **A value the broker normalises.** If the broker stores a value in a different form from the one you supplied, the comparison sees a difference on every run. Check the attribute with a `GET` against the broker and supply the stored form.

**To confirm which:** run with `--dry-run`. The dry-run message names the attributes it would send.

---

### Issue: Connection error

```
SEMPError: Connection refused / SSL error
```

**Fix:**
- Verify the `semp.host` URL includes the protocol (`https://`) and correct port
- Set `verify_ssl: false` for self-signed certificate environments
- Confirm credentials are correct and have management access to the Message VPN

---

### Issue: Unresolved Jinja2 expression

```
WorkflowError: Input 'queue_name' still contains an unresolved Jinja2 expression
```

**Fix:**
- Confirm all referenced input variables exist (no typos)
- Avoid circular references (e.g. A defaults to `{{ inputs.b }}` and B defaults to `{{ inputs.a }}`)

---

### Dry-run best practice

Always run in dry-run mode before executing changes:

```bash
semp-workflow run -c config.yaml --dry-run --verbose
```
