# Read a flow designed in the Console, to copy it into a file you manage with Terraform:
#   terraform output -raw sign_up_flow > sign-up-flow.json
# then use steps = file("sign-up-flow.json") in a wso2is_flow resource.
data "wso2is_flow" "sign_up" {
  flow_type = "REGISTRATION"
}

output "sign_up_flow" {
  value = data.wso2is_flow.sign_up.steps
}
