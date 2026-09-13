# nxs-anomaly Terraform Module

A Terraform module for declaratively describing on-call infrastructure in
[nxs-anomaly](https://github.com/nixys/nxs-anomaly) via the
[`nixys/nxs-anomaly` Terraform provider](../../README.md).

The module wraps all 8 provider resources and links them together by local keys
(`user_key`, `schedule_key`, `escalation_chain_key`), so you do not have to pass
computed IDs around by hand:

| What it describes | Provider resource |
|-------------------|-------------------|
| On-call users | `anomaly_user` |
| Team | `anomaly_team` |
| Shift schedules | `anomaly_schedule` |
| Temporary on-call overrides | `anomaly_schedule_override` |
| Escalation chains | `anomaly_escalation_chain` |
| Alert ingestion integrations | `anomaly_integration` |
| ChatOps channels | `anomaly_chatops_channel` |
| Maintenance windows | `anomaly_maintenance_window` |

---

## Requirements

| Component | Version |
|-----------|---------|
| Terraform | >= 1.3 (requires `optional()` with default values) |
| `nixys/nxs-anomaly` provider | >= 0.0.11 |

> **Provider 0.0.11 or newer is required** — it introduced `integration.pipeline`,
> which the module passes through. On 0.0.10 a configuration with `pipeline` will
> fail `terraform validate`.
>
> **0.0.10 is a breaking provider release.** It removed `anomaly_grafana_plugin`
> (the `/api/v1/grafana-plugins` route no longer exists in the service) and added
> `anomaly_maintenance_window`, `integration.heartbeat`, and `allow_uncovered` on
> the NOTIFY_SCHEDULE step. The module requires these features and will not work
> on 0.0.9 or older. If `anomaly_grafana_plugin` resources remain in state, remove
> them (`terraform state rm`) before upgrading.

> **Provider name.** The provider's local name is `anomaly`; resources are named
> `anomaly_user`, `anomaly_team`, and so on. In the Terraform Registry the provider
> is published as `nixys/nxs-anomaly`; the address in the root project must match
> the module's `versions.tf`.

---

## Usage

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
  url     = "https://anomaly.example.com"
  api_key = var.anomaly_api_key
  # url / api_key are also read from NXS_ANOMALY_URL / NXS_ANOMALY_API_KEY.
}

module "oncall" {
  # Path to your copy of this module.
  source = "./modules/nxs-anomaly"

  users = {
    alice = {
      name        = "Alice Smith"
      email       = "alice@example.com"
      telegram_id = "111111111"
      priority    = "high"
      notification_targets = [
        { channel = "telegram", target = "111111111" },
      ]
    }
  }

  team_name = "Ops Team"

  schedules = {
    ops_weekly = {
      name     = "Ops Weekly Rotation"
      timezone = "Europe/Moscow"
      shifts = [
        {
          user_key = "alice"
          start_at = "2026-06-01T09:00:00+03:00"
          end_at   = "2026-06-08T09:00:00+03:00"
        },
      ]
    }
  }

  escalation_chains = {
    critical = {
      name = "Critical — Production"
      steps = [
        { kind = "NOTIFY_SCHEDULE", schedule_key = "ops_weekly" },
        { kind = "WAIT", delay_minutes = 5 },
        { kind = "NOTIFY_DUTY_USERS", team = true },
      ]
    }
  }

  integrations = {
    prometheus = {
      name = "Prometheus Alertmanager"
      type = "alertmanager"
      routes = [
        {
          name                 = "default"
          match_type           = "all"
          is_default           = true
          escalation_chain_key = "critical"
        },
      ]
    }
  }
}

output "alertmanager_webhook" {
  value = "https://anomaly.example.com/integrations/v1/alertmanager/${module.oncall.integration_keys["prometheus"]}"
}
```

A complete working example is in [`examples/basic`](examples/basic/main.tf).

---

## Linking Resources by Keys

The module substitutes computed IDs for local keys:

| Where | Key field | Resolves to |
|-------|-----------|-------------|
| `schedules[*].shifts[*]` | `user_key` | user ID |
| `schedules[*].rotation` | `participant_keys` | `participant_ids` |
| `schedule_overrides[*]` | `schedule_key`, `user_key` | `schedule_id`, `user_id` |
| `escalation_chains[*].steps[*]` | `user_keys` | `user_ids` (NOTIFY_USER) |
| | `user_key` | `user_id` (NOTIFY_EMERGENCY) |
| | `schedule_key` | `schedule_id` (NOTIFY_SCHEDULE) |
| | `team = true` | `team_id` (NOTIFY_TEAM / NOTIFY_DUTY_USERS) |
| `integrations[*].routes[*]` | `escalation_chain_key` | `escalation_chain_id` |
| `maintenance_windows[*]` | `integration_keys` | `integration_ids` |
| | `team = true` | `team_id` |
| `integrations[*].notification_policy` | `emergency_user_key`, `epic_user_key` | the matching `*_user_id` |
| `chatops_channels[*]` | `team = true` / `user_key` | `team_id` / `user_id` |

To reference objects **outside** this module, every key field has a paired
attribute that takes a direct ID (`user_ids`, `schedule_id`, `team_id`,
`escalation_chain_id`, `*_user_id`).

---

## Maintenance Windows, Heartbeat, and Schedule Coverage

```hcl
integrations = {
  prometheus = {
    name = "Prometheus Alertmanager"
    type = "alertmanager"
    routes = [{ name = "default", match_type = "all", is_default = true, escalation_chain_key = "critical" }]

    # The source must report every 5 minutes; longer silence is news.
    heartbeat = {
      interval_seconds = 300   # 0 or no block disables the check
      grace_seconds    = 60    # defaults to a third of the interval
    }
  }
}

maintenance_windows = {
  db_upgrade = {
    name             = "PostgreSQL major upgrade"
    reason           = "Planned upgrade, database alerts are expected"
    team             = true
    integration_keys = ["prometheus"]
    starts_at        = "2026-06-20T02:00:00+03:00"
    ends_at          = "2026-06-20T06:00:00+03:00"
  }
}

escalation_chains = {
  weekend = {
    name = "Weekend"
    steps = [
      # Without the flag the API refuses: the schedule has a gap in the next
      # 7 days, and during that gap the step silently pages nobody.
      { kind = "NOTIFY_SCHEDULE", schedule_key = "ops_weekly", allow_uncovered = true },
    ]
  }
}
```

The module rejects a maintenance window with an empty integration list at `plan`
time: an empty list would mean a single typo silences the whole installation.

The engine stores window boundaries in UTC, but the provider keeps the value you
wrote in state as long as it denotes the same instant, so a change of notation
does not show up as drift.

The module intentionally does not read the provider's reporting data sources
(`anomaly_on_call`, `anomaly_schedule_coverage`, `anomaly_schedule_preview`,
`anomaly_readiness`): their values change on their own between runs. Call them
in the root configuration if you need a post-`apply` check.

---

## Input Variables

| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| `users` | `map(object)` | `{}` | Users; map key = `user_key`. |
| `team_name` | `string` | `null` | Team name. `null` — no team is created. |
| `team_member_keys` | `list(string)` | `null` | User keys in the team. `null` — all module users. |
| `team_extra_member_ids` | `list(string)` | `[]` | IDs of external users to add to the team. |
| `schedules` | `map(object)` | `{}` | Schedules; map key = `schedule_key`. |
| `schedule_overrides` | `map(object)` | `{}` | Temporary on-call overrides. |
| `escalation_chains` | `map(object)` | `{}` | Escalation chains; map key = `escalation_chain_key`. |
| `integrations` | `map(object)` | `{}` | Alert ingestion integrations. |
| `chatops_channels` | `map(object)` | `{}` | ChatOps channels. |
| `maintenance_windows` | `map(object)` | `{}` | Maintenance windows; silence alerts from the listed integrations during work. |

The exact shape of nested objects and allowed field values are in
[`variables.tf`](variables.tf) and the
[provider README](../../README.md).

---

## Outputs

| Output | Description |
|--------|-------------|
| `user_ids` | Map of `user_key => ID`. |
| `users` | Full objects of the created users. |
| `team_id` | Team ID (`null` if no team was created). |
| `schedule_ids` | Map of `schedule_key => ID`. |
| `schedule_override_ids` | Map of override key => ID. |
| `escalation_chain_ids` | Map of `escalation_chain_key => ID`. |
| `integration_ids` | Map of key => integration ID. |
| `integration_keys` | Map of key => integration `key` (for the webhook URL). |
| `chatops_channel_ids` | Map of key => channel ID. |
| `maintenance_window_ids` | Map of key => maintenance window ID. |

The webhook URL is `<provider url>/integrations/v1/<type>/<key>`. The base URL
comes from the provider configuration, so the URL is assembled in the calling
configuration (see the example above).

---

## Module Layout

```
versions.tf     — required_version and required_providers
variables.tf    — input variables
main.tf         — locals (key resolution) + resources
outputs.tf      — outputs
examples/basic  — complete working example
```
