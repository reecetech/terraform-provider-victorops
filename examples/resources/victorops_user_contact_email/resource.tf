resource "victorops_user_contact_email" "jdane_work" {
  username = victorops_user.jdane.user_name
  email    = "jdane-alerts@example.com"
  label    = "Work Email"
}
