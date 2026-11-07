resource "wso2is_identity_verification_provider" "onfido" {
  name = "Onfido"
  type = "ONFIDO"
  properties = {
    base_url    = "https://api.eu.onfido.com/v3.6"
    workflow_id = "f0b8..."
  }
  secret_properties = {
    token         = var.onfido_token
    webhook_token = var.onfido_webhook_token
  }
  claim_mappings = {
    "http://wso2.org/claims/givenname" = "first_name"
    "http://wso2.org/claims/lastname"  = "last_name"
  }
}

variable "onfido_token" {
  type      = string
  sensitive = true
}

variable "onfido_webhook_token" {
  type      = string
  sensitive = true
}
