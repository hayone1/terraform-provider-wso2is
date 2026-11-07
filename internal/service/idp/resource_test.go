package idp_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func exists(ctx context.Context, c *client.Client, id string) (bool, error) {
	_, err := c.GetIdentityProvider(ctx, id)
	if client.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func TestAccIdentityProvider_oidc(t *testing.T) {
	name := acctest.RandomName("oidc")
	config := func(scopes, secret, desc string, jit bool) string {
		return fmt.Sprintf(`
resource "wso2is_identity_provider" "test" {
  name        = %[1]q
  description = %[4]q
  template_id = "enterprise-oidc-idp"
  jwks_uri    = "https://op.example.com/jwks"

  federated_authenticators = {
    OpenIDConnectAuthenticator = {
      properties = {
        ClientId         = "tf-client"
        OAuth2AuthzEPUrl = "https://op.example.com/authorize"
        OAuth2TokenEPUrl = "https://op.example.com/token"
        UserInfoUrl      = "https://op.example.com/userinfo"
        callbackUrl      = "https://localhost:9443/commonauth"
        Scopes           = %[2]q
        IsPKCEEnabled    = "true"
      }
      secret_properties = {
        ClientSecret = %[3]q
      }
    }
  }

  claims = {
    user_id_claim = "email"
    mappings = [
      { idp_claim = "email", local_claim = "http://wso2.org/claims/emailaddress" },
      { idp_claim = "given_name", local_claim = "http://wso2.org/claims/givenname" },
    ]
  }

  role_mappings = [
    { idp_role = "op-admins", local_role = "Internal/admin" },
  ]

  jit_provisioning = {
    enabled              = %[5]t
    scheme               = "PROVISION_SILENTLY"
    associate_local_user = true
    account_lookup_attribute_mappings = [
      { local_attribute = "http://wso2.org/claims/emailaddress", federated_attribute = "email" },
    ]
  }
}`, name, scopes, secret, desc, jit)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_identity_provider", exists),
		Steps: []resource.TestStep{
			{
				Config: config("openid email profile", "first-secret", "Corporate SSO", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "enabled", "true"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "default_authenticator", "OpenIDConnectAuthenticator"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "federated_authenticators.OpenIDConnectAuthenticator.properties.ClientId", "tf-client"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "federated_authenticators.OpenIDConnectAuthenticator.secret_properties.ClientSecret", "first-secret"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "claims.mappings.#", "2"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "jit_provisioning.enabled", "true"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "jit_provisioning.account_lookup_attribute_mappings.#", "1"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "jwks_uri", "https://op.example.com/jwks"),
				),
			},
			{
				Config: config("openid email", "rotated-secret", "Corporate SSO (staging)", false),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_identity_provider.test", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "description", "Corporate SSO (staging)"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "federated_authenticators.OpenIDConnectAuthenticator.properties.Scopes", "openid email"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "federated_authenticators.OpenIDConnectAuthenticator.secret_properties.ClientSecret", "rotated-secret"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.test", "jit_provisioning.enabled", "false"),
				),
			},
			{
				ResourceName:            "wso2is_identity_provider.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"federated_authenticators"},
			},
		},
	})
}

// TestAccIdentityProvider_socialAndLogin wires a Google connection into an application's
// login flow, and checks that the connection cannot be deleted while it is in use unless
// force_delete is set.
func TestAccIdentityProvider_socialAndLogin(t *testing.T) {
	name := acctest.RandomName("google")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_identity_provider", exists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_identity_provider" "google" {
  name        = %[1]q
  template_id = "google-idp"
  image_url   = "assets/images/logos/google.svg"
  federated_authenticators = {
    GoogleOIDCAuthenticator = {
      properties = {
        ClientId    = "1234.apps.googleusercontent.com"
        callbackUrl = "https://localhost:9443/commonauth"
        AdditionalQueryParameters = "scope=email openid profile"
      }
      secret_properties = {
        ClientSecret = "google-secret"
      }
    }
  }
  force_delete = true
}

resource "wso2is_application" "app" {
  name = %[1]q
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://app.example.com/cb"]
  }
  authentication_sequence = {
    steps = [{
      options = [
        { idp = "LOCAL", authenticator = "BasicAuthenticator" },
        { idp = wso2is_identity_provider.google.name, authenticator = "GoogleOIDCAuthenticator" },
      ]
    }]
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application.app", "authentication_sequence.steps.0.options.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("wso2is_application.app", "authentication_sequence.steps.0.options.*", map[string]string{"idp": name, "authenticator": "GoogleOIDCAuthenticator"}),
				),
			},
		},
	})
}

func TestAccIdentityProvider_samlWithCertificateAndProvisioning(t *testing.T) {
	name := acctest.RandomName("saml")
	cert, err := os.ReadFile("testdata/cert.pem")
	if err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_identity_provider", exists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "wso2is_federated_authenticator_metadata" "saml" {
  name = "SAMLSSOAuthenticator"
}

resource "wso2is_identity_provider" "saml" {
  name         = %[1]q
  certificates = [%[2]q]
  federated_authenticators = {
    SAMLSSOAuthenticator = {
      properties = {
        IdPEntityId      = "https://saml.example.com/idp"
        SPEntityId       = "wso2is-%[1]s"
        SSOUrl           = "https://saml.example.com/sso"
        NameIDType       = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
        ISAuthnReqSigned = "false"
      }
    }
  }
  outbound_provisioning_roles = ["admin"]
  outbound_provisioning_connectors = {
    SCIM2 = {
      properties = {
        "scim2-user-ep"  = "https://scim.example.com/Users"
        "scim2-group-ep" = "https://scim.example.com/Groups"
        "scim2-username" = "provisioner"
      }
      secret_properties = {
        "scim2-password" = "provisioner-secret"
      }
    }
  }
}`, name, string(cert)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckTypeSetElemNestedAttrs("data.wso2is_federated_authenticator_metadata.saml", "properties.*", map[string]string{"key": "IdPEntityId"}),
					resource.TestCheckResourceAttr("wso2is_identity_provider.saml", "certificates.#", "1"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.saml", "default_outbound_connector", "SCIM2"),
					resource.TestCheckResourceAttr("wso2is_identity_provider.saml", "outbound_provisioning_connectors.SCIM2.properties.scim2-username", "provisioner"),
				),
			},
		},
	})
}
