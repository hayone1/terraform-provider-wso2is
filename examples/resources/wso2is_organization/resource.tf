resource "wso2is_organization" "partner" {
  name        = "Partner Inc"
  description = "Reseller"
  attributes = {
    region = "eu-west"
  }
}

# To manage resources inside the organization, use an application that can switch into it:
# client credentials + organization_switch, authorized for the organization APIs it needs,
# and shared with the organization.
resource "wso2is_application" "b2b_admin" {
  name = "Partner administration"
  oidc = {
    grant_types = ["client_credentials", "organization_switch"]
  }
}

data "wso2is_api_resource" "org_groups" {
  identifier = "/o/scim2/Groups"
}

resource "wso2is_application_authorized_api" "org_groups" {
  application_id  = wso2is_application.b2b_admin.id
  api_resource_id = data.wso2is_api_resource.org_groups.id
  scopes          = data.wso2is_api_resource.org_groups.scope_names
}

resource "wso2is_application_share" "b2b_admin" {
  application_id   = wso2is_application.b2b_admin.id
  organization_ids = [wso2is_organization.partner.id]
  depends_on       = [wso2is_application_authorized_api.org_groups]
}

provider "wso2is" {
  alias = "partner"
  # Read from the share, so the provider is configured only once the application is shared.
  organization_id = one(wso2is_application_share.b2b_admin.organization_ids)
  client_id       = wso2is_application.b2b_admin.oidc.client_id
  client_secret   = wso2is_application.b2b_admin.oidc.client_secret
}

resource "wso2is_group" "partner_admins" {
  provider = wso2is.partner
  name     = "partner-admins"
}
