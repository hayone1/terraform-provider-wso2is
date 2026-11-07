resource "wso2is_claim_dialect" "legacy" {
  dialect_uri = "urn:example:legacy-claims"
}

resource "wso2is_external_claim" "mail" {
  dialect_uri            = wso2is_claim_dialect.legacy.dialect_uri
  claim_uri              = "mail"
  mapped_local_claim_uri = "http://wso2.org/claims/emailaddress"
}
