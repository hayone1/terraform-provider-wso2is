package tenant_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

func TestAccTenant(t *testing.T) {
	domain := strings.ToLower(acctest.RandomName("t")) + ".example"
	cfg := func(first string, activated bool) string {
		return fmt.Sprintf(`
resource "wso2is_tenant" "test" {
  domain           = %[1]q
  owner_username   = "admin"
  owner_password   = "Tenant-Admin-Passw0rd!"
  owner_email      = "admin@%[1]s"
  owner_first_name = %[2]q
  owner_last_name  = "Admin"
  activated        = %[3]t
}`, domain, first, activated)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: acctest.CheckDestroyed(t, "wso2is_tenant", func(ctx context.Context, c *client.Client, id string) (bool, error) {
			_, err := c.GetTenant(ctx, id)
			if client.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		}),
		Steps: []resource.TestStep{
			{
				Config: cfg("Tenant", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_tenant.test", "activated", "true"),
					resource.TestCheckResourceAttrSet("wso2is_tenant.test", "owner_id"),
					resource.TestCheckResourceAttr("wso2is_tenant.test", "name", domain),
				),
			},
			{
				Config: cfg("Renamed", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_tenant.test", "activated", "false"),
					resource.TestCheckResourceAttr("wso2is_tenant.test", "owner_first_name", "Renamed"),
				),
			},
			{
				ResourceName:            "wso2is_tenant.test",
				ImportState:             true,
				ImportStateId:           domain,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"owner_password"},
			},
		},
	})
}
