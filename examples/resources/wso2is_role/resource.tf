resource "wso2is_api_resource" "orders" {
  name       = "Orders API"
  identifier = "https://api.example.com/orders"
  scopes = [
    { name = "orders:read", display_name = "Read orders" },
    { name = "orders:write", display_name = "Change orders" },
  ]
}

# Organization role.
resource "wso2is_role" "order_admins" {
  name        = "order-admins"
  permissions = ["orders:read", "orders:write"]
  depends_on  = [wso2is_api_resource.orders]
}

# Application role (the application uses the APPLICATION audience).
resource "wso2is_application" "orders_ui" {
  name = "Orders UI"
  associated_roles = {
    allowed_audience = "APPLICATION"
  }
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://orders.example.com/callback"]
  }
}

resource "wso2is_application_authorized_api" "orders_ui" {
  application_id  = wso2is_application.orders_ui.id
  api_resource_id = wso2is_api_resource.orders.id
  scopes          = ["orders:read"]
}

resource "wso2is_role" "order_viewer" {
  name          = "viewer"
  audience_type = "application"
  audience_id   = wso2is_application.orders_ui.id
  permissions   = ["orders:read"]
  depends_on    = [wso2is_application_authorized_api.orders_ui]
}
