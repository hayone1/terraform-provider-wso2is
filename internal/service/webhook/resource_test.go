package webhook_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccWebhook(t *testing.T) {
	name := acctest.RandomName("hook")
	cfg := func(channels string, active bool) string {
		return fmt.Sprintf(`
resource "wso2is_webhook" "events" {
  name     = %[1]q
  endpoint = "https://hooks.example.com/%[1]s"
  secret   = "signing-secret"
  channels = %[2]s
  active   = %[3]t
}`, name, channels, active)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(`["https://schemas.identity.wso2.org/events/login", "https://schemas.identity.wso2.org/events/registration"]`, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_webhook.events", "channels.#", "2"),
					resource.TestCheckResourceAttr("wso2is_webhook.events", "active", "true"),
				),
			},
			{
				Config: cfg(`["https://schemas.identity.wso2.org/events/user"]`, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_webhook.events", "channels.#", "1"),
					resource.TestCheckResourceAttr("wso2is_webhook.events", "active", "false"),
				),
			},
			{ResourceName: "wso2is_webhook.events", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"secret"}},
		},
	})
}
