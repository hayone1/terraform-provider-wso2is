package branding_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccBranding(t *testing.T) {
	name := acctest.RandomName("brand")
	cfg := func(color, heading string) string {
		return fmt.Sprintf(`
resource "wso2is_branding_preference" "org" {
  locale = "en-US"
  preference = jsonencode({
    configs = { isBrandingEnabled = true, removeDefaultBranding = false }
    organizationDetails = { displayName = "Acme", supportEmail = "support@acme.example" }
    theme = {
      activeTheme = "LIGHT"
      LIGHT = { colors = { primary = { main = %[2]q } } }
    }
    layout = { activeLayout = "centered" }
  })
}

resource "wso2is_application" "app" {
  name = %[1]q
}

resource "wso2is_branding_preference" "app" {
  type           = "APP"
  application_id = wso2is_application.app.id
  preference = jsonencode({
    configs = { isBrandingEnabled = true }
    theme   = { activeTheme = "DARK" }
  })
}

resource "wso2is_branding_text" "login" {
  screen = "login"
  text = {
    "login.heading" = %[3]q
    "login.button"  = "Continue"
  }
}`, name, color, heading)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("#FF7300", "Sign in to Acme"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_branding_preference.org", "type", "ORG"),
					resource.TestCheckResourceAttr("wso2is_branding_text.login", "text.login.heading", "Sign in to Acme"),
				),
			},
			{
				Config: cfg("#0055FF", "Welcome back"),
				Check:  resource.TestCheckResourceAttr("wso2is_branding_text.login", "text.login.heading", "Welcome back"),
			},
			{ResourceName: "wso2is_branding_preference.org", ImportState: true, ImportStateId: "ORG/carbon.super/en-US", ImportStateVerify: true},
			{ResourceName: "wso2is_branding_text.login", ImportState: true, ImportStateId: "ORG/carbon.super/en-US/login", ImportStateVerify: true},
		},
	})
}
