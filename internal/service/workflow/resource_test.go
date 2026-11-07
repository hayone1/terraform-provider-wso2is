package workflow_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccWorkflow(t *testing.T) {
	name := acctest.RandomName("wf")
	cfg := func(desc string, twoSteps, enabled bool) string {
		steps := `[{ role_ids = [wso2is_role.approvers.id] }]`
		if twoSteps {
			steps = `[{ role_ids = [wso2is_role.approvers.id] }, { role_ids = [wso2is_role.approvers.id] }]`
		}
		return fmt.Sprintf(`
resource "wso2is_role" "approvers" {
  name = "%[1]s-approvers"
}

resource "wso2is_workflow" "role_changes" {
  name        = %[1]q
  description = %[2]q
  steps       = %[3]s
  initiator_notifications = {
    channels = ["email"]
    events   = ["onApproval", "onRejection"]
  }
}

resource "wso2is_workflow_association" "new_roles" {
  name        = "%[1]s-new-roles"
  operation   = "ADD_ROLE"
  workflow_id = wso2is_workflow.role_changes.id
  enabled     = %[4]t
}`, name, desc, steps, enabled)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Role changes need approval", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_workflow.role_changes", "steps.#", "1"),
					resource.TestCheckResourceAttr("wso2is_workflow.role_changes", "engine", "WorkflowEngine"),
					resource.TestCheckResourceAttr("wso2is_workflow_association.new_roles", "operation", "ADD_ROLE"),
				),
			},
			{
				Config: cfg("Two approvals", true, false),
				Check:  resource.TestCheckResourceAttr("wso2is_workflow.role_changes", "steps.#", "2"),
			},
			{ResourceName: "wso2is_workflow.role_changes", ImportState: true, ImportStateVerify: true},
			{ResourceName: "wso2is_workflow_association.new_roles", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"workflow_id"}},
		},
	})
}
