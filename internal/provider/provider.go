// Package provider implements the WSO2 Identity Server Terraform provider.
package provider

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var _ provider.Provider = (*wso2isProvider)(nil)

// New returns a provider factory for the given version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &wso2isProvider{version: version}
	}
}

type wso2isProvider struct {
	version string
}

type providerModel struct {
	BaseURL            types.String `tfsdk:"base_url"`
	TenantDomain       types.String `tfsdk:"tenant_domain"`
	OrganizationID     types.String `tfsdk:"organization_id"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	ClientID           types.String `tfsdk:"client_id"`
	ClientSecret       types.String `tfsdk:"client_secret"`
	Scopes             types.List   `tfsdk:"scopes"`
	AccessToken        types.String `tfsdk:"access_token"`
	InsecureSkipVerify types.Bool   `tfsdk:"insecure_skip_verify"`
	CACertificate      types.String `tfsdk:"ca_certificate"`
	RequestTimeout     types.Int64  `tfsdk:"request_timeout"`
	MaxRetries         types.Int64  `tfsdk:"max_retries"`
}

func (p *wso2isProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "wso2is"
	resp.Version = p.version
}

func (p *wso2isProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages [WSO2 Identity Server](https://wso2.com/identity-server/) 7.x through its management REST APIs.\n\n" +
			"Authenticate with one of: `username`/`password` (basic authentication), `client_id`/`client_secret` " +
			"(OAuth2 client credentials of an application authorized for the management APIs), or a ready `access_token`. " +
			"Credentials set in the provider block take precedence, as a group, over the `WSO2IS_*` environment variables.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Origin of the server, for example `https://localhost:9443` or `https://id.example.com`. Do not include a path. Environment variable: `WSO2IS_BASE_URL`.",
				Optional:            true,
			},
			"tenant_domain": schema.StringAttribute{
				MarkdownDescription: "Tenant (root organization) to manage. The default is `carbon.super`. Environment variable: `WSO2IS_TENANT_DOMAIN`.",
				Optional:            true,
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "ID of a sub-organization (B2B) to manage instead of the root organization. Requests go to the organization-scoped (`/o/`) APIs. " +
					"It needs the `client_id` and `client_secret` of an application that can use the `organization_switch` grant, or an `access_token` issued for the organization. " +
					"Use a separate provider alias per organization; the ID may come from a `wso2is_organization` in the same configuration. Environment variable: `WSO2IS_ORGANIZATION_ID`.",
				Optional: true,
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username for basic authentication (for example the super admin). Environment variable: `WSO2IS_USERNAME`.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for basic authentication. Environment variable: `WSO2IS_PASSWORD`.",
				Optional:            true,
				Sensitive:           true,
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Client ID of an OAuth2 application (usually an M2M application) authorized for the management APIs. Environment variable: `WSO2IS_CLIENT_ID`.",
				Optional:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Client secret for `client_id`. Environment variable: `WSO2IS_CLIENT_SECRET`.",
				Optional:            true,
				Sensitive:           true,
			},
			"scopes": schema.ListAttribute{
				MarkdownDescription: "Scopes requested with the client credentials grant. The default is `[\"SYSTEM\"]`, which WSO2 expands to every internal scope the application is authorized for. Environment variable: `WSO2IS_SCOPES` (space separated).",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"access_token": schema.StringAttribute{
				MarkdownDescription: "A bearer token to use as-is (it is not refreshed). Environment variable: `WSO2IS_ACCESS_TOKEN`.",
				Optional:            true,
				Sensitive:           true,
			},
			"insecure_skip_verify": schema.BoolAttribute{
				MarkdownDescription: "Skip TLS certificate verification. Use it only for test servers that use the default self-signed certificate. Environment variable: `WSO2IS_INSECURE_SKIP_VERIFY`.",
				Optional:            true,
			},
			"ca_certificate": schema.StringAttribute{
				MarkdownDescription: "PEM encoded CA certificate(s) to trust in addition to the system roots. Environment variable: `WSO2IS_CA_CERTIFICATE` (PEM content or a file path).",
				Optional:            true,
			},
			"request_timeout": schema.Int64Attribute{
				MarkdownDescription: "Timeout of a single HTTP request in seconds. The default is `60`. Environment variable: `WSO2IS_REQUEST_TIMEOUT`.",
				Optional:            true,
			},
			"max_retries": schema.Int64Attribute{
				MarkdownDescription: "Retries for transient failures (HTTP 429/502/503/504 and connection errors). The default is `4`. Environment variable: `WSO2IS_MAX_RETRIES`.",
				Optional:            true,
			},
		},
	}
}

func (p *wso2isProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var m providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var unknown []string
	for name, v := range map[string]interface{ IsUnknown() bool }{
		"base_url": m.BaseURL, "tenant_domain": m.TenantDomain, "organization_id": m.OrganizationID,
		"username": m.Username, "password": m.Password, "client_id": m.ClientID, "client_secret": m.ClientSecret,
		"access_token": m.AccessToken, "scopes": m.Scopes, "ca_certificate": m.CACertificate,
		"insecure_skip_verify": m.InsecureSkipVerify, "request_timeout": m.RequestTimeout, "max_retries": m.MaxRetries,
	} {
		if v.IsUnknown() {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		// For example organization_id = wso2is_organization.x.id: new resources can be
		// planned without the server, and Terraform configures the provider again with the
		// real values before applying them.
		sort.Strings(unknown)
		data := &tfutil.ProviderData{Client: client.NewUnavailable(fmt.Sprintf(
			"the provider configuration (%s) depends on values that are known only after apply, so the server cannot be read during this plan. "+
				"This happens when the resource or data source already exists or is a data source, and the provider's inputs are being created or replaced in the same run: "+
				"apply the resources the provider configuration refers to first (terraform apply -target=...), or move them to a separate configuration",
			strings.Join(unknown, ", ")))}
		resp.ResourceData = data
		resp.DataSourceData = data
		return
	}

	cfg := client.Config{
		BaseURL:        stringOrEnv(m.BaseURL, "WSO2IS_BASE_URL"),
		TenantDomain:   stringOrEnv(m.TenantDomain, "WSO2IS_TENANT_DOMAIN"),
		OrganizationID: stringOrEnv(m.OrganizationID, "WSO2IS_ORGANIZATION_ID"),
		UserAgent:      fmt.Sprintf("terraform-provider-wso2is/%s (+https://registry.terraform.io/providers/hayone1/wso2is) Terraform/%s", p.version, req.TerraformVersion),
	}
	// Credentials set in the configuration take precedence, as a group, over the
	// environment: a provider block with client_id/client_secret ignores WSO2IS_USERNAME.
	if anyConfigured(m.Username, m.Password, m.ClientID, m.ClientSecret, m.AccessToken) {
		cfg.Username, cfg.Password = m.Username.ValueString(), m.Password.ValueString()
		cfg.ClientID, cfg.ClientSecret = m.ClientID.ValueString(), m.ClientSecret.ValueString()
		cfg.AccessToken = m.AccessToken.ValueString()
	} else {
		cfg.Username, cfg.Password = os.Getenv("WSO2IS_USERNAME"), os.Getenv("WSO2IS_PASSWORD")
		cfg.ClientID, cfg.ClientSecret = os.Getenv("WSO2IS_CLIENT_ID"), os.Getenv("WSO2IS_CLIENT_SECRET")
		cfg.AccessToken = os.Getenv("WSO2IS_ACCESS_TOKEN")
	}

	if !m.Scopes.IsNull() {
		resp.Diagnostics.Append(m.Scopes.ElementsAs(ctx, &cfg.Scopes, false)...)
	} else if s := os.Getenv("WSO2IS_SCOPES"); s != "" {
		cfg.Scopes = strings.Fields(s)
	}

	var err error
	if cfg.InsecureSkipVerify, err = boolOrEnv(m.InsecureSkipVerify, "WSO2IS_INSECURE_SKIP_VERIFY", false); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("insecure_skip_verify"), "Invalid WSO2IS_INSECURE_SKIP_VERIFY", err.Error())
	}
	timeout, err := int64OrEnv(m.RequestTimeout, "WSO2IS_REQUEST_TIMEOUT", 60)
	if err != nil || timeout <= 0 {
		resp.Diagnostics.AddAttributeError(path.Root("request_timeout"), "Invalid request timeout", "request_timeout must be a positive number of seconds")
	}
	cfg.Timeout = time.Duration(timeout) * time.Second
	retries, err := int64OrEnv(m.MaxRetries, "WSO2IS_MAX_RETRIES", 4)
	if err != nil || retries < 0 {
		resp.Diagnostics.AddAttributeError(path.Root("max_retries"), "Invalid max_retries", "max_retries must be zero or a positive number")
	}
	cfg.MaxRetries = int(retries)

	ca := stringOrEnv(m.CACertificate, "WSO2IS_CA_CERTIFICATE")
	if ca != "" && !strings.Contains(ca, "-----BEGIN") {
		b, err := os.ReadFile(ca)
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("ca_certificate"), "Unable to read CA certificate", fmt.Sprintf("ca_certificate is neither PEM content nor a readable file: %s", err))
		}
		ca = string(b)
	}
	cfg.CACertPEM = ca

	if cfg.BaseURL == "" {
		resp.Diagnostics.AddAttributeError(path.Root("base_url"), "Missing base URL", "Set base_url or the WSO2IS_BASE_URL environment variable.")
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(cfg)
	if err != nil {
		resp.Diagnostics.AddError("Invalid provider configuration", err.Error())
		return
	}
	tflog.Debug(ctx, "configured WSO2 Identity Server client", map[string]any{"base_url": c.BaseURL(), "tenant_domain": c.TenantDomain(), "organization_id": c.OrganizationID()})

	data := &tfutil.ProviderData{Client: c}
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (p *wso2isProvider) Resources(_ context.Context) []func() resource.Resource {
	return allResources()
}

func (p *wso2isProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return allDataSources()
}

func stringOrEnv(v types.String, env string) string {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueString()
	}
	return os.Getenv(env)
}

func boolOrEnv(v types.Bool, env string, def bool) (bool, error) {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool(), nil
	}
	if s := os.Getenv(env); s != "" {
		return strconv.ParseBool(s)
	}
	return def, nil
}

func int64OrEnv(v types.Int64, env string, def int64) (int64, error) {
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueInt64(), nil
	}
	if s := os.Getenv(env); s != "" {
		return strconv.ParseInt(s, 10, 64)
	}
	return def, nil
}

func anyConfigured(vs ...types.String) bool {
	for _, v := range vs {
		if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
			return true
		}
	}
	return false
}
