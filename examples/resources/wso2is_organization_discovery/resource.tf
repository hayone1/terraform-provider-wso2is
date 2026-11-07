# Turn on email domain discovery for the tenant first.
resource "wso2is_organization_discovery_configuration" "this" {
  email_domain_enabled = true
}

resource "wso2is_organization" "partner" {
  name = "Partner Inc"
}

# Users with an @partner.example or @partner-eu.example address go to Partner Inc's login.
resource "wso2is_organization_discovery" "partner" {
  organization_id = wso2is_organization.partner.id
  email_domains   = ["partner.example", "partner-eu.example"]
  depends_on      = [wso2is_organization_discovery_configuration.this]
}
