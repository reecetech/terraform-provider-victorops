resource "victorops_user" "jdane" {
  first_name       = "John"
  last_name        = "Dane"
  user_name        = "jdane"
  email            = "jdane@example.com"
  replacement_user = "default_user" # username that takes over this user's items on delete
}
