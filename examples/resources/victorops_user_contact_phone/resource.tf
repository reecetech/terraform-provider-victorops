resource "victorops_user_contact_phone" "jdane_mobile" {
  username = victorops_user.jdane.user_name
  phone    = "+1-555-123-4567"
  label    = "Mobile"
}
