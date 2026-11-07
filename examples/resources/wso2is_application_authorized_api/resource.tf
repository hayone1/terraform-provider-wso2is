resource "wso2is_api_resource" "orders" {
  name       = "Orders API"
  identifier = "https://api.example.com/orders"
  scopes = [
    { name = "orders:read", display_name = "Read orders" },
    { name = "orders:write", display_name = "Change orders" },
  ]
}

resource "wso2is_application" "backend" {
  name = "Order Processor"
  oidc = {
    grant_types = ["client_credentials"]
  }
}

resource "wso2is_application_authorized_api" "orders" {
  application_id  = wso2is_application.backend.id
  api_resource_id = wso2is_api_resource.orders.id
  scopes          = ["orders:read", "orders:write"]
}

# Authorize a management API, for example SCIM user management.
data "wso2is_api_resource" "scim_users" {
  identifier = "/scim2/Users"
}

resource "wso2is_application_authorized_api" "user_management" {
  application_id  = wso2is_application.backend.id
  api_resource_id = data.wso2is_api_resource.scim_users.id
  scopes          = data.wso2is_api_resource.scim_users.scope_names
}
