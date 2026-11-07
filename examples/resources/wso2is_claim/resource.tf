resource "wso2is_claim" "employee_id" {
  claim_uri            = "http://wso2.org/claims/employeeId"
  display_name         = "Employee ID"
  description          = "HR employee number"
  regex                = "^E[0-9]{5}$"
  supported_by_default = true
  attribute_mappings = {
    PRIMARY = "employeeId"
  }
}

# Expose it in SCIM and in OIDC tokens.
resource "wso2is_external_claim" "employee_id_scim" {
  dialect_uri            = "urn:scim:schemas:extension:custom:User"
  claim_uri              = "urn:scim:schemas:extension:custom:User:employeeId"
  mapped_local_claim_uri = wso2is_claim.employee_id.claim_uri
}

resource "wso2is_external_claim" "employee_id_oidc" {
  dialect_uri            = "http://wso2.org/oidc/claim"
  claim_uri              = "employee_id"
  mapped_local_claim_uri = wso2is_claim.employee_id.claim_uri
}
