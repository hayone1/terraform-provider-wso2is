package action_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

// Actions are limited to one per type per tenant, so this test must not run in parallel
// with other action tests.
func TestAccAction(t *testing.T) {
	name := acctest.RandomName("act")
	cfg := func(authType, authProps, rules string, active bool) string {
		return fmt.Sprintf(`
resource "wso2is_application" "app" {
  name = %[1]q
}

resource "wso2is_action" "token" {
  type                = "pre_issue_access_token"
  name                = "%[1]s-token"
  description         = "Adds tenant claims"
  endpoint_uri        = "https://hooks.example.com/pre-issue-access-token"
  authentication_type = %[2]q
  authentication_properties = %[3]s
  allowed_headers     = ["x-request-id"]
  rules               = %[4]s
  active              = %[5]t
}

resource "wso2is_action" "password" {
  type                    = "pre_update_password"
  name                    = "%[1]s-password"
  endpoint_uri            = "https://hooks.example.com/pre-update-password"
  authentication_type     = "NONE"
  password_sharing_format = "SHA256_HASHED"
}

resource "wso2is_action" "profile" {
  type                = "pre_update_profile"
  name                = "%[1]s-profile"
  endpoint_uri        = "https://hooks.example.com/pre-update-profile"
  authentication_type = "BEARER"
  authentication_properties = { accessToken = "hook-token" }
  attributes          = ["http://wso2.org/claims/givenname", "http://wso2.org/claims/lastname"]
}
`, name, authType, authProps, rules, active)
	}
	rule1 := `[{ expressions = [{ field = "application", operator = "equals", value = wso2is_application.app.id }] }]`
	rule2 := `[
    { expressions = [{ field = "grantType", operator = "equals", value = "client_credentials" }] },
    { expressions = [
      { field = "application", operator = "equals", value = wso2is_application.app.id },
      { field = "grantType", operator = "notEquals", value = "refresh_token" },
    ] },
  ]`
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("BASIC", `{ username = "hook", password = "hook-secret" }`, rule1, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_action.token", "active", "true"),
					resource.TestCheckResourceAttr("wso2is_action.token", "rules.#", "1"),
					resource.TestCheckResourceAttr("wso2is_action.password", "password_sharing_format", "SHA256_HASHED"),
					resource.TestCheckResourceAttr("wso2is_action.profile", "attributes.#", "2"),
				),
			},
			{
				Config: cfg("API_KEY", `{ header = "x-api-key", value = "k-123" }`, rule2, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_action.token", "authentication_type", "API_KEY"),
					resource.TestCheckResourceAttr("wso2is_action.token", "active", "false"),
					resource.TestCheckResourceAttr("wso2is_action.token", "rules.#", "2"),
					resource.TestCheckResourceAttr("wso2is_action.token", "rules.1.expressions.#", "2"),
				),
			},
			{
				ResourceName:            "wso2is_action.token",
				ImportState:             true,
				ImportStateIdFunc:       importID("wso2is_action.token", "pre_issue_access_token"),
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"authentication_properties"},
			},
		},
	})
}
