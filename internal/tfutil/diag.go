package tfutil

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

// ProviderData is what the provider hands to every resource and data source.
type ProviderData struct {
	Client *client.Client
}

// ConfigureResource extracts the provider data in a resource's Configure method.
func ConfigureResource(req resource.ConfigureRequest, resp *resource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *tfutil.ProviderData, got %T; please report this issue to the provider developers", req.ProviderData))
		return nil
	}
	return pd.Client
}

// ConfigureDataSource extracts the provider data in a data source's Configure method.
func ConfigureDataSource(req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) *client.Client {
	if req.ProviderData == nil {
		return nil
	}
	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *tfutil.ProviderData, got %T; please report this issue to the provider developers", req.ProviderData))
		return nil
	}
	return pd.Client
}

// AddAPIError reports a failed API call. action reads like "create application".
func AddAPIError(diags *diag.Diagnostics, action string, err error) {
	diags.AddError(fmt.Sprintf("Unable to %s", action), hint(err))
}

// AddAPIAttributeError reports a failed API call against one attribute.
func AddAPIAttributeError(diags *diag.Diagnostics, p path.Path, action string, err error) {
	diags.AddAttributeError(p, fmt.Sprintf("Unable to %s", action), hint(err))
}

// hint appends guidance for errors users commonly hit.
func hint(err error) string {
	msg := err.Error()
	switch client.StatusCode(err) {
	case 401:
		msg += "\n\nThe server rejected the provider credentials. Check username/password, client_id/client_secret or access_token."
	case 403:
		msg += "\n\nThe credentials are valid but lack permission for this API. For client credentials, authorize the application for the API (Console > Applications > <app> > API Authorization) and request the scopes (the provider requests SYSTEM by default)."
	}
	if strings.Contains(msg, "x509:") || strings.Contains(msg, "certificate") && strings.Contains(msg, "tls") {
		msg += "\n\nTLS verification failed. Set ca_certificate to the server's CA, or insecure_skip_verify = true for lab servers."
	}
	return msg
}
