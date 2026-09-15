terraform {
  required_version = ">= 1.3"

  required_providers {
    anomaly = {
      source  = "nixys/nxs-anomaly"
      version = "~> 0.0"
    }
  }
}

provider "anomaly" {
  # Values are also read from NXS_ANOMALY_URL / NXS_ANOMALY_API_KEY.
  url     = var.anomaly_url
  api_key = var.anomaly_api_key
}

variable "anomaly_url" {
  type    = string
  default = "http://localhost:8080"
}

variable "anomaly_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "webhook_secret" {
  type      = string
  sensitive = true
  default   = ""
}

module "oncall" {
  source = "../../"

  # ── Users ───────────────────────────────────────────────────────────────────
  users = {
    alice = {
      name        = "Alice Smith"
      email       = "alice@example.com"
      telegram_id = "111111111"
      timezone    = "Europe/Moscow"
      priority    = "high"
      role        = "responder"
      notification_targets = [
        { channel = "telegram", target = "111111111" },
        { channel = "email", target = "alice@example.com" },
      ]
    }
    bob = {
      name        = "Bob Jones"
      email       = "bob@example.com"
      telegram_id = "222222222"
      timezone    = "Europe/Moscow"
      notification_targets = [
        { channel = "telegram", target = "222222222" },
      ]
    }
  }

  # ── Team (all module users) ─────────────────────────────────────────────────
  team_name = "Ops Team"

  # ── Schedule ────────────────────────────────────────────────────────────────
  schedules = {
    ops_weekly = {
      name                   = "Ops Weekly Rotation"
      timezone               = "Europe/Moscow"
      notify_on_shift_change = true
      rotation = {
        start_at         = "2026-06-01T09:00:00+03:00"
        handoff_interval = 1
        handoff_unit     = "weeks"
        participant_keys = ["alice", "bob"]
      }
      shifts = [
        {
          user_key   = "alice"
          start_at   = "2026-06-01T09:00:00+03:00"
          end_at     = "2026-06-08T09:00:00+03:00"
          recurrence = "weekly"
        },
        {
          user_key   = "bob"
          start_at   = "2026-06-08T09:00:00+03:00"
          end_at     = "2026-06-15T09:00:00+03:00"
          recurrence = "weekly"
        },
      ]
    }
  }

  schedule_overrides = {
    alice_vacation = {
      schedule_key = "ops_weekly"
      user_key     = "bob"
      start_at     = "2026-06-03T09:00:00+03:00"
      until        = "2026-06-04T09:00:00+03:00"
      reason       = "Vacation cover"
    }
  }

  # ── Escalation chains ───────────────────────────────────────────────────────
  escalation_chains = {
    critical = {
      name = "Critical — Production"
      steps = [
        { kind = "NOTIFY_SCHEDULE", schedule_key = "ops_weekly" },
        { kind = "WAIT", delay_minutes = 5 },
        { kind = "NOTIFY_DUTY_USERS", team = true },
        { kind = "WAIT", delay_minutes = 10 },
        { kind = "TRIGGER_WEBHOOK", webhook_url = "https://hooks.example.com/pagerduty-fallback" },
      ]
    }
    warning = {
      name = "Warning — Non-critical"
      steps = [
        # Weekend on-call is not covered on purpose: without
        # allow_uncovered the API refuses to attach such a schedule.
        { kind = "NOTIFY_SCHEDULE", schedule_key = "ops_weekly", allow_uncovered = true },
        { kind = "WAIT", delay_minutes = 30 },
        { kind = "NOTIFY_TEAM", team = true },
      ]
    }
  }

  # ── Prometheus integration ──────────────────────────────────────────────────
  integrations = {
    prometheus = {
      name           = "Prometheus Alertmanager"
      type           = "alertmanager"
      group_by       = ["alertname", "cluster", "namespace"]
      webhook_secret = var.webhook_secret

      routes = [
        {
          name                 = "critical-prod"
          match_type           = "labels"
          labels               = { severity = "critical", env = "prod" }
          escalation_chain_key = "critical"
        },
        {
          name                 = "default"
          match_type           = "all"
          is_default           = true
          escalation_chain_key = "warning"
        },
      ]

      notification_policy = {
        channels              = ["telegram"]
        batch_timeout_seconds = 60
      }

      # Alertmanager must report at least every 5 minutes; longer silence
      # is news, not health.
      heartbeat = {
        interval_seconds = 300
      }

      templates = {
        telegram = "🚨 *[{{ .severity }}]* {{ .title }}"
      }
    }
  }

  # ── ChatOps ─────────────────────────────────────────────────────────────────
  chatops_channels = {
    ops_alerts = {
      platform = "telegram"
      name     = "-100987654321"
      team     = true
    }
  }

  # ── Maintenance window ──────────────────────────────────────────────────────
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
}

output "prometheus_webhook_url" {
  description = "URL for configuring the Alertmanager webhook receiver."
  value       = "${var.anomaly_url}/integrations/v1/alertmanager/${module.oncall.integration_keys["prometheus"]}"
}

output "team_id" {
  value = module.oncall.team_id
}
