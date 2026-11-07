# A custom scope that releases contact details. Claims are OIDC claim names
# (external claims of the http://wso2.org/oidc/claim dialect).
resource "wso2is_oidc_scope" "contact" {
  name         = "contact"
  display_name = "Contact details"
  description  = "Email address and phone number"
  claims       = ["email", "phone_number"]
}

# Custom claims need an OIDC mapping first.
resource "wso2is_external_claim" "department" {
  dialect_uri            = "http://wso2.org/oidc/claim"
  claim_uri              = "department"
  mapped_local_claim_uri = "http://wso2.org/claims/department"
}

resource "wso2is_oidc_scope" "employment" {
  name         = "employment"
  display_name = "Employment details"
  claims       = [wso2is_external_claim.department.claim_uri]
}
