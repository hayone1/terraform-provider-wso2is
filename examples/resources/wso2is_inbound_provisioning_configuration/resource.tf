# Send every user created through /scim2 with basic authentication to one user store.
# While this is set, the server ignores the DOMAIN/ prefix of new users (including
# wso2is_user.user_store) and rejects SCIM updates of users in other user stores.
# Destroying the resource clears it.
resource "wso2is_inbound_provisioning_configuration" "this" {
  provisioning_userstore = "CUSTOMERS"
}
