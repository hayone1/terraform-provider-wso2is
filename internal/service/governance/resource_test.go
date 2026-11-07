package governance_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

// Governance connectors and validation rules are tenant-wide singletons, so these tests
// must not run in parallel with tests relying on the defaults.
func TestAccGovernanceConnector(t *testing.T) {
	cfg := func(attempts string, extra string) string {
		return `
resource "wso2is_governance_connector" "lock" {
  name = "account.lock.handler"
  properties = {
    "account.lock.handler.lock.on.max.failed.attempts.enable" = "true"
    "account.lock.handler.On.Failure.Max.Attempts"            = "` + attempts + `"
    ` + extra + `
  }
}

resource "wso2is_governance_connector" "signup" {
  name = "self-sign-up"
  properties = {
    "SelfRegistration.Enable"                  = "true"
    "SelfRegistration.LockOnCreation"          = "true"
    "SelfRegistration.Notification.InternallyManage" = "true"
  }
}

data "wso2is_governance_connector" "lock" {
  name       = "account.lock.handler"
  depends_on = [wso2is_governance_connector.lock]
}
`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("3", `"account.lock.handler.Time" = "10"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_governance_connector.lock", "properties.%", "3"),
					resource.TestCheckResourceAttrSet("wso2is_governance_connector.lock", "category_id"),
					resource.TestCheckResourceAttr("data.wso2is_governance_connector.lock", "properties.account.lock.handler.On.Failure.Max.Attempts", "3"),
					resource.TestCheckResourceAttr("data.wso2is_governance_connector.lock", "properties.account.lock.handler.Time", "10"),
					resource.TestCheckResourceAttr("wso2is_governance_connector.signup", "properties.SelfRegistration.Enable", "true"),
				),
			},
			{
				// Removing a property reverts it to the default (5 minutes).
				Config: cfg("4", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_governance_connector.lock", "properties.%", "2"),
					resource.TestCheckResourceAttr("data.wso2is_governance_connector.lock", "properties.account.lock.handler.On.Failure.Max.Attempts", "4"),
					resource.TestCheckResourceAttr("data.wso2is_governance_connector.lock", "properties.account.lock.handler.Time", "5"),
				),
			},
			{
				ResourceName:            "wso2is_governance_connector.lock",
				ImportState:             true,
				ImportStateId:           "account.lock.handler",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"properties"},
			},
		},
	})
}

func TestAccValidationRules(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
resource "wso2is_validation_rules" "password" {
  field = "password"
  rules = [
    { validator = "LengthValidator", properties = { "min.length" = "12", "max.length" = "64" } },
    { validator = "NumeralValidator", properties = { "min.length" = "1" } },
    { validator = "UpperCaseValidator", properties = { "min.length" = "1" } },
    { validator = "SpecialCharacterValidator", properties = { "min.length" = "1" } },
  ]
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_validation_rules.password", "rules.#", "4"),
					resource.TestCheckTypeSetElemNestedAttrs("wso2is_validation_rules.password", "rules.*", map[string]string{"validator": "LengthValidator", "properties.min.length": "12"}),
				),
			},
			{
				Config: `
resource "wso2is_validation_rules" "password" {
  field = "password"
  regex = "^(?=.*[A-Z])(?=.*[0-9]).{10,64}$"
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_validation_rules.password", "regex", "^(?=.*[A-Z])(?=.*[0-9]).{10,64}$"),
					resource.TestCheckNoResourceAttr("wso2is_validation_rules.password", "rules"),
				),
			},
			{
				ResourceName:      "wso2is_validation_rules.password",
				ImportState:       true,
				ImportStateId:     "password",
				ImportStateVerify: true,
			},
		},
	})
}
