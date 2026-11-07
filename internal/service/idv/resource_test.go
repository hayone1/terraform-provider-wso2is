package idv_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccIdentityVerificationProvider(t *testing.T) {
	name := acctest.RandomName("idv")
	cfg := func(workflow string, enabled bool) string {
		return fmt.Sprintf(`
resource "wso2is_identity_verification_provider" "onfido" {
  name    = %[1]q
  type    = "ONFIDO"
  enabled = %[3]t
  properties = {
    base_url    = "https://api.eu.onfido.com/v3.6"
    workflow_id = %[2]q
  }
  secret_properties = {
    token         = "api-token"
    webhook_token = "webhook-token"
  }
  claim_mappings = {
    "http://wso2.org/claims/givenname" = "first_name"
    "http://wso2.org/claims/lastname"  = "last_name"
  }
}`, name, workflow, enabled)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("wf-1", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_identity_verification_provider.onfido", "claim_mappings.%", "2"),
					resource.TestCheckResourceAttrSet("wso2is_identity_verification_provider.onfido", "image_url"),
				),
			},
			{
				Config: cfg("wf-2", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_identity_verification_provider.onfido", "properties.workflow_id", "wf-2"),
					resource.TestCheckResourceAttr("wso2is_identity_verification_provider.onfido", "enabled", "false"),
				),
			},
			{ResourceName: "wso2is_identity_verification_provider.onfido", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"secret_properties"}},
		},
	})
}
