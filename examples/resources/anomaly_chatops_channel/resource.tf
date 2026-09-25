resource "anomaly_chatops_channel" "ops_telegram" {
  platform              = "telegram"
  name                  = "ops-alerts"
  team_id               = anomaly_team.ops.id
  commands_enabled      = true
  notifications_enabled = true
}

resource "anomaly_chatops_channel" "ops_mattermost" {
  platform    = "mattermost"
  name        = "ops-mattermost"
  webhook_url = "env:MATTERMOST_WEBHOOK_URL"
  # For a gateway in front of the chat that authenticates its callers.
  headers = { "X-Api-Key" = "env:CHAT_GATEWAY_KEY" }
}
