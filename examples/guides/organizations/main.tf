terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# The root organization. The provider signs in with administrator credentials or an
# M2M application, read from the WSO2IS_* environment variables.
provider "wso2is" {}

# A customer organization.
resource "wso2is_organization" "partner" {
  name        = "Partner Inc"
  description = "Reseller in the EU"
}

# The portal that customers sign in to. It is shared with every organization.
resource "wso2is_application" "portal" {
  name = "Partner Portal"
  oidc = {
    grant_types   = ["authorization_code", "refresh_token"]
    callback_urls = ["https://portal.example.com/callback"]
  }
}

resource "wso2is_application_share" "portal" {
  application_id          = wso2is_application.portal.id
  share_with_all_children = true
}

# The application that Terraform uses to make changes inside the organization. It needs
# the client_credentials and organization_switch grants.
resource "wso2is_application" "b2b_admin" {
  name = "Organization administration"
  oidc = {
    grant_types = ["client_credentials", "organization_switch"]
  }
}

# Allow the application to call the organization APIs (their identifiers start with /o/).
data "wso2is_api_resource" "org" {
  for_each = toset([
    "/o/scim2/Users",
    "/o/scim2/Groups",
    "/o/scim2/Roles",
    "/o/api/server/v1/organizations",
    "/o/api/server/v1/applications",
  ])
  identifier = each.value
}

resource "wso2is_application_authorized_api" "org" {
  for_each        = data.wso2is_api_resource.org
  application_id  = wso2is_application.b2b_admin.id
  api_resource_id = each.value.id
  scopes          = each.value.scope_names
}

# Share the application with the organization, after the application may call the APIs.
resource "wso2is_application_share" "b2b_admin" {
  application_id   = wso2is_application.b2b_admin.id
  organization_ids = [wso2is_organization.partner.id]
  depends_on       = [wso2is_application_authorized_api.org]
}

# A provider for the organization. It reads organization_id from the share, so Terraform
# configures it only after the application is shared and may call the APIs.
provider "wso2is" {
  alias           = "partner"
  organization_id = one(wso2is_application_share.b2b_admin.organization_ids)
  client_id       = wso2is_application.b2b_admin.oidc.client_id
  client_secret   = wso2is_application.b2b_admin.oidc.client_secret
}

# Everything below this line is created inside the organization.
resource "wso2is_user" "partner_admin" {
  provider = wso2is.partner
  username = "pat"
  email    = "pat@partner.example"
  password = var.partner_admin_password
}

resource "wso2is_group" "partner_admins" {
  provider = wso2is.partner
  name     = "administrators"
}

resource "wso2is_group_members" "partner_admins" {
  provider = wso2is.partner
  group_id = wso2is_group.partner_admins.id
  user_ids = [wso2is_user.partner_admin.id]
}

resource "wso2is_role" "partner_user_managers" {
  provider    = wso2is.partner
  name        = "user-managers"
  permissions = ["internal_org_user_mgt_view", "internal_org_user_mgt_list", "internal_org_user_mgt_create", "internal_org_user_mgt_update"]
}

resource "wso2is_role_assignment" "partner_user_managers" {
  provider  = wso2is.partner
  role_id   = wso2is_role.partner_user_managers.id
  group_ids = [wso2is_group.partner_admins.id]
}

# A sub-organization. Sub-organizations are created with their parent's provider.
resource "wso2is_organization" "partner_eu" {
  provider = wso2is.partner
  name     = "Partner EU"
}

variable "partner_admin_password" {
  type      = string
  sensitive = true
}
