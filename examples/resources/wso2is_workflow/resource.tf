data "wso2is_role" "admin" {
  name = "admin"
}

resource "wso2is_role" "security_officers" {
  name = "security-officers"
}

# Two approval steps: an administrator, then a security officer.
resource "wso2is_workflow" "role_changes" {
  name        = "Role changes"
  description = "Changes to roles need two approvals"
  steps = [
    { role_ids = [data.wso2is_role.admin.id] },
    { role_ids = [wso2is_role.security_officers.id] },
  ]
}
