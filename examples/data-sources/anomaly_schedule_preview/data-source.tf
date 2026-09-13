data "anomaly_schedule" "primary" {
  name = "Primary On-call"
}

data "anomaly_schedule_preview" "next_week" {
  schedule_id = data.anomaly_schedule.primary.id
  from        = "2027-01-04T00:00:00Z"
  to          = "2027-01-11T00:00:00Z"
}
