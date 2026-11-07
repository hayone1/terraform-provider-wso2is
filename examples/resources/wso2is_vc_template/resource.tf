# Custom attributes reach credentials through the VC claim dialect.
resource "wso2is_external_claim" "department_vc" {
  dialect_uri            = "http://wso2.org/vc/claim"
  claim_uri              = "department"
  mapped_local_claim_uri = "http://wso2.org/claims/department"
}

resource "wso2is_vc_template" "employee_badge" {
  identifier   = "EmployeeBadge"
  display_name = "Employee badge"
  format       = "dc+sd-jwt"
  expires_in   = 31536000 # one year
  claims = {
    given_name  = "given_name"
    family_name = "family_name"
    email       = "email"
    department  = wso2is_external_claim.department_vc.claim_uri
  }
}
