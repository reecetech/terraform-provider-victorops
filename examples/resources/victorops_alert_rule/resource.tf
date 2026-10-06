resource "victorops_alert_rule" "database_alerts" {
  alert_field       = "host_name"
  alert_value_match = "db-*"
  match_type        = "WILDCARD"
  routing_key       = victorops_routing_key.platform_alerts.name
  stop_flag         = false
  notes             = "Route database alerts to platform team"
}
