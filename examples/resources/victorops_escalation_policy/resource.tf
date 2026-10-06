resource "victorops_escalation_policy" "high_severity" {
  name    = "High Severity"
  team_id = victorops_team.platform.id

  step {
    timeout = 0
    entries = [
      {
        type = "rotationGroup"
        slug = "rtg-wvvhXshpvaRdn7jM"
      }
    ]
  }

  step {
    timeout = 10
    entries = [
      {
        type = "rotationGroup"
        slug = "rtg-hfy3fUytq7otMNbf"
      }
    ]
  }
}
