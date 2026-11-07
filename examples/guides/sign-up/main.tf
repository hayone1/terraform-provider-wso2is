terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

provider "wso2is" {}

# 1. The new attribute. A local claim stores it in the user store.
resource "wso2is_claim" "membership_number" {
  claim_uri    = "http://wso2.org/claims/membershipNumber"
  display_name = "Membership number"
  regex        = "^M-[0-9]{4,8}$"
  attribute_mappings = {
    PRIMARY = "membershipNumber"
  }
}

# A SCIM claim shows the attribute in the user APIs and in My Account.
resource "wso2is_external_claim" "membership_number_scim" {
  dialect_uri            = "urn:scim:schemas:extension:custom:User"
  claim_uri              = "urn:scim:schemas:extension:custom:User:membershipNumber"
  mapped_local_claim_uri = wso2is_claim.membership_number.claim_uri
}

# 2. The registration flow. Each field's identifier is the local claim URI that stores
# the value, or "password" for the password. While the flow is enabled, the login page
# shows a "Create an account" link.
resource "wso2is_flow" "sign_up" {
  flow_type = "REGISTRATION"
  enabled   = true
  completion_configs = {
    isAutoLoginEnabled         = "true"
    isEmailVerificationEnabled = "false"
  }
  steps = jsonencode([
    {
      id       = "sign_up"
      type     = "VIEW"
      size     = { width = 350, height = 560 }
      position = { x = 300, y = 200 }
      data = {
        components = [
          { id = "title", category = "DISPLAY", type = "TYPOGRAPHY", variant = "H3", config = { text = "Become a member" } },
          {
            id = "form", category = "BLOCK", type = "FORM", config = {}
            components = [
              { id = "username", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Username", required = true, identifier = "http://wso2.org/claims/username" } },
              { id = "email", category = "FIELD", type = "INPUT", variant = "EMAIL", config = { type = "email", label = "Email", required = true, identifier = "http://wso2.org/claims/emailaddress" } },
              { id = "membership", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Membership number", placeholder = "M-1234", required = true, identifier = wso2is_claim.membership_number.claim_uri } },
              { id = "password", category = "FIELD", type = "INPUT", variant = "PASSWORD", config = { type = "password", label = "Password", required = true, identifier = "password" } },
              {
                id     = "submit", category = "ACTION", type = "BUTTON", variant = "PRIMARY", config = { type = "submit", text = "Create account" }
                action = { type = "EXECUTOR", executor = { name = "PasswordProvisioningExecutor" }, next = "END" }
              },
            ]
          },
        ]
      }
    },
    {
      id       = "END"
      type     = "END"
      size     = { width = 200, height = 100 }
      position = { x = 800, y = 200 }
      data     = { components = [], action = { type = "EXECUTOR", executor = { name = "UserProvisioningExecutor" } } }
    },
  ])
  depends_on = [wso2is_external_claim.membership_number_scim]
}

# 3. Optional: release the attribute in ID tokens through a "membership" scope.
resource "wso2is_external_claim" "membership_number_oidc" {
  dialect_uri            = "http://wso2.org/oidc/claim"
  claim_uri              = "membership_number"
  mapped_local_claim_uri = wso2is_claim.membership_number.claim_uri
}

resource "wso2is_oidc_scope" "membership" {
  name         = "membership"
  display_name = "Membership"
  claims       = [wso2is_external_claim.membership_number_oidc.claim_uri]
}
