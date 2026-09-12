data "anomaly_schedule" "primary" {
  name = "Primary On-call"
}

data "anomaly_user" "replacement" {
  username = "alice.smith"
}

resource "anomaly_schedule_override" "example" {
  schedule_id = data.anomaly_schedule.primary.id
  user_id     = data.anomaly_user.replacement.id
  start_at    = "2027-01-04T09:00:00Z"
  until       = "2027-01-05T09:00:00Z"
  reason      = "Vacation cover"
}
