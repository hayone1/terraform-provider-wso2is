resource "wso2is_custom_authenticator" "branch_pin" {
  name                         = "custom-branch-pin"
  display_name                 = "Branch PIN"
  authentication_type          = "VERIFICATION"
  endpoint_uri                 = "https://pin.example.com/authenticate"
  endpoint_authentication_type = "BEARER"
  endpoint_authentication_properties = {
    accessToken = var.pin_service_token
  }
}

variable "pin_service_token" {
  type      = string
  sensitive = true
}
