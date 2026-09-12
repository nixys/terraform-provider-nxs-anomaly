# Look up an existing object. Alternatively, supply id instead of name.
data "anomaly_schedule" "example" {
  name = "Primary On-call"
}
