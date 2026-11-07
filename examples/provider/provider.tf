terraform {
  required_providers {
    wso2is = {
      source  = "hayone1/wso2is"
      version = "~> 0.1"
    }
  }
}

# Option 1: basic authentication with an administrator account. The provider reads the
# username and password from WSO2IS_USERNAME and WSO2IS_PASSWORD.
provider "wso2is" {
  base_url = "https://id.example.com"
}

# Option 2 (recommended for automation): the client credentials of an M2M application
# that may call the management APIs.
provider "wso2is" {
  alias         = "m2m"
  base_url      = "https://id.example.com"
  client_id     = var.client_id
  client_secret = var.client_secret
}

# Option 3: a sub-organization (B2B customer). The provider uses the application's
# credentials to get an organization token with the organization_switch grant.
provider "wso2is" {
  alias           = "partner"
  base_url        = "https://id.example.com"
  organization_id = "8f1c...-..."
  client_id       = var.client_id
  client_secret   = var.client_secret
}

variable "client_id" {
  type = string
}

variable "client_secret" {
  type      = string
  sensitive = true
}
