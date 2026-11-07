// Package provider makes the WSO2 Identity Server Terraform provider available to other Go
// programs. Use it to run the provider inside another process, for example in a Crossplane
// provider that Upjet generates:
//
//	ujconfig.WithTerraformPluginFrameworkProvider(provider.New(version)())
//
// Terraform itself does not need this package: it runs the provider binary.
//
// The provider keeps a few things per process, so that many short-lived provider
// instances behave well:
//
//   - Access tokens from the client credentials and organization switch grants are cached
//     and shared by every instance that uses the same server, tenant, organization, client
//     and scopes. A new instance does not request a new token while the cached one is valid.
//   - HTTP connections are shared by every instance with the same TLS settings.
//   - Writes to tenant-wide settings are serialized per server, tenant and organization.
//     Run only one process that manages the same tenant, or these writes can overwrite each
//     other.
package provider

import (
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"

	internal "github.com/hayone1/terraform-provider-wso2is/internal/provider"
)

// Address is the provider's address in the Terraform and OpenTofu registries.
const Address = "registry.terraform.io/hayone1/wso2is"

// New returns a function that creates the provider. version is reported to the server in
// the User-Agent header.
func New(version string) func() fwprovider.Provider {
	return internal.New(version)
}
