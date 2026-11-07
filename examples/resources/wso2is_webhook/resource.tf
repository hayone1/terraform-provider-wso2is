resource "wso2is_webhook" "audit" {
  name     = "Audit events"
  endpoint = "https://hooks.example.com/wso2is"
  secret   = var.webhook_secret
  channels = [
    "https://schemas.identity.wso2.org/events/login",
    "https://schemas.identity.wso2.org/events/user",
  ]
}

variable "webhook_secret" {
  type      = string
  sensitive = true
}
