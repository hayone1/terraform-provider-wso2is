resource "wso2is_remote_logging" "audit" {
  log_type               = "AUDIT"
  remote_url             = "https://logs.example.com/wso2is/audit"
  connect_timeout_millis = "5000"
  username               = "wso2is"
  password               = var.log_password
}

variable "log_password" {
  type      = string
  sensitive = true
}
