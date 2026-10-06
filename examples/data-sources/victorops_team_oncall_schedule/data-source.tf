data "victorops_team_oncall_schedule" "platform" {
  team_id      = victorops_team.platform.id
  days_forward = 14
}
