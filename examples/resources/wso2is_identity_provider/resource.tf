resource "wso2is_identity_provider" "google" {
  name        = "Google"
  template_id = "google-idp"
  federated_authenticators = {
    GoogleOIDCAuthenticator = {
      properties = {
        ClientId    = "1234.apps.googleusercontent.com"
        callbackUrl = "https://id.example.com/commonauth"
      }
      secret_properties = {
        ClientSecret = var.google_client_secret
      }
    }
  }
  jit_provisioning = {
    enabled = true
    scheme  = "PROVISION_SILENTLY"
  }
}

resource "wso2is_identity_provider" "corporate" {
  name     = "Corporate SSO"
  jwks_uri = "https://sso.corp.example/jwks"
  federated_authenticators = {
    OpenIDConnectAuthenticator = {
      properties = {
        ClientId         = "wso2is"
        OAuth2AuthzEPUrl = "https://sso.corp.example/authorize"
        OAuth2TokenEPUrl = "https://sso.corp.example/token"
        UserInfoUrl      = "https://sso.corp.example/userinfo"
        callbackUrl      = "https://id.example.com/commonauth"
        Scopes           = "openid email profile"
      }
      secret_properties = {
        ClientSecret = var.corporate_client_secret
      }
    }
  }
  claims = {
    user_id_claim = "email"
    mappings = [
      { idp_claim = "email", local_claim = "http://wso2.org/claims/emailaddress" },
    ]
  }
  role_mappings = [
    { idp_role = "admins", local_role = "Internal/admin" },
  ]
}

variable "google_client_secret" {
  type      = string
  sensitive = true
}

variable "corporate_client_secret" {
  type      = string
  sensitive = true
}
