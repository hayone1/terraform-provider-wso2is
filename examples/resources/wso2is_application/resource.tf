# OpenID Connect web application with MFA and requested attributes.
resource "wso2is_application" "portal" {
  name        = "Customer Portal"
  description = "Self-service portal"
  access_url  = "https://portal.example.com"
  template_id = "oidc-web-application"

  oidc = {
    grant_types     = ["authorization_code", "refresh_token"]
    callback_urls   = ["https://portal.example.com/callback"]
    allowed_origins = ["https://portal.example.com"]
    pkce = {
      mandatory = true
    }
    access_token = {
      type                                = "JWT"
      user_access_token_expiry_in_seconds = 900
    }
    refresh_token = {
      expiry_in_seconds   = 86400
      renew_refresh_token = true
    }
    logout = {
      back_channel_logout_url = "https://portal.example.com/logout/backchannel"
    }
  }

  claim_configuration = {
    requested_claims = [
      { uri = "http://wso2.org/claims/emailaddress", mandatory = true },
      { uri = "http://wso2.org/claims/givenname" },
    ]
  }

  authentication_sequence = {
    steps = [
      { options = [{ idp = "LOCAL", authenticator = "BasicAuthenticator" }] },
      { options = [{ idp = "LOCAL", authenticator = "totp" }, { idp = "LOCAL", authenticator = "email-otp-authenticator" }] },
    ]
  }

  advanced_configuration = {
    skip_login_consent = true
  }
}

# Machine-to-machine application (client credentials).
resource "wso2is_application" "backend" {
  name        = "Billing Service"
  template_id = "m2m-application"
  oidc = {
    grant_types = ["client_credentials"]
  }
}

# SAML 2.0 service provider.
resource "wso2is_application" "wiki" {
  name = "Wiki"
  saml = {
    issuer                  = "https://wiki.example.com/saml"
    assertion_consumer_urls = ["https://wiki.example.com/saml/acs"]
    single_sign_on_profile = {
      assertion = {
        name_id_format = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
      }
    }
    attribute_profile = {
      enabled = true
    }
  }
}

output "portal_client_id" {
  value = wso2is_application.portal.oidc.client_id
}
