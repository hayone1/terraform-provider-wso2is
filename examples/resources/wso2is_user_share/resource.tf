resource "wso2is_organization" "partner" {
  name = "Partner Inc"
}

# An application shared with the organization, with an application role.
resource "wso2is_application" "portal" {
  name = "Partner Portal"
  associated_roles = {
    allowed_audience = "APPLICATION"
  }
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://portal.example.com/callback"]
  }
}

resource "wso2is_role" "support_agent" {
  name          = "support-agent"
  audience_type = "application"
  audience_id   = wso2is_application.portal.id
}

resource "wso2is_application_share" "portal" {
  application_id   = wso2is_application.portal.id
  organization_ids = [wso2is_organization.partner.id]
  depends_on       = [wso2is_role.support_agent]
}

# A support engineer of the root organization who works in the partner's organization.
resource "wso2is_user" "support" {
  username = "sam"
  email    = "sam@example.com"
  password = var.sam_password
}

resource "wso2is_user_share" "support" {
  user_id = wso2is_user.support.id
  organizations = [{
    organization_id  = wso2is_organization.partner.id
    include_children = true
    roles = [{
      name          = wso2is_role.support_agent.name
      audience_type = "application"
      audience_name = wso2is_application.portal.name
    }]
  }]
  # Application roles reach the organization when the application is shared.
  depends_on = [wso2is_application_share.portal]
}

variable "sam_password" {
  type      = string
  sensitive = true
}
