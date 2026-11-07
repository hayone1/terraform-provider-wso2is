resource "wso2is_userstore" "customers" {
  name        = "CUSTOMERS"
  type        = "UniqueIDJDBCUserStoreManager"
  description = "Customer accounts"
  properties = {
    url        = "jdbc:postgresql://db.example.com:5432/customers"
    driverName = "org.postgresql.Driver"
    userName   = "wso2"
  }
  secret_properties = {
    password = var.customers_db_password
  }
}

resource "wso2is_user" "customer" {
  username   = "jane"
  user_store = wso2is_userstore.customers.name
  email      = "jane@example.com"
  password   = var.jane_password
}

variable "customers_db_password" {
  type      = string
  sensitive = true
}

variable "jane_password" {
  type      = string
  sensitive = true
}
