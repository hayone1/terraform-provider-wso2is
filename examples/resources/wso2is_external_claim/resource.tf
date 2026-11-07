resource "wso2is_external_claim" "department" {
  dialect_uri            = "http://wso2.org/oidc/claim"
  claim_uri              = "department"
  mapped_local_claim_uri = "http://wso2.org/claims/department"
}
