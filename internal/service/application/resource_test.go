package application_test

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func appExists(ctx context.Context, c *client.Client, id string) (bool, error) {
	_, err := c.GetApplication(ctx, id)
	if client.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func TestAccApplication_oidcLifecycle(t *testing.T) {
	name := acctest.RandomName("app")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_application", appExists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_application" "test" {
  name = %q
  oidc = {
    grant_types   = ["authorization_code", "refresh_token"]
    callback_urls = ["https://app.example.com/callback"]
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("wso2is_application.test", "id"),
					resource.TestCheckResourceAttr("wso2is_application.test", "name", name),
					resource.TestCheckResourceAttr("wso2is_application.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("wso2is_application.test", "oidc.client_id"),
					resource.TestCheckResourceAttrSet("wso2is_application.test", "oidc.client_secret"),
					resource.TestCheckResourceAttrPair("wso2is_application.test", "client_id", "wso2is_application.test", "oidc.client_id"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.state", "ACTIVE"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.public_client", "false"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.access_token.type", "Default"),
					resource.TestCheckResourceAttr("wso2is_application.test", "authentication_sequence.type", "DEFAULT"),
					resource.TestCheckResourceAttr("wso2is_application.test", "claim_configuration.dialect", "LOCAL"),
					resource.TestCheckResourceAttr("wso2is_application.test", "associated_roles.allowed_audience", "APPLICATION"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "wso2is_application" "test" {
  name              = %q
  description       = "updated by terraform"
  access_url        = "https://app.example.com"
  logout_return_url = "https://app.example.com/bye"

  oidc = {
    grant_types     = ["authorization_code", "refresh_token", "client_credentials"]
    callback_urls   = ["regexp=(https://app.example.com/callback|https://app.example.com/silent)"]
    allowed_origins = ["https://app.example.com"]
    pkce = {
      mandatory = true
    }
    access_token = {
      type                                = "JWT"
      user_access_token_expiry_in_seconds = 900
      access_token_attributes             = ["email", "username"]
    }
    refresh_token = {
      expiry_in_seconds   = 7200
      renew_refresh_token = true
    }
    id_token = {
      expiry_in_seconds = 600
      audience          = ["https://api.example.com"]
    }
    logout = {
      back_channel_logout_url = "https://app.example.com/bclogout"
    }
  }

  claim_configuration = {
    requested_claims = [
      { uri = "http://wso2.org/claims/emailaddress", mandatory = true },
      { uri = "http://wso2.org/claims/username" },
    ]
    subject = {
      claim = "http://wso2.org/claims/username"
    }
  }

  authentication_sequence = {
    steps = [
      { options = [{ idp = "LOCAL", authenticator = "BasicAuthenticator" }] },
      { options = [
        { idp = "LOCAL", authenticator = "totp" },
        { idp = "LOCAL", authenticator = "email-otp-authenticator" },
      ] },
    ]
    script = <<-EOT
      var onLoginRequest = function(context) {
        executeStep(1);
        executeStep(2);
      };
    EOT
  }

  advanced_configuration = {
    skip_login_consent        = true
    skip_logout_consent       = true
    discoverable_by_end_users = true
  }
}`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_application.test", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application.test", "description", "updated by terraform"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.grant_types.#", "3"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.pkce.mandatory", "true"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.access_token.type", "JWT"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.access_token.user_access_token_expiry_in_seconds", "900"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.refresh_token.renew_refresh_token", "true"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.id_token.expiry_in_seconds", "600"),
					resource.TestCheckResourceAttr("wso2is_application.test", "oidc.logout.back_channel_logout_url", "https://app.example.com/bclogout"),
					resource.TestCheckResourceAttr("wso2is_application.test", "claim_configuration.requested_claims.#", "2"),
					resource.TestCheckResourceAttr("wso2is_application.test", "claim_configuration.subject.claim", "http://wso2.org/claims/username"),
					resource.TestCheckResourceAttr("wso2is_application.test", "authentication_sequence.type", "USER_DEFINED"),
					resource.TestCheckResourceAttr("wso2is_application.test", "authentication_sequence.steps.#", "2"),
					resource.TestCheckResourceAttr("wso2is_application.test", "authentication_sequence.steps.1.options.#", "2"),
					resource.TestCheckResourceAttr("wso2is_application.test", "advanced_configuration.skip_login_consent", "true"),
					resource.TestCheckResourceAttr("wso2is_application.test", "advanced_configuration.discoverable_by_end_users", "true"),
				),
			},
			{
				ResourceName:            "wso2is_application.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"authentication_sequence.script"},
			},
			{
				// Removing OIDC and adding SAML on the same application.
				Config: fmt.Sprintf(`
resource "wso2is_application" "test" {
  name = %q
  saml = {
    issuer                  = "%s-sp"
    assertion_consumer_urls = ["https://sp.example.com/saml/acs"]
    single_sign_on_profile = {
      assertion = {
        name_id_format = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
      }
    }
    attribute_profile = {
      enabled = true
    }
  }
}`, name, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("wso2is_application.test", "oidc"),
					resource.TestCheckResourceAttr("wso2is_application.test", "saml.issuer", name+"-sp"),
					resource.TestCheckResourceAttr("wso2is_application.test", "saml.default_assertion_consumer_url", "https://sp.example.com/saml/acs"),
					resource.TestCheckResourceAttr("wso2is_application.test", "saml.single_sign_on_profile.assertion.name_id_format", "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"),
					resource.TestCheckResourceAttr("wso2is_application.test", "saml.attribute_profile.enabled", "true"),
					resource.TestCheckResourceAttr("wso2is_application.test", "issuer", name+"-sp"),
				),
			},
		},
	})
}

func TestAccApplication_m2mCustomCredentials(t *testing.T) {
	name := acctest.RandomName("m2m")
	clientID := "tfacc_" + name[len(name)-8:]
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_application", appExists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_application" "m2m" {
  name        = %q
  template_id = "m2m-application"
  oidc = {
    client_id     = %q
    client_secret = "tfacc-very-secret-value-123"
    grant_types   = ["client_credentials"]
    access_token = {
      application_access_token_expiry_in_seconds = 1200
    }
  }
}`, name, clientID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application.m2m", "oidc.client_id", clientID),
					resource.TestCheckResourceAttr("wso2is_application.m2m", "oidc.client_secret", "tfacc-very-secret-value-123"),
					resource.TestCheckResourceAttr("wso2is_application.m2m", "template_id", "m2m-application"),
					resource.TestCheckResourceAttr("wso2is_application.m2m", "oidc.access_token.application_access_token_expiry_in_seconds", "1200"),
				),
			},
			{
				// Changing the secret is only possible by replacing the application.
				Config: fmt.Sprintf(`
resource "wso2is_application" "m2m" {
  name        = %q
  template_id = "m2m-application"
  oidc = {
    client_id     = %q
    client_secret = "tfacc-rotated-secret-value-456"
    grant_types   = ["client_credentials"]
    access_token = {
      application_access_token_expiry_in_seconds = 1200
    }
  }
}`, name, clientID),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_application.m2m", plancheck.ResourceActionReplace)},
				},
				Check: resource.TestCheckResourceAttr("wso2is_application.m2m", "oidc.client_secret", "tfacc-rotated-secret-value-456"),
			},
		},
	})
}

func TestAccApplication_publicSPA(t *testing.T) {
	name := acctest.RandomName("spa")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_application", appExists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_application" "spa" {
  name        = %q
  template_id = "single-page-application"
  oidc = {
    grant_types     = ["authorization_code", "refresh_token"]
    callback_urls   = ["https://spa.example.com"]
    allowed_origins = ["https://spa.example.com"]
    public_client   = true
    pkce = {
      mandatory                         = true
      support_plain_transform_algorithm = false
    }
    access_token = {
      binding_type                              = "sso-session"
      revoke_tokens_when_idp_session_terminated = true
      validate_token_binding                    = true
    }
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application.spa", "oidc.public_client", "true"),
					resource.TestCheckResourceAttrSet("wso2is_application.spa", "oidc.client_id"),
					resource.TestCheckResourceAttr("wso2is_application.spa", "oidc.access_token.binding_type", "sso-session"),
				),
			},
		},
	})
}

func TestAccApplication_disappears(t *testing.T) {
	name := acctest.RandomName("app")
	config := fmt.Sprintf(`
resource "wso2is_application" "test" {
  name = %q
}`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_application", appExists),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: acctest.DeleteOutOfBand(t, "wso2is_application.test", func(ctx context.Context, c *client.Client, id string) error {
					return c.DeleteApplication(ctx, id)
				}),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_application.test", plancheck.ResourceActionCreate)},
				},
			},
		},
	})
}

func TestAccApplication_passiveSTS(t *testing.T) {
	name := acctest.RandomName("wsfed")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_application", appExists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_application" "wsfed" {
  name = %q
  passive_sts = {
    realm    = "urn:%s"
    reply_to = "https://rp.example.com/wsfed"
  }
}`, name, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application.wsfed", "passive_sts.realm", "urn:"+name),
					resource.TestCheckResourceAttr("wso2is_application.wsfed", "passive_sts.reply_to", "https://rp.example.com/wsfed"),
				),
			},
		},
	})
}

func TestAccApplication_duplicateName(t *testing.T) {
	name := acctest.RandomName("dup")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_application" "a" {
  name = %q
}
resource "wso2is_application" "b" {
  name       = %q
  depends_on = [wso2is_application.a]
}`, name, name),
				ExpectError: regexp.MustCompile(`(?i)already exists|conflict|409`),
			},
		},
	})
}
