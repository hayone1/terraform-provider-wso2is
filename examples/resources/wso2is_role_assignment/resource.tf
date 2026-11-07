resource "wso2is_role" "support" {
  name = "support"
}

resource "wso2is_group" "support" {
  name = "support"
}

resource "wso2is_role_assignment" "support" {
  role_id   = wso2is_role.support.id
  group_ids = [wso2is_group.support.id]
}
