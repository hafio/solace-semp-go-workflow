# Template: `app-outbound`

**File:** [examples/templates/app-outbound.yaml](../examples/templates/app-outbound.yaml)

**Message direction:** Solace -> APP (broker receives messages and delivers to a downstream APP system).

**Use case:** Provision the broker resources needed for a specific APP outbound topic — shared client profile, per-user ACL profile, client username, and the queue set (service + mirror + DMQ) with topic subscriptions.

---

## Template Variants

| Template reference | Purpose | Key difference |
|---|---|---|
| `app-outbound.new-seq` | Create the full outbound stack with **sequential** delivery | `service_queue_max_redelivery: 0` (retry forever, never give up) |
| `app-outbound.new-non-seq` | Create the full outbound stack with **concurrent/retry** delivery | `service_queue_max_redelivery: 5` (route to DMQ after 5 retries) |
| `app-outbound.delete` | Tear down all resources for a given outbound topic | Removes queues, topic exception, username, and per-user ACL profile |

`new-seq` and `new-non-seq` share the same action list via a YAML anchor — the only difference is the `service_queue_max_redelivery` default.

---

## Inputs

### Required

| Input | Description | Example |
|---|---|---|
| `domain` | Business domain identifier | `CENTRAL` |
| `system` | Source/downstream system name | `APPSYS` |
| `system_topic` | Business topic identifier | `SITEB.ORDERS.ORDER-CREATE` |
| `service_queue_owner` | Client username that owns the service queue | `svc-app-client` |
| `non_service_queue_owner` | Client username that owns the mirror queue and DMQ | `ADMIN-USER` |
| `aem_client_username` | Client username to create for this topic | `svc-app-client` |

### Optional (with defaults)

**Access control:**

| Input | Default | Notes |
|---|---|---|
| `client_profile_name` | `"cp-it-user"` | Shared client profile used by all APP workflows |
| `acl_profile_name` | `"acl-it-user-{{ inputs.aem_client_username }}"` | Per-user ACL profile |

**Topics and naming:**

| Input | Default |
|---|---|
| `topic_prefix` | `"SITEA/APP/AIF"` |
| `service_queue_name` | `"TO-{{ domain }}-{{ system }}-{{ system_topic }}"` |
| `service_queue_subscription` | `"{{ topic_prefix }}/{{ system_topic }}"` |
| `mirror_queue_name` | `"MIRROR/TO-{{ domain }}-{{ system }}-{{ system_topic }}"` |
| `mirror_queue_subscription` | `"{{ topic_prefix }}/{{ system_topic }}"` |
| `dmq_name` | `"DMQ/TO-{{ domain }}-{{ system }}-{{ system_topic }}"` |

**Service queue behaviour:**

| Input | Default | Notes |
|---|---|---|
| `service_queue_owner` | `""` | (Overridden by required input) |
| `service_queue_dmq` | `"DMQ/TO-..."` | Dead-message queue target |
| `service_queue_permission` | `"no-access"` | Permission for non-owner clients |
| `service_queue_access` | `exclusive` | |
| `service_queue_reject_msg_sender` | `"always"` | Notify sender when message is discarded |
| `service_queue_ttl` | `0` | 0 = no TTL |
| `service_queue_max_redelivery` | `0` (seq) / `5` (non-seq) | -1 = disabled, 0 = forever |
| `service_queue_max_spool` | `5000` | MB |
| `service_queue_enabled` | `false` | Ingress/egress start disabled for safety |

**Mirror queue behaviour** (audit/replay copy):

| Input | Default |
|---|---|
| `mirror_queue_owner` | `"{{ non_service_queue_owner }}"` |
| `mirror_queue_dmq` | `"#DMQ_LAST_VALUE"` |
| `mirror_queue_permission` | `"no-access"` |
| `mirror_queue_access` | `exclusive` |
| `mirror_queue_reject_msg_sender` | `"when-queue-enabled"` |
| `mirror_queue_ttl` | `1296000` (15 days) |
| `mirror_queue_max_spool` | `5000` |
| `mirror_queue_enabled` | `false` |

**DMQ behaviour:**

| Input | Default |
|---|---|
| `dmq_owner` | `"{{ non_service_queue_owner }}"` |
| `dmq_dmq` | `"#DEAD_MSG_QUEUE"` |
| `dmq_permission` | `"no-access"` |
| `dmq_access` | `exclusive` |
| `dmq_ttl` | `7776000` (90 days) |
| `dmq_max_spool` | `5000` |
| `dmq_enabled` | `false` |

---

## Action Sequence

### `new-seq` / `new-non-seq` (11 actions)

| # | Action name | Module | Purpose |
|---|---|---|---|
| 1 | Create Client Profile | `client_profile.add` | Create `cp-it-user`. Skipped if exists. |
| 2 | Update Client Profile | `client_profile.update` | Enable guaranteed send + receive. Runs every time to ensure correct state. |
| 3 | Create ACL Profile | `acl_profile.add` | Create `acl-it-user-{aem_client_username}` with allow-connect, disallow-publish, disallow-subscribe. Skipped if exists. |
| 4 | Create Client Username | `client_username.add` | Create `{aem_client_username}` linked to both profiles. Skipped if exists. |
| 5 | Update Client Username Association | `client_username.update` | Ensures profile links are correct even if the username pre-existed with wrong associations. |
| 6 | Add ACL Publish Topic Exception | `acl_publish_exception.add` | Add publish exception for `{topic_prefix}/{system_topic}` to the per-user ACL profile. |
| 7 | Create Service Queue | `queue.add` | Create `TO-{domain}-{system}-{system_topic}`. |
| 8 | Create Mirror Queue | `queue.add` | Create `MIRROR/TO-{domain}-{system}-{system_topic}`. |
| 9 | Create DMQ | `queue.add` | Create `DMQ/TO-{domain}-{system}-{system_topic}`. |
| 10 | Add Service Queue Subscription | `q_sub.add` | Subscribe service queue to `{topic_prefix}/{system_topic}`. |
| 11 | Add Mirror Queue Subscription | `q_sub.add` | Subscribe mirror queue to the same topic. |

All actions are idempotent. Re-running produces `SKIPPED` for every step (except the two update actions, which always run and report `OK` or `SKIPPED` if no fields changed).

### `delete` (6 actions)

| # | Action name | Module | Purpose |
|---|---|---|---|
| 1 | Delete Service Queue | `queue.delete` | Remove the service queue. |
| 2 | Delete Mirror Queue | `queue.delete` | Remove the mirror queue. |
| 3 | Delete DMQ | `queue.delete` | Remove the dead-message queue. |
| 4 | Remove ACL Publish Topic Exception | `acl_publish_exception.delete` | Remove publish exception. |
| 5 | Delete Client Username | `client_username.delete` | Remove the per-topic client username. |
| 6 | Delete ACL Profile | `acl_profile.delete` | Remove the per-user ACL profile. |

> **Note:** The shared `cp-it-user` client profile is **not** deleted — it's reused by all APP workflows.

---

## Example Workflow Entry

```yaml
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

With the defaults above, this creates:

| Resource | Name |
|---|---|
| Client profile | `cp-it-user` |
| ACL profile | `acl-it-user-svc-app-client` |
| Client username | `svc-app-client` |
| Publish topic exception | `SITEA/APP/AIF/SITEB.ORDERS.ORDER-CREATE` |
| Service queue | `TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE` |
| Mirror queue | `MIRROR/TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE` |
| DMQ | `DMQ/TO-CENTRAL-APPSYS-SITEB.ORDERS.ORDER-CREATE` |

### Dry-run then execute

```bash
semp-workflow run -c config.yaml --dry-run
semp-workflow run -c config.yaml
```

### Delete the same resources

```yaml
workflows:
  - template: "app-outbound.delete"
    inputs:
      domain: "CENTRAL"
      system: "APPSYS"
      system_topic: "SITEB.ORDERS.ORDER-CREATE"
      aem_client_username: "svc-app-client"
```
