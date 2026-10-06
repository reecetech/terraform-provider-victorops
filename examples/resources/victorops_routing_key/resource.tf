resource "victorops_routing_key" "platform_alerts" {
  name    = "platform-alerts"
  targets = [victorops_escalation_policy.high_severity.id]
}
