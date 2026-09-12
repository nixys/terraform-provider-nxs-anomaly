resource "anomaly_chatops_channel" "ops_telegram" {
  platform              = "telegram"
  name                  = "ops-alerts"
  team_id               = anomaly_team.ops.id
  commands_enabled      = true
  notifications_enabled = true
}
