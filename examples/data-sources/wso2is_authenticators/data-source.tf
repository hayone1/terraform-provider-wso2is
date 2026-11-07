data "wso2is_authenticators" "local" {
  type = "LOCAL"
}

output "local_authenticators" {
  value = [for a in data.wso2is_authenticators.local.authenticators : a.name]
}
