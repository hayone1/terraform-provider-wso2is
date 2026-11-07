data "wso2is_role" "admin" {
  name = "admin"
}

resource "wso2is_workflow" "approvals" {
  name  = "Administrator approval"
  steps = [{ role_ids = [data.wso2is_role.admin.id] }]
}

resource "wso2is_workflow_association" "deletions" {
  name        = "User deletions need approval"
  operation   = "DELETE_USER"
  workflow_id = wso2is_workflow.approvals.id
}
