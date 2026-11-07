package organization_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func TestAccOrganization(t *testing.T) {
	name := acctest.RandomName("org")
	cfg := func(desc string, enabled bool, shared string) string {
		return fmt.Sprintf(`
resource "wso2is_organization" "parent" {
  name        = %[1]q
  description = %[2]q
  attributes = {
    region = "eu-west"
  }
}

resource "wso2is_organization" "child" {
  name    = "%[1]s-sibling"
  enabled = %[3]t
}

resource "wso2is_application" "portal" {
  name = "%[1]s-portal"
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://portal.example.com/cb"]
  }
}

resource "wso2is_application_share" "portal" {
  application_id   = wso2is_application.portal.id
  organization_ids = %[4]s
}

data "wso2is_organization" "by_name" {
  name       = wso2is_organization.child.name
  depends_on = [wso2is_organization.child]
}
`, name, desc, enabled, shared)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Partner A", true, `[wso2is_organization.parent.id]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_organization.parent", "type", "TENANT"),
					resource.TestCheckResourceAttrSet("wso2is_organization.parent", "parent_id"),
					resource.TestCheckResourceAttr("wso2is_organization.parent", "attributes.region", "eu-west"),
					resource.TestCheckResourceAttrPair("wso2is_organization.child", "parent_id", "wso2is_organization.parent", "parent_id"),
					resource.TestCheckResourceAttr("wso2is_application_share.portal", "organization_ids.#", "1"),
					resource.TestCheckResourceAttrPair("data.wso2is_organization.by_name", "id", "wso2is_organization.child", "id"),
				),
			},
			{
				Config: cfg("Partner A (renamed)", false, `[wso2is_organization.parent.id, wso2is_organization.child.id]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_organization.parent", "description", "Partner A (renamed)"),
					resource.TestCheckResourceAttr("wso2is_organization.child", "enabled", "false"),
					resource.TestCheckResourceAttr("wso2is_application_share.portal", "organization_ids.#", "2"),
				),
			},
			{ResourceName: "wso2is_organization.parent", ImportState: true, ImportStateVerify: true},
			{ResourceName: "wso2is_application_share.portal", ImportState: true, ImportStateVerify: true},
		},
	})
}

// TestAccOrganizationScopedProvider manages resources inside a sub-organization with a
// provider configured with organization_id (organization_switch grant).
func TestAccOrganizationScopedProvider(t *testing.T) {
	acctest.PreCheck(t)
	orgID, clientID, clientSecret := acctest.TestOrganization(t)
	name := strings.ReplaceAll(acctest.RandomName("o"), "-", "")
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(orgScopedConfig, orgID, clientID, clientSecret, name, ""),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("wso2is_user.member", "id"),
				resource.TestCheckResourceAttr("wso2is_group_members.staff", "user_ids.#", "1"),
				resource.TestCheckResourceAttr("wso2is_role.org_admin", "audience_id", orgID),
				resource.TestCheckResourceAttr("wso2is_role_assignment.org_admin", "group_ids.#", "1"),
				resource.TestCheckResourceAttrSet("wso2is_application.org_app", "oidc.client_id"),
				// Organization applications use the root organization's issuer by default.
				resource.TestCheckResourceAttrSet("wso2is_application.org_app", "oidc.token_issuer_organization_id"),
				resource.TestCheckResourceAttr("wso2is_organization.nested", "parent_id", orgID),
			),
		}, {
			Config: fmt.Sprintf(orgScopedConfig, orgID, clientID, clientSecret, name, fmt.Sprintf("token_issuer_organization_id = %q", orgID)),
			Check:  resource.TestCheckResourceAttr("wso2is_application.org_app", "oidc.token_issuer_organization_id", orgID),
		}},
	})
}

const orgScopedConfig = `
provider "wso2is" {
  organization_id = %[1]q
  client_id       = %[2]q
  client_secret   = %[3]q
}

# Nested organization, created from inside its parent.
resource "wso2is_organization" "nested" {
  name = "%[4]s-nested"
}

resource "wso2is_user" "member" {
  username = "%[4]suser"
  password = "Org-Member-Passw0rd!"
  email    = "%[4]s@partner.example"
}

resource "wso2is_group" "staff" {
  name = "%[4]s-staff"
}

resource "wso2is_group_members" "staff" {
  group_id = wso2is_group.staff.id
  user_ids = [wso2is_user.member.id]
}

resource "wso2is_role" "org_admin" {
  name = "%[4]s-admin"
}

resource "wso2is_role_assignment" "org_admin" {
  role_id   = wso2is_role.org_admin.id
  group_ids = [wso2is_group.staff.id]
}

resource "wso2is_application" "org_app" {
  name = "%[4]s-app"
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://partner.example/cb"]
    %[5]s
  }
}
`

func TestAccOrganization_singleConfiguration(t *testing.T) {
	acctest.PreCheck(t)
	name := strings.ReplaceAll(acctest.RandomName("o"), "-", "")
	resource.Test(t, resource.TestCase{
		ExternalProviders: acctest.ExternalProviders(t),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
resource "wso2is_organization" "partner" {
  name = "%[1]s-partner"
}

resource "wso2is_application" "b2b_admin" {
  name = "%[1]s-b2b-admin"
  oidc = {
    grant_types = ["client_credentials", "organization_switch"]
  }
}

data "wso2is_api_resource" "org_groups" {
  identifier = "/o/scim2/Groups"
}

resource "wso2is_application_authorized_api" "org_groups" {
  application_id  = wso2is_application.b2b_admin.id
  api_resource_id = data.wso2is_api_resource.org_groups.id
  scopes          = data.wso2is_api_resource.org_groups.scope_names
}

resource "wso2is_application_share" "b2b_admin" {
  application_id   = wso2is_application.b2b_admin.id
  organization_ids = [wso2is_organization.partner.id]
  # The organization switch needs the authorization in place.
  depends_on = [wso2is_application_authorized_api.org_groups]
}

provider "wso2is" {
  alias = "partner"
  # Taken from the share so the provider is configured after the application is shared.
  organization_id = one(wso2is_application_share.b2b_admin.organization_ids)
  client_id       = wso2is_application.b2b_admin.oidc.client_id
  client_secret   = wso2is_application.b2b_admin.oidc.client_secret
}

resource "wso2is_group" "partner_staff" {
  provider = wso2is.partner
  name     = "%[1]s-staff"
}
`, name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("wso2is_group.partner_staff", "id"),
				func(s *terraform.State) error {
					// The group lives in the organization, not in the root organization.
					groupID := s.RootModule().Resources["wso2is_group.partner_staff"].Primary.ID
					c := acctest.Client(t)
					if _, err := c.GetGroup(acctest.Context(t), groupID); !client.IsNotFound(err) && !client.IsSCIMNotFound(err) {
						return fmt.Errorf("group %s found in the root organization (err=%w)", groupID, err)
					}
					return nil
				},
			),
		}},
	})
}

func TestAccOrganizationDiscovery(t *testing.T) {
	name := acctest.RandomName("disc")
	cfg := func(domains string, selfSignUp bool) string {
		return fmt.Sprintf(`
resource "wso2is_organization_discovery_configuration" "this" {
  email_domain_enabled                    = true
  email_domain_based_self_sign_up_enabled = %[3]t
  default_param                           = "orgHandle"
}

resource "wso2is_organization" "partner" {
  name = %[1]q
}

resource "wso2is_organization_discovery" "partner" {
  organization_id = wso2is_organization.partner.id
  email_domains   = %[2]s
  depends_on      = [wso2is_organization_discovery_configuration.this]
}
`, name, domains, selfSignUp)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			c := acctest.Client(t)
			var out any
			if err := c.Get(acctest.Context(t), c.ServerPath("/organization-configs/discovery"), nil, &out); !client.IsNotFound(err) {
				return fmt.Errorf("organization discovery configuration still present (err=%w)", err)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: cfg(fmt.Sprintf(`["%s.example"]`, name), false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_organization_discovery_configuration.this", "email_domain_enabled", "true"),
					resource.TestCheckResourceAttr("wso2is_organization_discovery_configuration.this", "default_param", "orgHandle"),
					resource.TestCheckResourceAttr("wso2is_organization_discovery.partner", "email_domains.#", "1"),
				),
			},
			{
				Config: cfg(fmt.Sprintf(`["%[1]s.example", "eu.%[1]s.example"]`, name), true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_organization_discovery_configuration.this", "email_domain_based_self_sign_up_enabled", "true"),
					resource.TestCheckTypeSetElemAttr("wso2is_organization_discovery.partner", "email_domains.*", "eu."+name+".example"),
				),
			},
			{ResourceName: "wso2is_organization_discovery_configuration.this", ImportState: true, ImportStateId: "organization_discovery_configuration", ImportStateVerify: true},
			{ResourceName: "wso2is_organization_discovery.partner", ImportState: true, ImportStateVerify: true},
		},
	})
}

func TestAccUserShare(t *testing.T) {
	name := strings.ReplaceAll(acctest.RandomName("us"), "-", "")
	cfg := func(share string) string {
		return fmt.Sprintf(`
resource "wso2is_organization" "partner" {
  name = "%[1]s-partner"
}

resource "wso2is_application" "portal" {
  name = "%[1]s-portal"
  associated_roles = {
    allowed_audience = "APPLICATION"
  }
  oidc = {
    grant_types   = ["authorization_code"]
    callback_urls = ["https://portal.example/cb"]
  }
}

resource "wso2is_role" "viewer" {
  name          = "viewer"
  audience_type = "application"
  audience_id   = wso2is_application.portal.id
}

resource "wso2is_application_share" "portal" {
  application_id   = wso2is_application.portal.id
  organization_ids = [wso2is_organization.partner.id]
  depends_on       = [wso2is_role.viewer]
}

resource "wso2is_user" "consultant" {
  username = "%[1]s"
  email    = "%[1]s@example.com"
  password = "Consultant-Passw0rd!"
}

resource "wso2is_user_share" "consultant" {
  user_id = wso2is_user.consultant.id
  %[2]s
  depends_on = [wso2is_application_share.portal]
}
`, name, share)
	}
	withRole := `organizations = [{
    organization_id = wso2is_organization.partner.id
    roles = [{ name = "viewer", audience_type = "application", audience_name = wso2is_application.portal.name }]
  }]`
	withChildren := `organizations = [{
    organization_id  = wso2is_organization.partner.id
    include_children = true
  }]`
	withAll := `share_with_all = {
    roles = [{ name = "viewer", audience_type = "application", audience_name = wso2is_application.portal.name }]
  }`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(withRole),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_user_share.consultant", "organizations.#", "1"),
					resource.TestCheckResourceAttr("wso2is_user_share.consultant", "organizations.0.roles.#", "1"),
					resource.TestCheckNoResourceAttr("wso2is_user_share.consultant", "organizations.0.include_children"),
				),
			},
			{
				Config: cfg(withChildren),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_user_share.consultant", "organizations.0.include_children", "true"),
					resource.TestCheckNoResourceAttr("wso2is_user_share.consultant", "organizations.0.roles"),
				),
			},
			{
				Config: cfg(withAll),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_user_share.consultant", "share_with_all.roles.#", "1"),
					resource.TestCheckNoResourceAttr("wso2is_user_share.consultant", "organizations"),
				),
			},
			{ResourceName: "wso2is_user_share.consultant", ImportState: true, ImportStateVerify: true},
			{
				// Back from "all" to one organization.
				Config: cfg(withRole),
				Check:  resource.TestCheckResourceAttr("wso2is_user_share.consultant", "organizations.#", "1"),
			},
		},
	})
}
