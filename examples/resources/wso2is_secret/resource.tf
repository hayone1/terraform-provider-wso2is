resource "wso2is_secret" "risk_api_key" {
  name             = "risk_api_key"
  value_wo         = var.risk_api_key # Terraform 1.11+: never stored in state
  value_wo_version = 1
  description      = "Used by adaptive scripts as secrets.risk_api_key"
}

variable "risk_api_key" {
  type      = string
  sensitive = true
  ephemeral = true
}
