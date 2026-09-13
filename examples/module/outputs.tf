output "user_ids" {
  description = "Map of user_key => user ID."
  value       = local.user_ids
}

output "users" {
  description = "Full objects of the created users (user_key => resource)."
  value       = anomaly_user.this
}

output "team_id" {
  description = "Module team ID (null when team_name is not set)."
  value       = local.team_id
}

output "schedule_ids" {
  description = "Map of schedule_key => schedule ID."
  value       = local.schedule_ids
}

output "schedule_override_ids" {
  description = "Map of override key => schedule override ID."
  value       = { for k, override in anomaly_schedule_override.this : k => override.id }
}

output "escalation_chain_ids" {
  description = "Map of escalation_chain_key => escalation chain ID."
  value       = local.escalation_chain_ids
}

output "integration_ids" {
  description = "Map of integration key => ID."
  value       = local.integration_ids
}

output "integration_keys" {
  description = <<-EOT
    Map of integration key => computed `key` for the webhook URL.
    The URL is `<provider url>/integrations/v1/<type>/<key>`.
  EOT
  value       = { for k, i in anomaly_integration.this : k => i.key }
}

output "chatops_channel_ids" {
  description = "Map of channel key => ChatOps channel ID."
  value       = { for k, c in anomaly_chatops_channel.this : k => c.id }
}

output "maintenance_window_ids" {
  description = "Map of maintenance window key => ID."
  value       = { for k, w in anomaly_maintenance_window.this : k => w.id }
}
