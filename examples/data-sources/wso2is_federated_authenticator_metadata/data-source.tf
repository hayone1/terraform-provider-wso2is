data "wso2is_federated_authenticator_metadata" "google" {
  name = "GoogleOIDCAuthenticator"
}

output "google_properties" {
  value = { for p in data.wso2is_federated_authenticator_metadata.google.properties : p.key => p.mandatory }
}
