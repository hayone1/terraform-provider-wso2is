package identity_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func userExists(ctx context.Context, c *client.Client, id string) (bool, error) {
	_, err := c.GetUser(ctx, id)
	if client.IsSCIMNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

func TestAccUser_lifecycle(t *testing.T) {
	name := strings.ReplaceAll(acctest.RandomName("user"), "-", "")
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_user", userExists),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_user" "test" {
  username    = %[1]q
  password    = "Initial-Passw0rd!"
  given_name  = "Ada"
  family_name = "Lovelace"
  email       = "%[1]s@example.com"
  emails = {
    work = "%[1]s.work@example.com"
  }
  phone_numbers = {
    mobile = "+15551234567"
  }
  attributes = {
    "urn:scim:wso2:schema:country"                                        = "Sri Lanka"
    "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department" = "Engineering"
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("wso2is_user.test", "id"),
					resource.TestCheckResourceAttr("wso2is_user.test", "user_store", "PRIMARY"),
					resource.TestCheckResourceAttr("wso2is_user.test", "email", name+"@example.com"),
					resource.TestCheckResourceAttr("wso2is_user.test", "emails.work", name+".work@example.com"),
					resource.TestCheckResourceAttr("wso2is_user.test", "phone_numbers.mobile", "+15551234567"),
					resource.TestCheckResourceAttr("wso2is_user.test", "attributes.urn:scim:wso2:schema:country", "Sri Lanka"),
					resource.TestCheckResourceAttr("wso2is_user.test", "locked", "false"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "wso2is_user" "test" {
  username     = %[1]q
  password     = "Changed-Passw0rd!"
  given_name   = "Augusta Ada"
  display_name = "Ada L."
  email        = "%[1]s@example.org"
  locked       = true
  attributes = {
    "urn:scim:wso2:schema:country" = "Nigeria"
  }
}`, name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_user.test", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_user.test", "given_name", "Augusta Ada"),
					resource.TestCheckNoResourceAttr("wso2is_user.test", "family_name"),
					resource.TestCheckResourceAttr("wso2is_user.test", "display_name", "Ada L."),
					resource.TestCheckResourceAttr("wso2is_user.test", "email", name+"@example.org"),
					resource.TestCheckNoResourceAttr("wso2is_user.test", "emails"),
					resource.TestCheckNoResourceAttr("wso2is_user.test", "phone_numbers"),
					resource.TestCheckResourceAttr("wso2is_user.test", "locked", "true"),
					resource.TestCheckResourceAttr("wso2is_user.test", "attributes.%", "1"),
					checkUserAttribute(t, "wso2is_user.test", client.SCIMEnterpriseUser+":department", ""),
				),
			},
			{
				ResourceName:            "wso2is_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "attributes", "ask_password"},
			},
		},
	})
}

func TestAccUser_writeOnlyPassword(t *testing.T) {
	name := strings.ReplaceAll(acctest.RandomName("wo"), "-", "")
	cfg := func(pw string, version int) string {
		return fmt.Sprintf(`
resource "wso2is_user" "test" {
  username            = %q
  password_wo         = %q
  password_wo_version = %d
  email               = "%s@example.com"
}`, name, pw, version, name)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             acctest.CheckDestroyed(t, "wso2is_user", userExists),
		Steps: []resource.TestStep{
			{
				Config: cfg("Wr1te-Only-Passw0rd!", 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("wso2is_user.test", "password_wo"),
					resource.TestCheckResourceAttr("wso2is_user.test", "password_wo_version", "1"),
				),
			},
			{
				Config: cfg("Wr1te-Only-Passw0rd-2!", 2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("wso2is_user.test", plancheck.ResourceActionUpdate)},
				},
			},
		},
	})
}

func checkUserAttribute(t *testing.T, address, p, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := acctest.ResourceID(s, address)
		if err != nil {
			return err
		}
		u, err := acctest.Client(t).GetUser(context.Background(), id)
		if err != nil {
			return err
		}
		v, _ := client.LookupSCIM(u, p)
		got, _ := client.SCIMString(v)
		if got != want {
			return fmt.Errorf("user attribute %s = %q, want %q", p, got, want)
		}
		return nil
	}
}

func TestAccGroupAndRoles(t *testing.T) {
	name := strings.ReplaceAll(acctest.RandomName("grp"), "-", "")
	config := func(members, roleUsers, perms string) string {
		return fmt.Sprintf(`
resource "wso2is_user" "a" {
  username = "%[1]sa"
  password = "Member-Passw0rd!"
  email    = "%[1]sa@example.com"
}

resource "wso2is_user" "b" {
  username = "%[1]sb"
  password = "Member-Passw0rd!"
  email    = "%[1]sb@example.com"
}

resource "wso2is_group" "devs" {
  name = "%[1]s-devs"
}

resource "wso2is_group_members" "devs" {
  group_id = wso2is_group.devs.id
  user_ids = %[2]s
}

resource "wso2is_api_resource" "api" {
  name       = %[1]q
  identifier = "https://%[1]s.example.com"
  scopes = [
    { name = "%[1]s:read", display_name = "Read" },
    { name = "%[1]s:write", display_name = "Write" },
  ]
}

resource "wso2is_role" "org" {
  name        = "%[1]s-org-role"
  permissions = %[4]s
  depends_on  = [wso2is_api_resource.api]
}

resource "wso2is_role_assignment" "org" {
  role_id   = wso2is_role.org.id
  user_ids  = %[3]s
  group_ids = [wso2is_group.devs.id]
}

resource "wso2is_application" "app" {
  name = "%[1]s-app"
  associated_roles = {
    allowed_audience = "APPLICATION"
  }
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://%[1]s.example.com/cb"]
  }
}

resource "wso2is_application_authorized_api" "app" {
  application_id  = wso2is_application.app.id
  api_resource_id = wso2is_api_resource.api.id
  scopes          = ["%[1]s:read", "%[1]s:write"]
}

resource "wso2is_role" "app" {
  name          = "%[1]s-app-role"
  audience_type = "application"
  audience_id   = wso2is_application.app.id
  permissions   = ["%[1]s:read"]
  depends_on    = [wso2is_application_authorized_api.app]
}
`, name, members, roleUsers, perms)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(`[wso2is_user.a.id]`, `[wso2is_user.b.id]`, fmt.Sprintf(`["%s:read"]`, name)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_group.devs", "user_store", "PRIMARY"),
					resource.TestCheckResourceAttr("wso2is_group_members.devs", "user_ids.#", "1"),
					resource.TestCheckResourceAttr("wso2is_role.org", "audience_type", "organization"),
					resource.TestCheckResourceAttrSet("wso2is_role.org", "audience_id"),
					resource.TestCheckResourceAttr("wso2is_role.org", "permissions.#", "1"),
					resource.TestCheckResourceAttr("wso2is_role_assignment.org", "user_ids.#", "1"),
					resource.TestCheckResourceAttr("wso2is_role_assignment.org", "group_ids.#", "1"),
					resource.TestCheckResourceAttr("wso2is_role.app", "audience_type", "application"),
					resource.TestCheckResourceAttrPair("wso2is_role.app", "audience_id", "wso2is_application.app", "id"),
					resource.TestCheckResourceAttrPair("wso2is_role.app", "audience_name", "wso2is_application.app", "name"),
				),
			},
			{
				// Change membership, assignments and permissions; rename the group.
				Config: strings.Replace(config(`[wso2is_user.a.id, wso2is_user.b.id]`, `[]`, fmt.Sprintf(`["%[1]s:read", "%[1]s:write"]`, name)), name+"-devs", name+"-developers", 1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_group.devs", "name", name+"-developers"),
					resource.TestCheckResourceAttr("wso2is_group_members.devs", "user_ids.#", "2"),
					resource.TestCheckResourceAttr("wso2is_role.org", "permissions.#", "2"),
					resource.TestCheckResourceAttr("wso2is_role_assignment.org", "user_ids.#", "0"),
					resource.TestCheckResourceAttr("wso2is_role_assignment.org", "group_ids.#", "1"),
				),
			},
			{ResourceName: "wso2is_group.devs", ImportState: true, ImportStateVerify: true},
			{ResourceName: "wso2is_group_members.devs", ImportState: true, ImportStateVerify: true},
			{ResourceName: "wso2is_role.org", ImportState: true, ImportStateVerify: true},
			{ResourceName: "wso2is_role.app", ImportState: true, ImportStateVerify: true},
			{ResourceName: "wso2is_role_assignment.org", ImportState: true, ImportStateVerify: true},
		},
	})
}
