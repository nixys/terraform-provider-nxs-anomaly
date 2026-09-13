# Look up an existing object. Alternatively, supply id instead of name.
data "anomaly_maintenance_window" "example" {
  name = "Database upgrade"
}
