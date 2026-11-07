resource "wso2is_organization" "partner" {
  name = "Partner Inc"
}

resource "wso2is_application" "portal" {
  name = "Partner Portal"
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://partners.example.com/callback"]
  }
}

resource "wso2is_application_share" "portal" {
  application_id   = wso2is_application.portal.id
  organization_ids = [wso2is_organization.partner.id]
}
