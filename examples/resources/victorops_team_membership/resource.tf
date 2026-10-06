resource "victorops_team_membership" "jdane_platform" {
  team_id   = victorops_team.platform.id
  user_name = victorops_user.jdane.user_name
}
