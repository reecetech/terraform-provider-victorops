resource "victorops_scheduled_override" "vacation" {
  username = "jdane"
  timezone = "America/New_York"
  start    = "2024-12-20T09:00:00Z"
  end      = "2024-12-27T09:00:00Z"
}
