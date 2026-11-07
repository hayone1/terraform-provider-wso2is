# The super tenant's administrator creates tenants. This needs basic authentication.
provider "wso2is" {
  username = "admin"
  password = var.super_admin_password
}

resource "wso2is_tenant" "acme" {
  domain           = "acme.com"
  owner_username   = "admin"
  owner_password   = var.acme_admin_password
  owner_email      = "it@acme.com"
  owner_first_name = "Acme"
  owner_last_name  = "Admin"
}

# A second provider block manages the new tenant. It signs in as the tenant owner.
provider "wso2is" {
  alias         = "acme"
  tenant_domain = wso2is_tenant.acme.domain
  username      = "${wso2is_tenant.acme.owner_username}@${wso2is_tenant.acme.domain}"
  password      = var.acme_admin_password
}

resource "wso2is_group" "acme_staff" {
  provider = wso2is.acme
  name     = "staff"
}

variable "super_admin_password" {
  type      = string
  sensitive = true
}

variable "acme_admin_password" {
  type      = string
  sensitive = true
}
