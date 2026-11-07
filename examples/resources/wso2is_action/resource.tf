resource "wso2is_action" "token_claims" {
  type                = "pre_issue_access_token"
  name                = "Add tenant claims"
  endpoint_uri        = "https://hooks.example.com/pre-issue-access-token"
  authentication_type = "API_KEY"
  authentication_properties = {
    header = "x-api-key"
    value  = var.hook_api_key
  }
  allowed_headers = ["x-request-id"]
  rules = [{
    expressions = [{ field = "grantType", operator = "equals", value = "client_credentials" }]
  }]
}

variable "hook_api_key" {
  type      = string
  sensitive = true
}
