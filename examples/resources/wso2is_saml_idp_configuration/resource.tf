resource "wso2is_saml_idp_configuration" "this" {
  metadata_validity_period = 120
  enable_metadata_signing  = true
}
