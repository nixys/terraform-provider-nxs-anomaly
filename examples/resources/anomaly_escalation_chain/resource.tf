resource "anomaly_escalation_chain" "critical" {
  name = "Critical Incidents"

  steps = [
    {
      kind     = "notify_user"
      user_ids = [anomaly_user.alice.id]
    },
    {
      kind             = "wait"
      duration_seconds = 300
    },
    {
      kind        = "notify_schedule"
      schedule_id = anomaly_schedule.weekdays.id
    },
    {
      kind        = "trigger_webhook"
      webhook_url = "https://hooks.example.com/pagerduty-bridge"
    }
  ]
}
