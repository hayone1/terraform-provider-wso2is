resource "wso2is_notification_template_type" "contract_ready" {
  channel = "email"
  name    = "ContractReady"
}

resource "wso2is_notification_template" "contract_ready" {
  channel = "email"
  type    = wso2is_notification_template_type.contract_ready.name
  subject = "Your contract is ready"
  body    = "<p>Hi {{user-name}}, your contract is ready to sign.</p>"
}
