resource "victorops_rotation" "primary" {
  team_id = victorops_team.platform.id
  label   = "Primary"

  shift {
    label      = "Weekdays"
    timezone   = "Australia/Sydney"
    start      = "2024-01-01T09:00:00+11:00" # not in the future
    duration   = 7                           # days per turn
    shift_type = "std"
    usernames  = ["jdane", "asmith"]

    mask {
      monday    = true
      tuesday   = true
      wednesday = true
      thursday  = true
      friday    = true

      time_range {
        start_hour   = 0
        start_minute = 0
        end_hour     = 23
        end_minute   = 59
      }
    }
  }
}

# The slug goes in escalation policy steps
resource "victorops_escalation_policy" "primary" {
  name    = "Primary"
  team_id = victorops_team.platform.id
  step {
    timeout = 30
    entries = [{ type = "rotationGroup", slug = victorops_rotation.primary.slug }]
  }
}
