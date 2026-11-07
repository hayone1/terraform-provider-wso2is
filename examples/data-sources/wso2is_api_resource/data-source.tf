# Look up a management API to authorize an application for it.
data "wso2is_api_resource" "applications_api" {
  identifier = "/api/server/v1/applications"
}

output "application_management_scopes" {
  value = data.wso2is_api_resource.applications_api.scope_names
}
