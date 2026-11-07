package vc_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func TestAccVCTemplate(t *testing.T) {
	name := acctest.RandomName("vc")
	cfg := func(display, format, expires, extra string) string {
		return fmt.Sprintf(`
resource "wso2is_external_claim" "department_vc" {
  dialect_uri            = "http://wso2.org/vc/claim"
  claim_uri              = "%[1]s_department"
  mapped_local_claim_uri = "http://wso2.org/claims/department"
}

resource "wso2is_vc_template" "badge" {
  identifier   = %[1]q
  display_name = %[2]q
  format       = %[3]q
  expires_in   = %[4]s
  claims = {
    given_name      = "given_name"
    department      = wso2is_external_claim.department_vc.claim_uri
    %[5]s
  }
}
`, name, display, format, expires, extra)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: acctest.CheckDestroyed(t, "wso2is_vc_template", func(ctx context.Context, c *client.Client, id string) (bool, error) {
			var out any
			err := c.Get(ctx, c.ServerPath("/vc-templates/"+id), nil, &out)
			if client.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		}),
		Steps: []resource.TestStep{
			{
				Config: cfg("Employee badge", "jwt_vc_json", "31536000", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_vc_template.badge", "claims.%", "2"),
					resource.TestCheckResourceAttrSet("wso2is_vc_template.badge", "offer_id"),
				),
			},
			{
				Config: cfg("Employee badge (SD-JWT)", "dc+sd-jwt", "86400", `mail = "email"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_vc_template.badge", "format", "dc+sd-jwt"),
					resource.TestCheckResourceAttr("wso2is_vc_template.badge", "claims.mail", "email"),
					resource.TestCheckResourceAttr("wso2is_vc_template.badge", "expires_in", "86400"),
				),
			},
			{ResourceName: "wso2is_vc_template.badge", ImportState: true, ImportStateVerify: true},
		},
	})
}
