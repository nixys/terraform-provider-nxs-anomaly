resource "anomaly_schedule" "weekdays" {
  name     = "Weekdays On-Call"
  timezone = "Europe/Moscow"
  team_id  = anomaly_team.ops.id

  shifts = [
    {
      user_id    = anomaly_user.alice.id
      start_at   = "2026-06-02T09:00:00Z"
      end_at     = "2026-06-02T18:00:00Z"
      recurrence = "weekly"
    }
  ]
}
