# All integrations.
data "anomaly_integrations" "all" {}

# Only alertmanager integrations.
data "anomaly_integrations" "alertmanager" {
  type = "alertmanager"
}

# Webhook URL for the first alertmanager integration.
output "alertmanager_webhook_url" {
  value = length(data.anomaly_integrations.alertmanager.integrations) > 0 ? (
    "http://nxs-anomaly.example.com/integrations/v1/alertmanager/${data.anomaly_integrations.alertmanager.integrations[0].key}"
  ) : ""
}

# Map of integration name → key for use in alerting config.
output "integration_keys" {
  value = { for i in data.anomaly_integrations.all.integrations : i.name => i.key }
}
