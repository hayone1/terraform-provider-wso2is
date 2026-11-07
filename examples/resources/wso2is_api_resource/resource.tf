resource "wso2is_api_resource" "orders" {
  name        = "Orders API"
  identifier  = "https://api.example.com/orders"
  description = "Order management"
  scopes = [
    { name = "orders:read", display_name = "Read orders" },
    { name = "orders:write", display_name = "Change orders", description = "Create and update orders" },
  ]
}
