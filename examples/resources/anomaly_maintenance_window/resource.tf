resource "anomaly_maintenance_window" "db_upgrade" {
  name            = "PostgreSQL major upgrade"
  reason          = "Planned upgrade; database alerts are expected"
  team_id         = anomaly_team.ops.id
  integration_ids = [anomaly_integration.prometheus.id]
  starts_at       = "2026-06-01T02:00:00Z"
  ends_at         = "2026-06-01T06:00:00Z"
}
