data "victorops_users" "all" {}

data "victorops_users" "by_email" {
  email = "jdane@example.com"
}
