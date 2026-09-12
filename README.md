# Terraform Provider: nxs-anomaly

Terraform provider for managing [nxs-anomaly](https://github.com/nixys/nxs-anomaly), a compact Go service for alerting and on-call notifications.

Use infrastructure as code to manage:
- users and on-call teams,
- on-call schedules with shifts and recurrence,
- escalation chains with all supported step types,
- integrations with Prometheus Alertmanager, PagerDuty, VictorOps, and Grafana Alerting,
- ChatOps channels (Telegram, Slack, Mattermost),
- schedule overrides and maintenance windows.

---

## Documentation

- [Provider reference](docs/index.md): generated schemas for every resource and data source.
- [Usage guide](docs/guides/usage.md): configuration, imports, and operational examples.
- [Complete configuration](examples/complete/main.tf).
- [Wiki](wiki/Home.md).
- [Contributing](CONTRIBUTING.md) and [release procedure](RELEASING.md).

The public Registry address is `nixys/nxs-anomaly`. Registry installation becomes
available after the first signed release is published and registered. Until then,
use a local build. The examples use the intended `0.1.x` public release series.
Resource and data-source schemas in `docs/` are the authoritative attribute reference.

## Contents

- [Requirements](#requirements)
- [Installation](#installation)
  - [Terraform Registry](#terraform-registry)
  - [Local Build](#local-build)
  - [Dev Override](#dev-override)
- [Provider Configuration](#provider-configuration)
  - [Attributes](#provider-attributes)
  - [Environment Variables](#environment-variables)
- [Resources](#resources)
  - [anomaly_user](#anomaly_user)
  - [anomaly_team](#anomaly_team)
  - [anomaly_schedule](#anomaly_schedule)
  - [anomaly_schedule_override](#anomaly_schedule_override)
  - [anomaly_escalation_chain](#anomaly_escalation_chain)
  - [anomaly_integration](#anomaly_integration)
  - [anomaly_chatops_channel](#anomaly_chatops_channel)
  - [anomaly_maintenance_window](#anomaly_maintenance_window)
- [Data Sources](#data-sources)
  - [Single-object Lookups](#single-object-lookups)
  - [Lists](#lists)
  - [Reports](#reports)
- [Importing Existing Resources](#importing-existing-resources)
- [Complete Example](#complete-on-call-example)
- [Provider Development](#provider-development)

---

## Requirements

| Component | Minimum Version |
|-----------|-------------------|
| Terraform | 1.0 |
| nxs-anomaly | A compatible API `/api/v1/` release; verify acceptance tests for the target service version |
| Go (building from source) | 1.25.8 (see `go.mod`) |

---

## Installation

### Terraform Registry

After the first public release is published, add the following `required_providers` block:

```hcl
terraform {
  required_providers {
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = "~> 0.1"
    }
  }
}
```

### Local Build

```bash
git clone https://github.com/nixys/terraform-provider-nxs-anomaly
cd terraform-provider-nxs-anomaly
make build          # build ./terraform-provider-nxs-anomaly
make install        # copy to ~/.terraform.d/plugins/...
```

### Dev Override

To develop without publishing to the Registry, add this to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "nixys/nxs-anomaly" = "/path/to/terraform-provider-nxs-anomaly"
  }
  direct {}
}
```

> **Note:** when using a development override for this provider, skip `terraform init` in a provider-only test configuration.
> Run `terraform plan` / `terraform apply` directly. Other providers and modules may still require initialization.

---

## Provider Configuration

```hcl
provider "anomaly" {
  url     = "http://nxs-anomaly.internal:8080"
  api_key = var.anomaly_api_key
}
```

All attributes are optional and fall back to environment variables when omitted.

### Provider Attributes

| Attribute | Type | Default | Description |
|---------|-----|--------------|----------|
| `url` | string | `http://localhost:8080` | Base URL of the nxs-anomaly service |
| `api_key` | string | `""` | API key (`X-API-Key`). Sensitive. |
| `tls_insecure_skip_verify` | bool | `false` | Disable TLS certificate verification |
| `request_timeout` | number | `30` | HTTP request timeout in seconds |
| `max_retries` | number | `0` | Number of retries on 502/503/504 responses |

### Environment Variables

| Variable | Attribute |
|-----------|---------|
| `NXS_ANOMALY_URL` | `url` |
| `NXS_ANOMALY_API_KEY` | `api_key` |
| `NXS_ANOMALY_TLS_INSECURE` | `tls_insecure_skip_verify` |
| `NXS_ANOMALY_REQUEST_TIMEOUT` | `request_timeout` |
| `NXS_ANOMALY_MAX_RETRIES` | `max_retries` |

Explicit attributes in the `provider` block take precedence over environment variables.

**Example: supplying a secret in CI using Vault:**

```bash
export NXS_ANOMALY_API_KEY="$(vault kv get -field=key secret/nxs-anomaly)"
terraform apply
```

---

## Resources

### anomaly_user

Manages a user in the alerting service.

#### Attributes

| Attribute | Required | Default | Description |
|---------|----------|---------|----------|
| `name` | ✓ | — | Full display name |
| `username` | | automatic | Login. Defaults to lowercase `name` with spaces replaced by `.` |
| `email` | | `""` | Email address |
| `phone` | | `""` | Phone number |
| `telegram_id` | | `""` | Telegram chat ID |
| `timezone` | | `UTC` | IANA timezone (`Europe/Moscow`, `Asia/Yekaterinburg`) |
| `on_duty` | | `false` | Whether the user is currently on duty |
| `priority` | | `medium` | Notification priority: `low`, `medium`, `high` |
| `role` | | `""` | Role: `viewer`, `responder`, `editor`, `admin` |
| `notification_targets` | | `[]` | Notification delivery channels |
| `notification_policies` | | — | Personal delivery chains for `default` and `important` notifications |
| `id` | computed | — | Resource ID |
| `created_at` | computed | — | Creation time (RFC3339) |
| `updated_at` | computed | — | Last update time (RFC3339) |

**notification_targets** is a list of objects:

| Field | Required | Description |
|------|----------|----------|
| `channel` | ✓ | `webhook`, `telegram`, `email`, `sms`, `phone` |
| `target` | ✓ | URL, Telegram chat ID, email, or phone number |

#### Example

```hcl
resource "anomaly_user" "alice" {
  name        = "Alice Smith"
  username    = "alice.smith"
  email       = "alice@example.com"
  telegram_id = "123456789"
  timezone    = "Europe/Moscow"
  priority    = "high"

  notification_targets = [
    {
      channel = "telegram"
      target  = "123456789"
    },
    {
      channel = "webhook"
      target  = "https://hooks.example.com/alice"
    }
  ]
}

output "alice_id" {
  value = anomaly_user.alice.id
}
```

---

### anomaly_team

Manages an on-call team.

#### Attributes

| Attribute | Required | Description |
|---------|----------|----------|
| `name` | ✓ | Team name |
| `member_ids` | | IDs of users in the team |
| `id` | computed | Resource ID |
| `created_at` | computed | |
| `updated_at` | computed | |

#### Example

```hcl
resource "anomaly_team" "ops" {
  name = "Ops Team"
  member_ids = [
    anomaly_user.alice.id,
    anomaly_user.bob.id,
  ]
}
```

---

### anomaly_schedule

Manages an on-call schedule.

#### Attributes

| Attribute | Required | Default | Description |
|---------|----------|---------|----------|
| `name` | ✓ | — | Schedule name |
| `timezone` | | `UTC` | IANA timezone used to interpret shifts |
| `team_id` | | — | ID of the team that owns the schedule |
| `enabled` | | `true` | Whether the schedule is enabled |
| `notify_on_shift_change` | | `false` | Notify participants when the shift changes |
| `rotation` | | — | Automatic participant rotation |
| `shifts` | | `[]` | List of shifts |
| `id` | computed | | |
| `created_at` | computed | | |
| `updated_at` | computed | | |

**shifts** is a list of objects:

| Field | Required | Default | Description |
|------|----------|---------|----------|
| `user_id` | ✓ | — | ID of the on-call user |
| `start_at` | ✓ | — | Shift start (RFC3339) |
| `end_at` | ✓ | — | Shift end (RFC3339) |
| `recurrence` | | `none` | Recurrence: `none`, `daily`, `weekly` |
| `id` | | computed | Shift ID, assigned on creation |

#### Example

```hcl
resource "anomaly_schedule" "weekdays" {
  name     = "Weekday On-Call"
  timezone = "Europe/Moscow"
  team_id  = anomaly_team.ops.id

  shifts = [
    {
      user_id    = anomaly_user.alice.id
      start_at   = "2026-06-02T09:00:00+03:00"
      end_at     = "2026-06-02T18:00:00+03:00"
      recurrence = "weekly"
    },
    {
      user_id    = anomaly_user.bob.id
      start_at   = "2026-06-03T09:00:00+03:00"
      end_at     = "2026-06-03T18:00:00+03:00"
      recurrence = "weekly"
    }
  ]
}
```

---

### anomaly_schedule_override

Manages a temporary on-call replacement within a schedule.

| Attribute | Required | Default | Description |
|---------|----------|---------|----------|
| `schedule_id` | ✓ | — | Schedule ID; changing it replaces the override |
| `user_id` | ✓ | — | ID of the replacement user |
| `start_at` | ✓ | — | Override start (RFC3339) |
| `until` | ✓ | — | Override end (RFC3339) |
| `reason` | | `""` | Reason for the override |
| `id`, `created_at`, `updated_at` | computed | — | Server-assigned fields |

```hcl
resource "anomaly_schedule_override" "vacation" {
  schedule_id = anomaly_schedule.weekdays.id
  user_id     = anomaly_user.bob.id
  start_at    = "2026-06-02T09:00:00+03:00"
  until       = "2026-06-03T09:00:00+03:00"
  reason      = "Vacation cover"
}
```

---

### anomaly_escalation_chain

Manages an escalation chain: an ordered sequence of incident-handling steps.

#### Attributes

| Attribute | Required | Description |
|---------|----------|----------|
| `name` | ✓ | Chain name |
| `steps` | | Ordered steps; an empty list is allowed for a draft |
| `id` | computed | |
| `created_at` | computed | |
| `updated_at` | computed | |

#### Steps

Every step must contain `kind`. Other fields depend on the step kind. Use the uppercase values below in Terraform configuration.

---

**WAIT** pauses before the next step:

```hcl
{
  kind          = "WAIT"
  delay_minutes = 5
}
```

---

**NOTIFY_USER** notifies specific users:

```hcl
{
  kind     = "NOTIFY_USER"
  user_ids = [anomaly_user.alice.id, anomaly_user.bob.id]
}
```

---

**NOTIFY_SCHEDULE** notifies the currently scheduled on-call user:

```hcl
{
  kind        = "NOTIFY_SCHEDULE"
  schedule_id = anomaly_schedule.weekdays.id
}
```

| Field | Default | Description |
|------|---------|----------|
| `schedule_id` | — | Schedule ID |
| `allow_uncovered` | `false` | Accept a schedule with a coverage gap in the next 7 days |

Without `allow_uncovered = true`, the API **refuses** to attach a schedule with
an uncovered period: the step would silently notify nobody during that period.
This flag explicitly acknowledges the gap. The corresponding report in
`anomaly_schedule_coverage` marks the schedule as `acknowledged = true` when all referring steps accept it.

```hcl
{
  kind            = "NOTIFY_SCHEDULE"
  schedule_id     = anomaly_schedule.weekends.id
  allow_uncovered = true   # Intentionally leave weekends uncovered
}
```

---

**NOTIFY_TEAM** notifies every member of a team:

```hcl
{
  kind    = "NOTIFY_TEAM"
  team_id = anomaly_team.ops.id
}
```

---

**NOTIFY_EMERGENCY** sends an emergency notification to a specific user:

```hcl
{
  kind    = "NOTIFY_EMERGENCY"
  user_id = anomaly_user.oncall_manager.id
}
```

---

**NOTIFY_DUTY_USERS** notifies on-duty team members, falling back to everyone if configured:

```hcl
{
  kind            = "NOTIFY_DUTY_USERS"
  team_id         = anomaly_team.ops.id
  fallback_to_all = true
}
```

---

**TRIGGER_WEBHOOK** sends a POST request to an external URL:

```hcl
{
  kind        = "TRIGGER_WEBHOOK"
  webhook_url = "https://hooks.example.com/incident"
}
```

---

**CREATE_ISSUE** creates an issue in a tracker:

```hcl
{
  kind             = "CREATE_ISSUE"
  tracker_type     = "redmine"
  url              = "https://redmine.example.com"
  token_env        = "REDMINE_API_TOKEN"   # Or token = "..." (sensitive)
  project          = "ops"
  subject_template = "[{{ .severity }}] {{ .title }}"
  body_template    = "Group: {{ .group_id }}\n\nTitle: {{ .title }}\nStatus: {{ .status }}"
}
```

Issue template variables: `title`, `severity`, `group_id`, `status`, `labels`
(inserted as a whole; individual key lookup is not supported). CamelCase aliases
such as `{{ .Severity }}` also work.

| Field | Description |
|------|----------|
| `tracker_type` | Tracker type (currently `redmine`) |
| `url` | Tracker base URL |
| `token` | API token (Sensitive) |
| `token_env` | Environment variable containing the token, as an alternative to `token` |
| `project` | Project identifier |
| `subject_template` | Go template for the issue title |
| `body_template` | Go template for the issue body |

---

**RESOLVE** automatically resolves the incident:

```hcl
{ kind = "RESOLVE" }
```

---

**REPEAT** repeats the chain from the specified position:

```hcl
{
  kind              = "REPEAT"
  from_position     = 0          # Zero-based position to return to
  max_repeat_count  = 3          # Maximum number of repetitions
  cooldown_minutes  = 60         # Minimum interval between repetitions
}
```

#### Complete Chain Example

```hcl
resource "anomaly_escalation_chain" "critical" {
  name = "Critical Incidents"

  steps = [
    {
      kind     = "NOTIFY_USER"
      user_ids = [anomaly_user.alice.id]
    },
    {
      kind          = "WAIT"
      delay_minutes = 5
    },
    {
      kind        = "NOTIFY_SCHEDULE"
      schedule_id = anomaly_schedule.weekdays.id
    },
    {
      kind          = "WAIT"
      delay_minutes = 10
    },
    {
      kind    = "NOTIFY_DUTY_USERS"
      team_id = anomaly_team.ops.id
    },
    {
      kind          = "WAIT"
      delay_minutes = 15
    },
    {
      kind        = "TRIGGER_WEBHOOK"
      webhook_url = "https://hooks.example.com/pagerduty-escalation"
    }
  ]
}
```

---

### anomaly_integration

Manages an alert ingestion endpoint (integration).

After creation, the provider stores the computed `key` in state. Use it to construct the webhook URL:

```
<url>/integrations/v1/<type>/<key>
```

#### Attributes

| Attribute | Required | Default | Description |
|---------|----------|---------|----------|
| `name` | ✓ | — | Integration name |
| `type` | | `webhook` | Type: `webhook`, `alertmanager`, `pagerduty`, `victorops`, `grafana-alerting` |
| `group_by` | | `[alertname, service]` | Alert fields used for grouping |
| `routes` | | default route | Routing rules (see below) |
| `notification_policy` | | system default | Notification batching policy |
| `templates` | | `{}` | Go templates for delivery channels |
| `pipeline` | | — | Alert parsing and enrichment pipeline (see below) |
| `webhook_secret` | | — | HMAC secret for signature verification (`X-Hub-Signature-256`). Sensitive. |
| `team_id` | | — | ID of the team that owns the integration |
| `kafka_topic` | | — | Kafka topic for event delivery |
| `heartbeat` | | disabled | Source silence detection (see below) |
| `legacy_pool` | | — | Legacy on-call pool compatibility settings |
| `provisioned_by` | computed | — | `terraform` for provider-created objects; the API prevents UI edits |
| `key` | computed | — | Integration key for the webhook URL |
| `source_type` | | = type | Original source type |
| `id` | computed | | |
| `created_at` | computed | | |
| `updated_at` | computed | | |

#### Routes

Every integration must have **exactly one** route with `is_default = true`.

| Field | Required | Default | Description |
|------|----------|---------|----------|
| `name` | ✓ | — | Route name |
| `match_type` | | `all` | Strategy: `all` (catch-all), `labels` (label matching), `regex` (regular expression) |
| `is_default` | | `false` | Default route; exactly one per integration |
| `escalation_chain_id` | | — | Escalation chain ID |
| `labels` | | `{}` | Label map for `match_type = labels` |
| `pattern` | | `""` | Regular expression for `match_type = regex` |

**Example: label routes and a catch-all route**

```hcl
routes = [
  {
    name                = "critical"
    match_type          = "labels"
    is_default          = false
    labels              = { severity = "critical", env = "prod" }
    escalation_chain_id = anomaly_escalation_chain.critical.id
  },
  {
    name                = "default"
    match_type          = "all"
    is_default          = true
    escalation_chain_id = anomaly_escalation_chain.low_priority.id
  }
]
```

#### notification_policy

```hcl
notification_policy = {
  channels               = ["webhook", "telegram"]
  batch_timeout_seconds  = 30    # Wait for alerts to accumulate before sending
  batch_deadline_seconds = 120   # Maximum batching delay
  epic_threshold_count   = 10    # At least N alerts activates epic mode
  epic_threshold_seconds = 300
  emergency_user_id      = anomaly_user.oncall_manager.id
  epic_user_id           = anomaly_user.incident_lead.id
}
```

#### heartbeat

Detects source silence: raises `SourceSilent` if the source stops sending alerts.
This check is **opt-in**. The source owner decides whether it must send something
every N seconds; many integrations can legitimately remain silent for weeks.

```hcl
heartbeat = {
  interval_seconds = 300   # Report more than five minutes of silence
  grace_seconds    = 60    # Allow for a late heartbeat
}
```

| Field | Default | Description |
|------|---------|----------|
| `interval_seconds` | `0` | Allowed silence in seconds. `0` disables the check. The API raises positive values below 60 to 60 |
| `grace_seconds` | one third of the interval | Jitter allowance |

The entire block is optional. When omitted, the provider does not send the field,
and the API retains the current setting.

#### pipeline

Parses and enriches an alert before storage. The pipeline is a list of stages, each with **one
operation** and an optional `if` condition. Pass it through `jsonencode()`.

**Runs after route selection.** Routing uses the original alert as received from
the source. The pipeline affects deduplication, the stored alert, and notification
text, but does not change which escalation chain notifies the on-call user.
Changing enrichment therefore cannot silently redirect a notification.

| Operation | Effect |
|---|---|
| `extract` | Extract named regular-expression groups from a field into labels |
| `set` | Add or overwrite labels |
| `rename` | Rename a label |
| `remove` | Remove labels |
| `gsub` | Rewrite a field using a regular expression |
| `truncate` | Limit field length |
| `drop` | Discard the entire alert |

Fields: `title`, `message`, `severity`, `label:<name>`. The `if` condition accepts exactly
one check: `equals`, `matches`, or `exists`.

```hcl
resource "nxs_anomaly_integration" "prometheus" {
  name = "prometheus-prod"

  pipeline = jsonencode([
    # Extract pod/namespace from the title when the source omits these labels
    { extract = { from = "title", pattern = "pod (?P<pod>\\S+) in (?P<namespace>\\S+)" } },
    # Remove build IDs so nightly failures group into a single incident
    { gsub = { field = "title", pattern = "\\s+run-[0-9a-f]+$", replace = "" } },
    # Discard noise without notifying anyone
    { if = { field = "label:severity", equals = "info" }, drop = true },
  ])
}
```

Limits: up to 32 stages, regular expressions up to 512 characters, no processing of
fields longer than 16 KB, and up to 32 groups per `extract`. A failed stage does not discard
the alert; it is skipped and the server logs `alert_pipeline_stage_skipped`.

Rules that fail to compile are rejected during `apply`. To inspect the pipeline
before sending a production alert, use the debug endpoint:
`POST /api/v1/routes/debug/{key}` returns the labels and title before and after processing.

An unconditional `drop` is prohibited because it would silently disable an integration
that continues to respond with `202`.

#### templates

Go templates for formatting notifications. Keys are channel names (`telegram`,
`email`, `webhook`, `sms`, `phone`). The `default` key applies to channels without
a dedicated template.

Available variables:

| Variable | Description |
|---|---|
| `title` | Alert title |
| `severity` | Alert severity |
| `reason` | Notification reason, such as escalation or resolution |
| `group_id` | Alert group ID |
| `status` | Group status; defaults to `open` |
| `user_name` | Recipient display name |
| `user_username` | Recipient username |
| `labels` | All alert labels, sorted and formatted as `k=v, k=v` |
| `label_<name>` | Individual label: `{{ .label_pod }}`, `{{ .label_namespace }}` |

```hcl
templates = {
  telegram = "🔥 *[{{ .severity }}]* {{ .title }}\n{{ .label_namespace }}/{{ .label_pod }}\nGroup: `{{ .group_id }}`"
  webhook  = "{{ .title }} | {{ .severity }} | {{ .labels }}"
}
```

Placeholders accept `{{ .severity }}`, `{{ severity }}`, or
CamelCase aliases such as `{{ .Severity }}` / `{{ .GroupID }}`. These forms produce
the same result. Standard `text/template` constructs are supported:
(`{{ if .severity }}…{{ end }}`).

Characters that are invalid in template identifiers are replaced with underscores
in `label_<name>`: use `{{ .label_kubernetes_io_name }}` for `kubernetes.io/name`.

Variables outside this table are unavailable. An unknown placeholder does not prevent
delivery; it remains unchanged in the notification text (the server logs
`notification_template_render_failed`).

**Clearing templates.** This attribute is `Optional + Computed`. Removing
`templates` from configuration preserves its previous state value, so existing
templates continue to apply. To restore the built-in format
(`[severity] title`), explicitly set an empty map:

```hcl
templates = {}
```

#### Complete Example

```hcl
resource "anomaly_integration" "prometheus" {
  name     = "Prometheus Alertmanager"
  type     = "alertmanager"
  group_by = ["alertname", "cluster", "namespace"]

  webhook_secret = var.webhook_secret

  routes = [
    {
      name                = "critical-prod"
      match_type          = "labels"
      labels              = { severity = "critical", env = "prod" }
      escalation_chain_id = anomaly_escalation_chain.critical.id
    },
    {
      name                = "default"
      match_type          = "all"
      is_default          = true
      escalation_chain_id = anomaly_escalation_chain.low_priority.id
    }
  ]

  notification_policy = {
    channels              = ["telegram", "webhook"]
    batch_timeout_seconds = 30
  }

  templates = {
    telegram = "🔥 *[{{ .severity }}]* {{ .title }}"
  }
}

# Webhook URL for Prometheus Alertmanager:
output "alertmanager_url" {
  value = "${var.anomaly_url}/integrations/v1/alertmanager/${anomaly_integration.prometheus.key}"
}
```

---

### anomaly_chatops_channel

Manages a ChatOps channel for commands and notifications.

#### Attributes

| Attribute | Required | Default | Description |
|---------|----------|---------|----------|
| `platform` | ✓ | — | Platform: `telegram`, `slack`, `mattermost` |
| `name` | ✓ | — | Channel identifier on the platform |
| `team_id` | | — | ID of the team that owns the channel |
| `user_id` | | — | User ID for a personal channel |
| `commands_enabled` | | `true` | Enable ChatOps commands (`/ack`, `/resolve`, `/oncall`) |
| `notifications_enabled` | | `true` | Send incident notifications |
| `webhook_url` | | — | Channel webhook URL. Sensitive. |
| `external_id` | | — | Channel identifier in the external system |
| `id` | computed | | |
| `created_at` | computed | | |
| `updated_at` | computed | | |

#### Example

```hcl
resource "anomaly_chatops_channel" "ops_telegram" {
  platform              = "telegram"
  name                  = "-100123456789"   # Telegram group ID
  team_id               = anomaly_team.ops.id
  commands_enabled      = true
  notifications_enabled = true
}
```

---

### anomaly_maintenance_window

Schedules a maintenance window. Alert groups opened by the listed integrations
between `starts_at` and `ends_at` are recorded and silenced without
notifying anyone.

#### Attributes

| Attribute | Required | Default | Description |
|---------|----------|---------|----------|
| `name` | ✓ | — | Window name |
| `integration_ids` | ✓ | — | Integrations covered by the window; at least one is required |
| `starts_at` | ✓ | — | Window start (RFC3339) |
| `ends_at` | ✓ | — | Window end (RFC3339); must be later than `starts_at` |
| `reason` | | `""` | Explanation displayed on silenced groups |
| `team_id` | | — | ID of the team that owns the window |
| `id` | computed | | |
| `created_at` | computed | | |
| `updated_at` | computed | | |

The API deliberately rejects empty `integration_ids` so a configuration mistake
cannot silence the entire installation.

The engine stores boundaries in UTC. The provider preserves the timestamp notation
from configuration when it represents the same instant, so changing the notation
does not appear as drift.

#### Example

```hcl
resource "anomaly_maintenance_window" "db_upgrade" {
  name            = "PostgreSQL major upgrade"
  reason          = "Scheduled upgrade; database alerts are expected"
  team_id         = anomaly_team.ops.id
  integration_ids = [anomaly_integration.prometheus.id]
  starts_at       = "2026-06-01T02:00:00Z"
  ends_at         = "2026-06-01T06:00:00Z"
}
```

---

## Data Sources

### Single-object Lookups

Single-object data sources accept `id` or `name`, except `anomaly_user`, which accepts `id` or `username`. Terraform reports an error if neither is supplied.

---

#### anomaly_user

```hcl
# By ID
data "anomaly_user" "alice" {
  id = "usr-abc123"
}

# By username
data "anomaly_user" "alice" {
  username = "alice.smith"
}

output "alice_priority" {
  value = data.anomaly_user.alice.priority
}
```

Returned fields include `id`, `name`, `username`, `email`, `phone`, `telegram_id`, `timezone`, `on_duty`, `priority`, `role`, `notification_targets`, `notification_policies`, `created_at`, and `updated_at`.

---

#### anomaly_team

```hcl
data "anomaly_team" "ops" {
  name = "Ops Team"
}

output "ops_members" {
  value = data.anomaly_team.ops.member_ids
}
```

---

#### anomaly_integration

```hcl
data "anomaly_integration" "prometheus" {
  name = "Prometheus Alertmanager"
}

output "webhook_url" {
  value = "${var.anomaly_url}/integrations/v1/alertmanager/${data.anomaly_integration.prometheus.key}"
}
```

---

#### anomaly_escalation_chain

```hcl
data "anomaly_escalation_chain" "critical" {
  name = "Critical Incidents"
}
```

---

#### anomaly_schedule

```hcl
data "anomaly_schedule" "weekdays" {
  name = "Weekday On-Call"
}

output "current_oncall_schedule_id" {
  value = data.anomaly_schedule.weekdays.id
}
```

---

#### anomaly_chatops_channel

```hcl
data "anomaly_chatops_channel" "ops" {
  name = "ops-alerts"
}
```

---

#### anomaly_maintenance_window

```hcl
data "anomaly_maintenance_window" "db_upgrade" {
  name = "DB upgrade"
}
```

---

### Lists

Return all objects in a collection with their complete nested structure.

---

#### anomaly_users

```hcl
data "anomaly_users" "all" {}

# Map username to ID for references in other configurations
output "user_ids" {
  value = { for u in data.anomaly_users.all.users : u.username => u.id }
}
```

---

#### anomaly_teams

```hcl
data "anomaly_teams" "all" {}

output "team_ids" {
  value = { for t in data.anomaly_teams.all.teams : t.name => t.id }
}
```

---

#### anomaly_integrations

Supports client-side filtering by `type`.

```hcl
# All integrations
data "anomaly_integrations" "all" {}

# Alertmanager integrations only
data "anomaly_integrations" "am" {
  type = "alertmanager"
}

# Map name to key for webhook URLs
output "integration_keys" {
  value = { for i in data.anomaly_integrations.all.integrations : i.name => i.key }
}
```

---

#### anomaly_schedules

```hcl
data "anomaly_schedules" "all" {}

output "schedule_count" {
  value = length(data.anomaly_schedules.all.schedules)
}
```

---

#### anomaly_escalation_chains

```hcl
data "anomaly_escalation_chains" "all" {}

output "chain_names" {
  value = [for c in data.anomaly_escalation_chains.all.escalation_chains : c.name]
}
```

---

#### anomaly_chatops_channels

```hcl
data "anomaly_chatops_channels" "all" {}
```

---

#### anomaly_maintenance_windows

```hcl
data "anomaly_maintenance_windows" "all" {}
```

---

### Reports

Read-only snapshots of installation state. There is no stored object behind them,
and values can change between Terraform runs. Use them after `apply` to check
that somebody is on call, schedules have coverage, and alerts can be delivered.

---

#### anomaly_on_call

Returns who is currently on call across all enabled schedules.

```hcl
data "anomaly_on_call" "now" {}

output "on_call_usernames" {
  value = [for e in data.anomaly_on_call.now.items : e.username if e.exists]
}
```

Item fields: `user_id`, `name`, `username`, `exists`, `schedule_id`,
`schedule_name`, `source` (`override` / `rotation` / `shift`).

`exists = false` means the schedule references a deleted user.
The report exposes this gap instead of hiding it.

---

#### anomaly_schedule_coverage

A coverage report. `items` contains **only** schedules with gaps,
unknown participants, or disabled schedules.

```hcl
data "anomaly_schedule_coverage" "report" {}

# Schedules with gaps that are referenced by an escalation chain.
output "coverage_gaps_that_page" {
  value = data.anomaly_schedule_coverage.report.degraded_attached
}
```

Report fields: `checked_at`, `window_days`, `schedules_total`,
`schedules_degraded`, `degraded_attached`, `items`.
Item fields: `schedule_id`, `name`, `disabled`, `gap_count`, `unknown_users`,
`attached_to` (chains), `acknowledged`, `first_gap`.

`acknowledged = true` means every referring step has set `allow_uncovered`,
explicitly accepting the gap.

---

#### anomaly_schedule_preview

Expands a schedule into a timeline for a time window, showing participants and
gaps. Use it before attaching a schedule to an escalation chain.

```hcl
data "anomaly_schedule_preview" "weekdays" {
  schedule_id = anomaly_schedule.weekdays.id
  from        = "2026-06-01T00:00:00Z"
  to          = "2026-06-08T00:00:00Z"
}

output "coverage_ratio" {
  value = data.anomaly_schedule_preview.weekdays.coverage_ratio
}
```

| Attribute | Required | Description |
|---------|----------|----------|
| `schedule_id` | ✓ | Schedule ID |
| `from` | | Window start (RFC3339); defaults to the API window |
| `to` | | Window end (RFC3339); defaults to the API window |

Computed: `timezone`, `segments`, `gaps`, `overlaps`, `warnings`,
`coverage_ratio` (0..1), `participants`, `unknown_users`.

---

#### anomaly_readiness

Reports whether this installation can notify anyone.

```hcl
data "anomaly_readiness" "check" {}

output "blockers" {
  value = [for c in data.anomaly_readiness.check.checks : c.detail if c.severity == "blocker"]
}
```

Computed: `checked_at`, `ready`, `production_ready`, `blockers`, `warnings`,
`blocker_fingerprint`, `checks`, `acknowledgement`.

A check that **could not run** is reported as a blocker, not as a success.
`production_ready` is a weaker claim: either there are no blockers,
or a named person has explicitly accepted the listed blockers.

---

## Importing Existing Resources

All 8 resources support `terraform import`. Use the nxs-anomaly object ID, except for schedule overrides, which require `<schedule_id>/<override_id>`.

```bash
# User
terraform import anomaly_user.alice usr-abc123

# Team
terraform import anomaly_team.ops team-xyz456

# Schedule
terraform import anomaly_schedule.weekdays sch-def789

# Schedule override: <schedule_id>/<override_id>
terraform import anomaly_schedule_override.vacation sch-def789/ovr-abc123

# Escalation chain
terraform import anomaly_escalation_chain.critical esc-ghi012

# Integration
terraform import anomaly_integration.prometheus int-jkl345

# ChatOps channel
terraform import anomaly_chatops_channel.ops_telegram chat-pqr901

# Maintenance window
terraform import anomaly_maintenance_window.db_upgrade mnt-stu234
```

After importing, run `terraform plan` and reconcile configuration with the imported values. Import reads remote attributes but does not write resource configuration.

---

## Complete On-call Example

This example defines an on-call setup with users, a team, a schedule, a multi-step escalation chain, a Prometheus integration, and a Telegram channel.

```hcl
terraform {
  required_providers {
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = "~> 0.1"
    }
  }
}

provider "anomaly" {
  url = var.anomaly_url
  # Credentials from environment variables:
  # NXS_ANOMALY_URL, NXS_ANOMALY_API_KEY
}

# Users

resource "anomaly_user" "alice" {
  name        = "Alice Smith"
  email       = "alice@example.com"
  telegram_id = "111111111"
  timezone    = "Europe/Moscow"
  priority    = "high"

  notification_targets = [
    { channel = "telegram", target = "111111111" },
    { channel = "email", target = "alice@example.com" }
  ]
}

resource "anomaly_user" "bob" {
  name        = "Bob Jones"
  email       = "bob@example.com"
  telegram_id = "222222222"
  timezone    = "Europe/Moscow"
  priority    = "medium"

  notification_targets = [
    { channel = "telegram", target = "222222222" }
  ]
}

# Team

resource "anomaly_team" "ops" {
  name       = "Ops Team"
  member_ids = [anomaly_user.alice.id, anomaly_user.bob.id]
}

# Schedule

resource "anomaly_schedule" "ops_weekly" {
  name     = "Ops Weekly Rotation"
  timezone = "Europe/Moscow"
  team_id  = anomaly_team.ops.id

  shifts = [
    {
      user_id    = anomaly_user.alice.id
      start_at   = "2026-06-01T09:00:00+03:00"
      end_at     = "2026-06-08T09:00:00+03:00"
      recurrence = "weekly"
    },
    {
      user_id    = anomaly_user.bob.id
      start_at   = "2026-06-08T09:00:00+03:00"
      end_at     = "2026-06-15T09:00:00+03:00"
      recurrence = "weekly"
    }
  ]
}

# Escalation chains

resource "anomaly_escalation_chain" "critical" {
  name = "Critical — Production"

  steps = [
    # 1. Immediately notify the current on-call user
    {
      kind            = "NOTIFY_SCHEDULE"
      schedule_id     = anomaly_schedule.ops_weekly.id
      allow_uncovered = true
    },
    # 2. Wait five minutes
    {
      kind          = "WAIT"
      delay_minutes = 5
    },
    # 3. Notify on-duty team members, falling back to everyone
    {
      kind            = "NOTIFY_DUTY_USERS"
      team_id         = anomaly_team.ops.id
      fallback_to_all = true
    },
    # 4. Wait another ten minutes
    {
      kind          = "WAIT"
      delay_minutes = 10
    },
    # 5. Send a webhook to an external system
    {
      kind        = "TRIGGER_WEBHOOK"
      webhook_url = "https://hooks.example.com/pagerduty-fallback"
    }
  ]
}

resource "anomaly_escalation_chain" "warning" {
  name = "Warning — Non-critical"

  steps = [
    {
      kind            = "NOTIFY_SCHEDULE"
      schedule_id     = anomaly_schedule.ops_weekly.id
      allow_uncovered = true
    },
    {
      kind          = "WAIT"
      delay_minutes = 30
    },
    {
      kind    = "NOTIFY_TEAM"
      team_id = anomaly_team.ops.id
    }
  ]
}

# Prometheus integration

resource "anomaly_integration" "prometheus" {
  name           = "Prometheus Alertmanager"
  type           = "alertmanager"
  group_by       = ["alertname", "cluster", "namespace"]
  webhook_secret = var.webhook_secret

  routes = [
    {
      name                = "critical-prod"
      match_type          = "labels"
      labels              = { severity = "critical", env = "prod" }
      escalation_chain_id = anomaly_escalation_chain.critical.id
    },
    {
      name                = "warning-prod"
      match_type          = "labels"
      labels              = { severity = "warning", env = "prod" }
      escalation_chain_id = anomaly_escalation_chain.warning.id
    },
    {
      name                = "default"
      match_type          = "all"
      is_default          = true
      escalation_chain_id = anomaly_escalation_chain.warning.id
    }
  ]

  notification_policy = {
    channels              = ["telegram"]
    batch_timeout_seconds = 60
  }

  templates = {
    telegram = "🚨 *[{{ .severity }}]* {{ .title }}\nGroup: `{{ .group_id }}`"
  }
}

# ── ChatOps ───────────────────────────────────────────────────────────────────

resource "anomaly_chatops_channel" "ops_alerts" {
  platform              = "telegram"
  name                  = "-100987654321"
  team_id               = anomaly_team.ops.id
  commands_enabled      = true
  notifications_enabled = true
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "prometheus_webhook_url" {
  description = "URL for the Alertmanager webhook receiver"
  value       = "${var.anomaly_url}/integrations/v1/alertmanager/${anomaly_integration.prometheus.key}"
}

output "oncall_schedule_id" {
  value = anomaly_schedule.ops_weekly.id
}

# ── Variables ─────────────────────────────────────────────────────────────────

variable "anomaly_url" {
  type    = string
  default = "http://localhost:8080"
}

variable "webhook_secret" {
  type      = string
  sensitive = true
  default   = ""
}
```

---

## Provider Development

### Repository Layout

```
main.go                           — entry point, providerserver.Serve
internal/provider/
  provider.go                     — NxsAnomalyProvider, Configure, registration
  client.go                       — HTTP client (get/post/put/delete/listAll, retry)
  validators.go                   — allowed-value constants
  resource_<name>.go              — one file per resource
  datasource_<name>.go            — one file per data source
  datasource_lookup.go            — lookupByName helper
examples/
  provider/provider.tf
  resources/<name>/resource.tf
  data-sources/<name>/data-source.tf
GNUmakefile                       — build/test/install
.goreleaser.yml                   — multi-arch release
.github/workflows/ci.yml          — GitHub Actions CI
```

### Commands

```bash
make build      # Build the binary
make vet        # go vet ./...
make test       # Unit tests without external services
make install    # Install into ~/.terraform.d/plugins/

# Acceptance tests against a running nxs-anomaly instance:
NXS_ANOMALY_URL=http://localhost:8080 \
NXS_ANOMALY_API_KEY=secret \
make testacc
```

### Adding a Resource

1. Create `internal/provider/resource_<name>.go`.
2. Define the model (`struct` with tfsdk tags), `var _ resource.Resource`, and `var _ resource.ResourceWithImportState`.
3. Implement `Metadata`, `Schema`, `Configure`, `Create`, `Read`, `Update`, `Delete`, and `ImportState`.
4. On a 404 in `Read`, call `resp.State.RemoveResource(ctx)` and return without an error.
5. Register the resource in `provider.go → Resources()`.
6. Add an example in `examples/resources/<name>/resource.tf`, an `import.sh`, tests, and regenerate documentation with `make docs`.

### Test Environment Variables

| Variable | Purpose |
|-----------|-----------|
| `NXS_ANOMALY_URL` | nxs-anomaly instance URL for acceptance tests |
| `NXS_ANOMALY_API_KEY` | API key for acceptance tests |

When `NXS_ANOMALY_URL` is absent, all acceptance tests (`TestAcc*`) are skipped automatically. Set `TF_ACC=1` to enable framework acceptance tests.
