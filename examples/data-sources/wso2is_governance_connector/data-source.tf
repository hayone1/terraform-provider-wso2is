data "wso2is_governance_connector" "password_recovery" {
  name = "account-recovery"
}

output "recovery_properties" {
  value = data.wso2is_governance_connector.password_recovery.properties
}
