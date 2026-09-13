data "anomaly_teams" "all" {}

# Map of team name → id.
output "teams_by_name" {
  value = { for t in data.anomaly_teams.all.teams : t.name => t.id }
}
