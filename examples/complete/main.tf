terraform {
  required_providers {
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = "~> 0.0"
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
