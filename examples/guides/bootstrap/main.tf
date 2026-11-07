# Creates an M2M application for Terraform. Apply this configuration once with
# administrator credentials:
#   WSO2IS_BASE_URL=https://id.example.com WSO2IS_USERNAME=admin WSO2IS_PASSWORD=... terraform apply
# Then use the two outputs as WSO2IS_CLIENT_ID and WSO2IS_CLIENT_SECRET in your other
# configurations.
terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# The URL and the credentials come from the environment variables above.
provider "wso2is" {}

locals {
  # The management APIs that the provider's resources call. Remove the APIs of the
  # resources that you do not manage.
  management_apis = toset([
    "/api/server/v1/actions",
    "/api/server/v1/api-resources",
    "/api/server/v1/applications",
    "/api/server/v1/applications/share",
    "/api/server/v1/authenticators",
    "/api/server/v1/authenticators/custom",
    "/api/server/v1/branding-preference",
    "/api/server/v1/claim-dialects",
    "/api/server/v1/configs",
    "/api/server/v1/flow",
    "/api/server/v1/identity-governance",
    "/api/server/v1/identity-providers",
    "/api/server/v1/idv-providers",
    "/api/server/v1/notification-senders",
    "/api/server/v(.*)/notification/(?:email|sms)/template-types",
    "/api/server/v1/oidc/scopes",
    "/api/server/v1/organization-configs/discovery",
    "/api/server/v1/organizations",
    "/api/server/v1/organizations/discovery",
    "/api/server/v1/script-libraries",
    "/api/server/v1/secrets",
    "/api/server/v1/userstore",
    "/api/server/v1/validation-rules",
    "/api/server/v1/vc-templates",
    "/api/server/v1/webhooks",
    "/api/server/v1/webhooks/metadata",
    "/api/server/v1/workflow-associations",
    "/api/server/v1/workflows",
    "/scim2/Groups",
    "/scim2/Roles",
    "/api/server/v2/users", # sharing users with organizations
    "/scim2/Users",
  ])
  # These APIs exist only in the super tenant (carbon.super). They manage tenants and the
  # admin advisory banner.
  system_apis = toset([
    "/api/server/v1/tenants",
    "/api/server/v1/admin-advisory-management/banner",
  ])
}

# The application that Terraform signs in with.
resource "wso2is_application" "terraform" {
  name        = "Terraform"
  description = "Manages this server's configuration"
  template_id = "m2m-application"
  oidc = {
    grant_types = ["client_credentials"]
  }
}

# Look up each management API, and allow the application to use all of its scopes.
data "wso2is_api_resource" "management" {
  for_each   = setunion(local.management_apis, local.system_apis)
  identifier = each.value
}

resource "wso2is_application_authorized_api" "management" {
  for_each        = data.wso2is_api_resource.management
  application_id  = wso2is_application.terraform.id
  api_resource_id = each.value.id
  scopes          = each.value.scope_names
}

output "client_id" {
  value = wso2is_application.terraform.oidc.client_id
}

output "client_secret" {
  value     = wso2is_application.terraform.oidc.client_secret
  sensitive = true
}
