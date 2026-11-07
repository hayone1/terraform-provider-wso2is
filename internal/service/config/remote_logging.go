package config

import (
	"context"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*remoteLoggingResource)(nil)
	_ resource.ResourceWithImportState = (*remoteLoggingResource)(nil)
)

// NewRemoteLoggingResource returns the wso2is_remote_logging resource.
func NewRemoteLoggingResource() resource.Resource { return &remoteLoggingResource{} }

type remoteLoggingResource struct{ client *client.Client }

type remoteLoggingModel struct {
	ID                 types.String `tfsdk:"id"`
	LogType            types.String `tfsdk:"log_type"`
	RemoteURL          types.String `tfsdk:"remote_url"`
	ConnectTimeoutMs   types.String `tfsdk:"connect_timeout_millis"`
	VerifyHostname     types.Bool   `tfsdk:"verify_hostname"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	KeystoreLocation   types.String `tfsdk:"keystore_location"`
	KeystorePassword   types.String `tfsdk:"keystore_password"`
	TruststoreLocation types.String `tfsdk:"truststore_location"`
	TruststorePassword types.String `tfsdk:"truststore_password"`
}

func (r *remoteLoggingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_remote_logging"
}

func (r *remoteLoggingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sends server logs (audit logs or carbon logs) to a remote HTTP endpoint. This resource only works in the super tenant. When you destroy the resource, the server stops sending the logs. To import, use the log type.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `log_type`."),
			"log_type": schema.StringAttribute{
				MarkdownDescription: "`AUDIT` or `CARBON`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("AUDIT", "CARBON")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"remote_url":             schema.StringAttribute{MarkdownDescription: "Endpoint the logs are posted to.", Required: true},
			"connect_timeout_millis": schema.StringAttribute{MarkdownDescription: "Connection timeout in milliseconds.", Optional: true},
			"verify_hostname":        schema.BoolAttribute{MarkdownDescription: "Verify the endpoint's hostname. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"username":               schema.StringAttribute{MarkdownDescription: "Basic authentication username.", Optional: true},
			"password":               schema.StringAttribute{MarkdownDescription: "Basic authentication password (write-only on the server).", Optional: true, Sensitive: true},
			"keystore_location":      schema.StringAttribute{MarkdownDescription: "Path, on the server, of a keystore with a client certificate.", Optional: true},
			"keystore_password":      schema.StringAttribute{MarkdownDescription: "Keystore password (write-only on the server).", Optional: true, Sensitive: true},
			"truststore_location":    schema.StringAttribute{MarkdownDescription: "Path, on the server, of a truststore for the endpoint's certificate.", Optional: true},
			"truststore_password":    schema.StringAttribute{MarkdownDescription: "Truststore password (write-only on the server).", Optional: true, Sensitive: true},
		},
	}
}

func (r *remoteLoggingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *remoteLoggingResource) path(m *remoteLoggingModel) string {
	// The path takes the log type in lower case; the body must not carry logType.
	return r.client.ServerPath("/configs/remote-logging/" + url.PathEscape(strings.ToLower(m.LogType.ValueString())))
}

func (r *remoteLoggingResource) put(ctx context.Context, m *remoteLoggingModel, diags *diag.Diagnostics) {
	body := tfutil.JSON{}.Str("remoteUrl", m.RemoteURL).Str("connectTimeoutMillis", m.ConnectTimeoutMs).Bool("verifyHostname", m.VerifyHostname).
		Str("username", m.Username).Str("password", m.Password).Str("keystoreLocation", m.KeystoreLocation).Str("keystorePassword", m.KeystorePassword).
		Str("truststoreLocation", m.TruststoreLocation).Str("truststorePassword", m.TruststorePassword).Map()
	if _, err := r.client.Put(ctx, r.path(m), body, nil); err != nil {
		tfutil.AddAPIError(diags, "configure remote logging", err)
	}
}

func (r *remoteLoggingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan remoteLoggingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.LogType
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *remoteLoggingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state remoteLoggingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *remoteLoggingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan remoteLoggingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.LogType
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *remoteLoggingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state remoteLoggingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state)); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "remove remote logging", err)
	}
}

func (r *remoteLoggingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("log_type"), req.ID)...)
}

func (r *remoteLoggingResource) read(ctx context.Context, m *remoteLoggingModel, diags *diag.Diagnostics) bool {
	var c map[string]any
	if err := r.client.Get(ctx, r.path(m), nil, &c); err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read remote logging", err)
		return false
	}
	if tfutil.GetStringOrEmpty(c, "remoteUrl").ValueString() == "" {
		return false
	}
	importing := m.RemoteURL.IsNull()
	m.RemoteURL = tfutil.GetString(c, "remoteUrl")
	m.VerifyHostname = tfutil.GetBool(c, "verifyHostname", true)
	for _, f := range []struct {
		dst *types.String
		key string
	}{{&m.ConnectTimeoutMs, "connectTimeoutMillis"}, {&m.Username, "username"}, {&m.KeystoreLocation, "keystoreLocation"}, {&m.TruststoreLocation, "truststoreLocation"}} {
		v := tfutil.GetStringOrEmpty(c, f.key)
		if !f.dst.IsNull() || (importing && v.ValueString() != "") {
			*f.dst = v
		}
	}
	return true
}
