resource "wso2is_user" "alice" {
  username    = "alice"
  given_name  = "Alice"
  family_name = "Smith"
  email       = "alice@example.com"
  emails = {
    work = "alice@corp.example"
  }
  phone_numbers = {
    mobile = "+15551234567"
  }
  attributes = {
    "urn:scim:wso2:schema:country"                                          = "Sri Lanka"
    "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department" = "Engineering"
  }
  password_wo         = var.alice_password # Terraform 1.11+
  password_wo_version = 1
}

# Invite a user to set their own password (needs an email provider).
resource "wso2is_user" "bob" {
  username     = "bob"
  email        = "bob@example.com"
  ask_password = true
}

variable "alice_password" {
  type      = string
  sensitive = true
  ephemeral = true
}
