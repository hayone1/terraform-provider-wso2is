resource "wso2is_cors_configuration" "this" {
  supports_credentials = true
  max_age              = 3600
  exposed_headers      = ["Location"]
}
