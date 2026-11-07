package authenticator_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccCustomAuthenticator(t *testing.T) {
	name := "custom-" + acctest.RandomName("auth")
	cfg := func(display string, enabled bool) string {
		return fmt.Sprintf(`
resource "wso2is_custom_authenticator" "pin" {
  name                          = %[1]q
  display_name                  = %[2]q
  description                   = "Branch PIN pad"
  enabled                       = %[3]t
  endpoint_uri                  = "https://pin.example.com/authenticate"
  endpoint_authentication_type  = "API_KEY"
  endpoint_authentication_properties = {
    header = "x-api-key"
    value  = "pin-key"
  }
}

data "wso2is_authenticators" "local" {
  type       = "LOCAL"
  depends_on = [wso2is_custom_authenticator.pin]
}`, name, display, enabled)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Branch PIN", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_custom_authenticator.pin", "authentication_type", "IDENTIFICATION"),
					resource.TestCheckTypeSetElemNestedAttrs("data.wso2is_authenticators.local", "authenticators.*", map[string]string{"name": name, "defined_by": "USER"}),
					resource.TestCheckTypeSetElemNestedAttrs("data.wso2is_authenticators.local", "authenticators.*", map[string]string{"name": "BasicAuthenticator"}),
				),
			},
			{
				Config: cfg("Branch PIN pad", false),
				Check:  resource.TestCheckResourceAttr("wso2is_custom_authenticator.pin", "enabled", "false"),
			},
			{ResourceName: "wso2is_custom_authenticator.pin", ImportState: true, ImportStateId: name, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"endpoint_authentication_properties"}},
		},
	})
}
