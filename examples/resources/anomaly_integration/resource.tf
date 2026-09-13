resource "anomaly_integration" "prometheus" {
  name     = "Prometheus Alertmanager"
  type     = "alertmanager"
  group_by = ["alertname", "cluster", "namespace"]

  webhook_secret = var.webhook_secret

  routes = [
    {
      name                = "critical-route"
      escalation_chain_id = anomaly_escalation_chain.critical.id
      conditions = [
        {
          field = "severity"
          op    = "eq"
          value = "critical"
        }
      ]
    },
    {
      name                = "default-route"
      escalation_chain_id = anomaly_escalation_chain.critical.id
      conditions          = []
    }
  ]
}

variable "webhook_secret" {
  type      = string
  sensitive = true
  default   = ""
}

output "webhook_url" {
  value = "http://localhost:8080/integrations/v1/alertmanager/${anomaly_integration.prometheus.key}"
}
