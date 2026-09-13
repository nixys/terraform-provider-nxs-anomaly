resource "anomaly_user" "alice" {
  name     = "Alice Smith"
  username = "alice.smith"
  email    = "alice@example.com"
  timezone = "Europe/Moscow"

  notification_targets = [
    {
      channel = "telegram"
      target  = "123456789"
    },
    {
      channel = "webhook"
      target  = "https://hooks.example.com/notify"
    }
  ]
}
