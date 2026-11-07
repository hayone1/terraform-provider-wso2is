package claim_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

// TestAccClaim_customAttributeEndToEnd creates a custom user attribute, exposes it in
// SCIM and OIDC, adds it to an OIDC scope and sets it on a user through SCIM.
func TestAccClaim_customAttributeEndToEnd(t *testing.T) {
	suffix := strings.ReplaceAll(acctest.RandomName("c"), "-", "")
	config := func(displayName, scopeClaims string, required bool) string {
		return fmt.Sprintf(`
resource "wso2is_claim" "employee_id" {
  claim_uri    = "http://wso2.org/claims/%[1]s"
  display_name = %[2]q
  description  = "Employee number"
  regex        = "^E[0-9]{5}$"
  required     = %[4]t
  supported_by_default = true
  attribute_mappings = {
    PRIMARY = "%[1]s"
  }
  canonical_values = [
    { label = "First", value = "E00001" },
  ]
  properties = {
    "custom.tf.marker" = "yes"
  }
}

resource "wso2is_external_claim" "scim" {
  dialect_uri            = "urn:scim:schemas:extension:custom:User"
  claim_uri              = "urn:scim:schemas:extension:custom:User:%[1]s"
  mapped_local_claim_uri = wso2is_claim.employee_id.claim_uri
}

resource "wso2is_external_claim" "oidc" {
  dialect_uri            = "http://wso2.org/oidc/claim"
  claim_uri              = "%[1]s"
  mapped_local_claim_uri = wso2is_claim.employee_id.claim_uri
}

resource "wso2is_oidc_scope" "employee" {
  name         = "%[1]s_scope"
  display_name = "Employee details"
  claims       = %[3]s
  depends_on   = [wso2is_external_claim.oidc]
}

resource "wso2is_user" "employee" {
  username = "%[1]suser"
  password = "Employee-Passw0rd!"
  email    = "%[1]s@example.com"
  attributes = {
    "urn:scim:schemas:extension:custom:User:%[1]s" = "E12345"
  }
  depends_on = [wso2is_external_claim.scim]
}
`, suffix, displayName, scopeClaims, required)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("Employee ID", fmt.Sprintf(`["%s", "email"]`, suffix), false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_claim.employee_id", "data_type", "string"),
					resource.TestCheckResourceAttr("wso2is_claim.employee_id", "attribute_mappings.PRIMARY", suffix),
					resource.TestCheckResourceAttr("wso2is_claim.employee_id", "properties.custom.tf.marker", "yes"),
					resource.TestCheckResourceAttr("wso2is_oidc_scope.employee", "claims.#", "2"),
					resource.TestCheckResourceAttr("wso2is_user.employee", "attributes.urn:scim:schemas:extension:custom:User:"+suffix, "E12345"),
				),
			},
			{
				Config: config("Employee number", fmt.Sprintf(`["%s"]`, suffix), true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("wso2is_claim.employee_id", plancheck.ResourceActionUpdate),
						plancheck.ExpectResourceAction("wso2is_oidc_scope.employee", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_claim.employee_id", "display_name", "Employee number"),
					resource.TestCheckResourceAttr("wso2is_claim.employee_id", "required", "true"),
					resource.TestCheckResourceAttr("wso2is_oidc_scope.employee", "claims.#", "1"),
				),
			},
			{
				ResourceName:            "wso2is_claim.employee_id",
				ImportState:             true,
				ImportStateId:           "http://wso2.org/claims/" + suffix,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"properties"},
			},
			{
				ResourceName:      "wso2is_external_claim.oidc",
				ImportState:       true,
				ImportStateId:     "http://wso2.org/oidc/claim|" + suffix,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "wso2is_oidc_scope.employee",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccClaimDialect(t *testing.T) {
	uri := "urn:" + acctest.RandomName("dialect")
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "wso2is_claim_dialect" "legacy" {
  dialect_uri = %q
}

resource "wso2is_external_claim" "mail" {
  dialect_uri            = wso2is_claim_dialect.legacy.dialect_uri
  claim_uri              = "mail"
  mapped_local_claim_uri = "http://wso2.org/claims/emailaddress"
}`, uri),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("wso2is_claim_dialect.legacy", "id"),
					resource.TestCheckResourceAttr("wso2is_external_claim.mail", "mapped_local_claim_uri", "http://wso2.org/claims/emailaddress"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "wso2is_claim_dialect" "legacy" {
  dialect_uri = %q
}

resource "wso2is_external_claim" "mail" {
  dialect_uri            = wso2is_claim_dialect.legacy.dialect_uri
  claim_uri              = "mail"
  mapped_local_claim_uri = "http://wso2.org/claims/username"
}`, uri),
				Check: resource.TestCheckResourceAttr("wso2is_external_claim.mail", "mapped_local_claim_uri", "http://wso2.org/claims/username"),
			},
			{
				ResourceName:      "wso2is_claim_dialect.legacy",
				ImportState:       true,
				ImportStateId:     uri,
				ImportStateVerify: true,
			},
		},
	})
}
