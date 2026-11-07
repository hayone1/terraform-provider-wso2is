package lookup_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccLookupDataSources(t *testing.T) {
	admin := os.Getenv("WSO2IS_USERNAME")
	if admin == "" {
		admin = "admin"
	}
	name := acctest.RandomName("lookup")
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
data "wso2is_application" "console" {
  name = "Console"
}

data "wso2is_application" "by_client_id" {
  client_id = data.wso2is_application.console.client_id
}

data "wso2is_role" "admin" {
  name = "admin"
}

data "wso2is_user" "admin" {
  username = "` + admin + `"
}

data "wso2is_claim" "email" {
  claim_uri = "http://wso2.org/claims/emailaddress"
}

resource "wso2is_group" "g" {
  name = "` + name + `"
}

data "wso2is_group" "g" {
  name       = wso2is_group.g.name
  depends_on = [wso2is_group.g]
}

resource "wso2is_identity_provider" "idp" {
  name = "` + name + `"
  federated_authenticators = {
    GithubAuthenticator = {
      properties        = { ClientId = "gh", callbackUrl = "https://localhost:9443/commonauth" }
      secret_properties = { ClientSecret = "gh-secret" }
    }
  }
}

data "wso2is_identity_provider" "idp" {
  name       = wso2is_identity_provider.idp.name
  depends_on = [wso2is_identity_provider.idp]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.wso2is_application.by_client_id", "id", "data.wso2is_application.console", "id"),
				resource.TestCheckTypeSetElemAttr("data.wso2is_application.console", "protocols.*", "oauth2"),
				resource.TestCheckResourceAttr("data.wso2is_role.admin", "audience_type", "organization"),
				resource.TestCheckTypeSetElemAttrPair("data.wso2is_user.admin", "role_ids.*", "data.wso2is_role.admin", "id"),
				resource.TestCheckResourceAttr("data.wso2is_claim.email", "attribute_mappings.PRIMARY", "mail"),
				resource.TestCheckResourceAttrPair("data.wso2is_group.g", "id", "wso2is_group.g", "id"),
				resource.TestCheckResourceAttr("data.wso2is_identity_provider.idp", "default_authenticator", "GithubAuthenticator"),
			),
		}},
	})
}

// Names with spaces and with the words "and" or "or" broke unquoted filters
// ("name eq Sales and Marketing" is a syntax error on the server), and the roles API
// cannot filter such names at all.
func TestAccLookupDataSources_namesWithFilterKeywords(t *testing.T) {
	prefix := acctest.RandomName("kw")
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
locals {
  prefix = "` + prefix + `"
}

resource "wso2is_application" "app" {
  name = "${local.prefix} Sales and Marketing"
}

data "wso2is_application" "app" {
  name       = wso2is_application.app.name
  depends_on = [wso2is_application.app]
}

resource "wso2is_role" "role" {
  name = "${local.prefix} Read and Write"
}

data "wso2is_role" "role" {
  name       = wso2is_role.role.name
  depends_on = [wso2is_role.role]
}

resource "wso2is_identity_provider" "idp" {
  name = "${local.prefix} Partners or Vendors"
}

data "wso2is_identity_provider" "idp" {
  name       = wso2is_identity_provider.idp.name
  depends_on = [wso2is_identity_provider.idp]
}

resource "wso2is_api_resource" "api" {
  name       = "${local.prefix} Orders and Billing"
  identifier = "https://${lower(local.prefix)}.example.com/orders"
}

data "wso2is_api_resource" "api" {
  name       = wso2is_api_resource.api.name
  depends_on = [wso2is_api_resource.api]
}

resource "wso2is_organization" "org" {
  name = "${local.prefix} Acme and Co"
}

data "wso2is_organization" "org" {
  name       = wso2is_organization.org.name
  depends_on = [wso2is_organization.org]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrPair("data.wso2is_application.app", "id", "wso2is_application.app", "id"),
				resource.TestCheckResourceAttrPair("data.wso2is_role.role", "id", "wso2is_role.role", "id"),
				resource.TestCheckResourceAttrPair("data.wso2is_identity_provider.idp", "id", "wso2is_identity_provider.idp", "id"),
				resource.TestCheckResourceAttrPair("data.wso2is_api_resource.api", "id", "wso2is_api_resource.api", "id"),
				resource.TestCheckResourceAttrPair("data.wso2is_organization.org", "id", "wso2is_organization.org", "id"),
			),
		}},
	})
}
