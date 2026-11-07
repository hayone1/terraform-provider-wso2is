resource "wso2is_user" "alice" {
  username = "alice"
  email    = "alice@example.com"
  password = var.alice_password
}

resource "wso2is_group" "developers" {
  name = "developers"
}

resource "wso2is_group_members" "developers" {
  group_id = wso2is_group.developers.id
  user_ids = [wso2is_user.alice.id]
}

variable "alice_password" {
  type      = string
  sensitive = true
}
