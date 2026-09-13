###############################################################################
# Locals — resolve local keys (user_key / schedule_key / chain_key)
# into the real IDs of the created resources.
###############################################################################

locals {
  # user_key -> user id
  user_ids = { for k, u in anomaly_user.this : k => u.id }

  # Module team ID (null when team_name is not set)
  team_id = one(anomaly_team.this[*].id)

  # schedule_key -> schedule id
  schedule_ids = { for k, s in anomaly_schedule.this : k => s.id }

  # escalation_chain_key -> chain id
  escalation_chain_ids = { for k, e in anomaly_escalation_chain.this : k => e.id }

  # integration_key -> integration id
  integration_ids = { for k, i in anomaly_integration.this : k => i.id }

  # Final list of team members.
  team_member_ids = concat(
    var.team_member_keys != null
    ? [for k in var.team_member_keys : anomaly_user.this[k].id]
    : [for k, u in anomaly_user.this : u.id],
    var.team_extra_member_ids,
  )

  # Resolved escalation chain steps: keys -> IDs, one uniform object type.
  escalation_chains_resolved = {
    for ck, chain in var.escalation_chains : ck => {
      name = chain.name
      steps = [
        for s in chain.steps : {
          kind             = s.kind
          delay_minutes    = s.delay_minutes
          user_ids         = s.user_keys != null ? [for uk in s.user_keys : local.user_ids[uk]] : s.user_ids
          user_id          = s.user_key != null ? local.user_ids[s.user_key] : s.user_id
          schedule_id      = s.schedule_key != null ? local.schedule_ids[s.schedule_key] : s.schedule_id
          allow_uncovered  = s.allow_uncovered
          team_id          = s.team ? local.team_id : s.team_id
          fallback_to_all  = s.fallback_to_all
          webhook_url      = s.webhook_url
          tracker_type     = s.tracker_type
          url              = s.url
          token            = s.token
          token_env        = s.token_env
          project          = s.project
          subject_template = s.subject_template
          body_template    = s.body_template
          from_position    = s.from_position
          max_repeat_count = s.max_repeat_count
          cooldown_minutes = s.cooldown_minutes
        }
      ]
    }
  }
}

###############################################################################
# Users
###############################################################################

resource "anomaly_user" "this" {
  for_each = var.users

  name                  = each.value.name
  username              = each.value.username
  email                 = each.value.email
  phone                 = each.value.phone
  telegram_id           = each.value.telegram_id
  timezone              = each.value.timezone
  on_duty               = each.value.on_duty
  priority              = each.value.priority
  notification_targets  = each.value.notification_targets
  role                  = each.value.role
  notification_policies = each.value.notification_policies
}

###############################################################################
# Team
###############################################################################

resource "anomaly_team" "this" {
  count = var.team_name == null ? 0 : 1

  name       = var.team_name
  member_ids = local.team_member_ids
}

###############################################################################
# Schedules
###############################################################################

resource "anomaly_schedule" "this" {
  for_each = var.schedules

  name                   = each.value.name
  timezone               = each.value.timezone
  team_id                = local.team_id
  enabled                = each.value.enabled
  notify_on_shift_change = each.value.notify_on_shift_change

  rotation = each.value.rotation == null ? null : {
    enabled          = each.value.rotation.enabled
    start_at         = each.value.rotation.start_at
    handoff_interval = each.value.rotation.handoff_interval
    handoff_unit     = each.value.rotation.handoff_unit
    participant_ids = each.value.rotation.participant_keys != null ? [
      for key in each.value.rotation.participant_keys : local.user_ids[key]
    ] : each.value.rotation.participant_ids
    restriction = each.value.rotation.restriction == null ? null : {
      start = each.value.rotation.restriction.start
      end   = each.value.rotation.restriction.end
      days  = each.value.rotation.restriction.days
    }
  }

  shifts = [
    for s in each.value.shifts : {
      user_id    = local.user_ids[s.user_key]
      start_at   = s.start_at
      end_at     = s.end_at
      recurrence = s.recurrence
    }
  ]
}

resource "anomaly_schedule_override" "this" {
  for_each = var.schedule_overrides

  schedule_id = each.value.schedule_key != null ? local.schedule_ids[each.value.schedule_key] : each.value.schedule_id
  user_id     = each.value.user_key != null ? local.user_ids[each.value.user_key] : each.value.user_id
  start_at    = each.value.start_at
  until       = each.value.until
  reason      = each.value.reason
}

###############################################################################
# Escalation chains
###############################################################################

resource "anomaly_escalation_chain" "this" {
  for_each = local.escalation_chains_resolved

  name  = each.value.name
  steps = each.value.steps
}

###############################################################################
# Integrations
###############################################################################

resource "anomaly_integration" "this" {
  for_each = var.integrations

  name           = each.value.name
  type           = each.value.type
  source_type    = each.value.source_type
  group_by       = each.value.group_by
  webhook_secret = each.value.webhook_secret
  team_id        = each.value.team_id != null ? each.value.team_id : (each.value.team ? local.team_id : null)
  kafka_topic    = each.value.kafka_topic
  templates      = each.value.templates
  pipeline       = each.value.pipeline
  heartbeat      = each.value.heartbeat

  routes = [
    for r in each.value.routes : {
      name                = r.name
      match_type          = r.match_type
      is_default          = r.is_default
      labels              = r.labels
      pattern             = r.pattern
      escalation_chain_id = r.escalation_chain_key != null ? local.escalation_chain_ids[r.escalation_chain_key] : r.escalation_chain_id
    }
  ]

  notification_policy = each.value.notification_policy == null ? null : {
    channels               = each.value.notification_policy.channels
    batch_timeout_seconds  = each.value.notification_policy.batch_timeout_seconds
    batch_deadline_seconds = each.value.notification_policy.batch_deadline_seconds
    epic_threshold_count   = each.value.notification_policy.epic_threshold_count
    epic_threshold_seconds = each.value.notification_policy.epic_threshold_seconds
    emergency_user_id      = each.value.notification_policy.emergency_user_key != null ? local.user_ids[each.value.notification_policy.emergency_user_key] : each.value.notification_policy.emergency_user_id
    epic_user_id           = each.value.notification_policy.epic_user_key != null ? local.user_ids[each.value.notification_policy.epic_user_key] : each.value.notification_policy.epic_user_id
  }

  legacy_pool = each.value.legacy_pool == null ? null : {
    name              = each.value.legacy_pool.name
    description       = each.value.legacy_pool.description
    emergency_user_id = each.value.legacy_pool.emergency_user_key != null ? local.user_ids[each.value.legacy_pool.emergency_user_key] : each.value.legacy_pool.emergency_user_id
    duty_user_id      = each.value.legacy_pool.duty_user_key != null ? local.user_ids[each.value.legacy_pool.duty_user_key] : each.value.legacy_pool.duty_user_id
    emails            = each.value.legacy_pool.emails
  }
}

###############################################################################
# ChatOps channels
###############################################################################

resource "anomaly_chatops_channel" "this" {
  for_each = var.chatops_channels

  platform = each.value.platform
  name     = each.value.name

  team_id = each.value.team_id != null ? each.value.team_id : (each.value.team ? local.team_id : null)
  user_id = each.value.user_key != null ? local.user_ids[each.value.user_key] : each.value.user_id

  commands_enabled      = each.value.commands_enabled
  notifications_enabled = each.value.notifications_enabled
  webhook_url           = each.value.webhook_url
  external_id           = each.value.external_id
}

###############################################################################
# Maintenance windows
###############################################################################

resource "anomaly_maintenance_window" "this" {
  for_each = var.maintenance_windows

  name      = each.value.name
  reason    = each.value.reason
  starts_at = each.value.starts_at
  ends_at   = each.value.ends_at

  team_id = each.value.team_id != null ? each.value.team_id : (each.value.team ? local.team_id : null)

  integration_ids = each.value.integration_keys != null ? [
    for key in each.value.integration_keys : local.integration_ids[key]
  ] : each.value.integration_ids
}
