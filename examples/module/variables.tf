###############################################################################
# Users
###############################################################################

variable "users" {
  description = <<-EOT
    Alerting system users. The map key is a local handle (`user_key`) used to
    reference the user from shifts, escalation steps, chatops, and so on. The
    value holds the `anomaly_user` resource attributes.
  EOT
  type = map(object({
    name        = string
    username    = optional(string)
    email       = optional(string)
    phone       = optional(string)
    telegram_id = optional(string)
    timezone    = optional(string)
    on_duty     = optional(bool)
    priority    = optional(string)
    role        = optional(string)
    notification_targets = optional(list(object({
      channel = string
      target  = string
    })), [])
    notification_policies = optional(object({
      default = optional(list(object({
        channel      = string
        target       = optional(string, "")
        wait_minutes = optional(number, 0)
      })), [])
      important = optional(list(object({
        channel      = string
        target       = optional(string, "")
        wait_minutes = optional(number, 0)
      })), [])
    }))
  }))
  default = {}
}

###############################################################################
# Team
###############################################################################

variable "team_name" {
  description = "On-call team name. When null, no team is created."
  type        = string
  default     = null
}

variable "team_member_keys" {
  description = <<-EOT
    Keys from `var.users` to include in the team. When null, all module users
    are included.
  EOT
  type        = list(string)
  default     = null
}

variable "team_extra_member_ids" {
  description = "Additional IDs of users not managed by the module to add to the team."
  type        = list(string)
  default     = []
}

###############################################################################
# Schedules
###############################################################################

variable "schedules" {
  description = <<-EOT
    On-call schedules. The map key is a local handle (`schedule_key`) referenced
    from escalation steps. In shifts, the user is set via `user_key` (a key from
    `var.users`).
  EOT
  type = map(object({
    name                   = string
    timezone               = optional(string)
    enabled                = optional(bool, true)
    notify_on_shift_change = optional(bool, false)
    rotation = optional(object({
      enabled          = optional(bool, true)
      start_at         = string
      handoff_interval = optional(number, 1)
      handoff_unit     = optional(string, "weeks")
      participant_keys = optional(list(string))
      participant_ids  = optional(list(string))
      restriction = optional(object({
        start = string
        end   = string
        days  = optional(list(string), [])
      }))
    }))
    shifts = optional(list(object({
      user_key   = string
      start_at   = string
      end_at     = string
      recurrence = optional(string)
    })), [])
  }))
  default = {}
}

variable "schedule_overrides" {
  description = "Temporary on-call overrides; schedule_key and user_key resolve to module resource IDs."
  type = map(object({
    schedule_key = optional(string)
    schedule_id  = optional(string)
    user_key     = optional(string)
    user_id      = optional(string)
    start_at     = string
    until        = string
    reason       = optional(string, "")
  }))
  default = {}
}

###############################################################################
# Escalation chains
###############################################################################

variable "escalation_chains" {
  description = <<-EOT
    Escalation chains. The map key is a local handle (`escalation_chain_key`)
    referenced from integration routes. Every step must set `kind`; the other
    fields depend on the step kind (see README). References to module objects:
      - `user_keys`    -> user_ids   (NOTIFY_USER)
      - `user_key`     -> user_id    (NOTIFY_EMERGENCY)
      - `schedule_key` -> schedule_id(NOTIFY_SCHEDULE)
      - `team = true`  -> team_id    (NOTIFY_TEAM / NOTIFY_DUTY_USERS)
  EOT
  type = map(object({
    name = string
    steps = optional(list(object({
      kind = string

      # WAIT
      delay_minutes = optional(number)

      # NOTIFY_USER (module user references or external IDs)
      user_keys = optional(list(string))
      user_ids  = optional(list(string))

      # NOTIFY_EMERGENCY
      user_key = optional(string)
      user_id  = optional(string)

      # NOTIFY_SCHEDULE
      schedule_key = optional(string)
      schedule_id  = optional(string)
      # Accept a schedule with a coverage gap in the next 7 days. Without it the
      # API refuses to attach such a schedule: during the gap the step pages
      # nobody, and does so silently.
      allow_uncovered = optional(bool)

      # NOTIFY_TEAM / NOTIFY_DUTY_USERS
      team            = optional(bool, false)
      team_id         = optional(string)
      fallback_to_all = optional(bool)

      # TRIGGER_WEBHOOK
      webhook_url = optional(string)

      # CREATE_ISSUE
      tracker_type     = optional(string)
      url              = optional(string)
      token            = optional(string)
      token_env        = optional(string)
      project          = optional(string)
      subject_template = optional(string)
      body_template    = optional(string)

      # REPEAT
      from_position    = optional(number)
      max_repeat_count = optional(number)
      cooldown_minutes = optional(number)
    })), [])
  }))
  default = {}
}

###############################################################################
# Integrations
###############################################################################

variable "integrations" {
  description = <<-EOT
    Alert ingestion endpoints. In routes, the escalation chain is set via
    `escalation_chain_key` (a key from `var.escalation_chains`) or directly via
    `escalation_chain_id`. Exactly one route must have `is_default = true`.

    `templates` — notification text per channel (`telegram`, `email`, `webhook`,
    `sms`, `phone`; the `default` key covers the rest). Variables: `title`,
    `severity`, `reason`, `group_id`, `status`, `user_name`, `user_username`,
    `labels` (all labels as one string), and `label_<name>` for each label —
    for example `"*[{{ .severity }}]* {{ .title }} ({{ .label_namespace }})"`.
    An empty map (the default) clears the templates and restores the built-in
    format.

    `pipeline` — alert parsing and enrichment, a list of stages via `jsonencode()`.
    It runs AFTER route selection: it affects deduplication, the stored alert,
    and the notification text, but not which escalation chain pages the on-call
    engineer. Actions: `extract`, `set`, `rename`, `remove`, `gsub`, `truncate`,
    `drop`; each stage has exactly one action and an optional `if` condition.
  EOT
  type = map(object({
    name           = string
    type           = optional(string)
    source_type    = optional(string)
    group_by       = optional(list(string))
    webhook_secret = optional(string)
    team           = optional(bool, false)
    team_id        = optional(string)
    kafka_topic    = optional(string)
    templates      = optional(map(string), {})
    pipeline       = optional(string)

    # Source silence detection: raise SourceSilent when the source stops sending
    # alerts. The check is opt-in — without the block the field is not sent, and
    # the API keeps the current setting as is.
    heartbeat = optional(object({
      interval_seconds = optional(number, 0)
      # Left null, the API picks a third of the interval.
      grace_seconds = optional(number)
    }))

    routes = list(object({
      name                 = string
      match_type           = optional(string)
      is_default           = optional(bool)
      labels               = optional(map(string))
      pattern              = optional(string)
      escalation_chain_key = optional(string)
      escalation_chain_id  = optional(string)
    }))

    notification_policy = optional(object({
      channels               = optional(list(string))
      batch_timeout_seconds  = optional(number)
      batch_deadline_seconds = optional(number)
      epic_threshold_count   = optional(number)
      epic_threshold_seconds = optional(number)
      emergency_user_key     = optional(string)
      emergency_user_id      = optional(string)
      epic_user_key          = optional(string)
      epic_user_id           = optional(string)
    }))

    legacy_pool = optional(object({
      name               = optional(string, "")
      description        = optional(string, "")
      emergency_user_key = optional(string)
      emergency_user_id  = optional(string)
      duty_user_key      = optional(string)
      duty_user_id       = optional(string)
      emails             = optional(list(string), [])
    }))
  }))
  default = {}
}

###############################################################################
# ChatOps channels
###############################################################################

variable "chatops_channels" {
  description = <<-EOT
    ChatOps channels. The owner can be set via `team = true` (the module team),
    an explicit `team_id`, or `user_key` / `user_id`.
  EOT
  type = map(object({
    platform              = string
    name                  = string
    team                  = optional(bool, false)
    team_id               = optional(string)
    user_key              = optional(string)
    user_id               = optional(string)
    commands_enabled      = optional(bool)
    notifications_enabled = optional(bool)
    webhook_url           = optional(string)
    external_id           = optional(string)
  }))
  default = {}
}

###############################################################################
# Maintenance windows
###############################################################################

variable "maintenance_windows" {
  description = <<-EOT
    Maintenance windows. Alert groups opened by the listed integrations between
    `starts_at` and `ends_at` are recorded and silenced instead of paging anyone.
    Integrations are set via `integration_keys` (keys from `var.integrations`) or
    directly via `integration_ids`.
  EOT
  type = map(object({
    name             = string
    reason           = optional(string)
    team             = optional(bool, false)
    team_id          = optional(string)
    integration_keys = optional(list(string))
    integration_ids  = optional(list(string))
    starts_at        = string
    ends_at          = string
  }))
  default = {}

  validation {
    # The API rejects an empty integration list on purpose: otherwise a single
    # typo would silence the whole installation. Catch it at plan, not apply.
    # The condition mirrors the choice in main.tf: set keys win over direct IDs,
    # so an empty `integration_keys` is an empty list too.
    condition = alltrue([
      for w in values(var.maintenance_windows) :
      length(w.integration_keys != null ? w.integration_keys : coalesce(w.integration_ids, [])) > 0
    ])
    error_message = "Every maintenance window must name at least one integration: integration_keys or integration_ids."
  }
}
