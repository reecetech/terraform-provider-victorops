resource "victorops_user_paging_policy" "jdane" {
  username = victorops_user.jdane.user_name

  step {
    timeout = 5

    rule {
      type = "push"
    }
  }

  step {
    timeout = 0

    rule {
      type = "phone"

      contact {
        id   = 12345 # contact ID
        type = "phone"
      }
    }
  }
}
