resource "anomaly_team" "ops" {
  name = "Ops Team"
  member_ids = [
    anomaly_user.alice.id,
  ]
}
