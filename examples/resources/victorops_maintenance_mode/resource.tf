# Specific routing keys
resource "victorops_maintenance_mode" "deploy" {
  routing_keys = [victorops_routing_key.platform_alerts.name]
  purpose      = "Scheduled deployment"
}

# Global (all routing keys)
resource "victorops_maintenance_mode" "global" {
  purpose = "Infrastructure maintenance"
}
