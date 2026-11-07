resource "wso2is_dcr_configuration" "this" {
  authentication_required = true
  mandate_ssa             = true
  ssa_jwks                = "https://directory.example.com/jwks"
}
