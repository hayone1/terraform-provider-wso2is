package apiresource_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func exists(ctx context.Context, c *client.Client, id string) (bool, error) {
	_, err := c.GetAPIResource(ctx, id)
	if client.IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func TestAccAPIResource_lifecycle(t *testing.T) {
	name := acctest.RandomName("api")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_api_resource", exists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_api_resource" "test" {
  name       = %[1]q
  identifier = "https://%[1]s.example.com"
  scopes = [
    { name = "%[1]s:read", display_name = "Read" },
    { name = "%[1]s:write", display_name = "Write", description = "Change things" },
  ]
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "type", "BUSINESS"),
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "requires_authorization", "true"),
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "scopes.#", "2"),
					resource.TestCheckResourceAttrSet("wso2is_api_resource.test", "scope_ids."+name+":read"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "wso2is_api_resource" "test" {
  name        = "%[1]s renamed"
  identifier  = "https://%[1]s.example.com"
  description = "Orders API"
  scopes = [
    { name = "%[1]s:read", display_name = "Read everything" },
    { name = "%[1]s:admin", display_name = "Administer" },
  ]
  authorization_details_types = [{
    type        = "%[1]s_payment"
    name        = "Payment"
    description = "Initiate a payment"
    schema = jsonencode({
      type     = "object"
      required = ["type"]
      properties = {
        type   = { type = "string", enum = ["%[1]s_payment"] }
        amount = { type = "number" }
      }
    })
  }]
}`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_api_resource.test", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "name", name+" renamed"),
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "description", "Orders API"),
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "scopes.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("wso2is_api_resource.test", "scopes.*", map[string]string{"name": name + ":read", "display_name": "Read everything"}),
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "authorization_details_types.#", "1"),
				),
			},
			{
				ResourceName:      "wso2is_api_resource.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// requires_authorization can only be set at creation.
				Config: fmt.Sprintf(`
resource "wso2is_api_resource" "test" {
  name                   = "%[1]s renamed"
  identifier             = "https://%[1]s.example.com"
  description            = "Orders API"
  requires_authorization = false
}`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_api_resource.test", plancheck.ResourceActionReplace)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "requires_authorization", "false"),
					resource.TestCheckResourceAttr("wso2is_api_resource.test", "scopes.#", "0"),
				),
			},
		},
	})
}

func TestAccAPIResource_authorizeApplication(t *testing.T) {
	name := acctest.RandomName("authz")
	config := func(scopes string) string {
		return fmt.Sprintf(`
resource "wso2is_api_resource" "api" {
  name       = %[1]q
  identifier = "https://%[1]s.example.com"
  scopes = [
    { name = "%[1]s:read", display_name = "Read" },
    { name = "%[1]s:write", display_name = "Write" },
  ]
}

resource "wso2is_application" "app" {
  name = %[1]q
  oidc = {
    grant_types = ["client_credentials"]
  }
}

resource "wso2is_application_authorized_api" "test" {
  application_id  = wso2is_application.app.id
  api_resource_id = wso2is_api_resource.api.id
  scopes          = %[2]s
}

# Management API, looked up by identifier.
data "wso2is_api_resource" "users" {
  identifier = "/scim2/Users"
}

resource "wso2is_application_authorized_api" "users" {
  application_id  = wso2is_application.app.id
  api_resource_id = data.wso2is_api_resource.users.id
  scopes          = data.wso2is_api_resource.users.scope_names
}
`, name, scopes)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(fmt.Sprintf(`["%s:read"]`, name)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application_authorized_api.test", "scopes.#", "1"),
					resource.TestCheckResourceAttr("wso2is_application_authorized_api.test", "policy_identifier", "RBAC"),
					resource.TestCheckResourceAttr("wso2is_application_authorized_api.test", "identifier", "https://"+name+".example.com"),
					resource.TestCheckResourceAttr("data.wso2is_api_resource.users", "type", "TENANT"),
					resource.TestCheckResourceAttrSet("wso2is_application_authorized_api.users", "scopes.#"),
					tokenScopes("wso2is_application.app", name, []string{name + ":read"}, []string{name + ":write"}),
				),
			},
			{
				Config: config(fmt.Sprintf(`["%[1]s:read", "%[1]s:write"]`, name)),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_application_authorized_api.test", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application_authorized_api.test", "scopes.#", "2"),
					tokenScopes("wso2is_application.app", name, []string{name + ":read", name + ":write"}, nil),
				),
			},
			{
				ResourceName:      "wso2is_application_authorized_api.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(fmt.Sprintf(`["%s:write"]`, name)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_application_authorized_api.test", "scopes.#", "1"),
					tokenScopes("wso2is_application.app", name, []string{name + ":write"}, []string{name + ":read"}),
				),
			},
		},
	})
}

// tokenScopes requests a client credentials token with the application's credentials,
// asking for both API scopes, and checks which ones the server grants.
func tokenScopes(app, name string, want, notWant []string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[app]
		if !ok {
			return fmt.Errorf("%s not in state", app)
		}
		form := url.Values{"grant_type": {"client_credentials"}, "scope": {name + ":read " + name + ":write"}}
		req, err := http.NewRequest(http.MethodPost, strings.TrimRight(os.Getenv("WSO2IS_BASE_URL"), "/")+"/oauth2/token", strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		req.SetBasicAuth(rs.Primary.Attributes["oidc.client_id"], rs.Primary.Attributes["oidc.client_secret"])
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		insecure := os.Getenv("WSO2IS_INSECURE_SKIP_VERIFY") == "true"
		hc := &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}}} //nolint:gosec // test servers
		resp, err := hc.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		var tok struct {
			Scope string `json:"scope"`
			Error string `json:"error_description"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("token request failed: %d %s", resp.StatusCode, tok.Error)
		}
		granted := map[string]bool{}
		for _, sc := range strings.Fields(tok.Scope) {
			granted[sc] = true
		}
		for _, w := range want {
			if !granted[w] {
				return fmt.Errorf("scope %q not granted (token scopes: %q)", w, tok.Scope)
			}
		}
		for _, n := range notWant {
			if granted[n] {
				return fmt.Errorf("scope %q granted although not authorized (token scopes: %q)", n, tok.Scope)
			}
		}
		return nil
	}
}
