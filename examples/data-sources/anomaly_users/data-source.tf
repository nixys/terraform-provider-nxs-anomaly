data "anomaly_users" "all" {}

# Use the first user's ID in another resource.
output "first_user_id" {
  value = try(data.anomaly_users.all.users[0].id, null)
}

# Build a map of username → id for easy lookup.
output "users_by_username" {
  value = { for u in data.anomaly_users.all.users : u.username => u.id }
}
