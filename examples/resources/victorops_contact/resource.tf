resource "victorops_contact" "jdane_cell" {
  user_name = victorops_user.jdane.user_name
  type      = "phone"
  value     = "+13038674309"
  label     = "Cell Phone"
}

resource "victorops_contact" "jdane_email" {
  user_name = victorops_user.jdane.user_name
  type      = "email"
  value     = "jdane@example.com"
  label     = "Work Email"
}
