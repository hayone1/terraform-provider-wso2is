// Package authenticator implements user-defined (custom) local authenticators and the
// authenticator listing data source.
package authenticator

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*customResource)(nil)
	_ resource.ResourceWithImportState = (*customResource)(nil)
)

// NewCustomResource returns the wso2is_custom_authenticator resource.
func NewCustomResource() resource.Resource { return &customResource{} }

type customResource struct{ client *client.Client }

type customModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	DisplayName            types.String `tfsdk:"display_name"`
	Description            types.String `tfsdk:"description"`
	ImageURL               types.String `tfsdk:"image_url"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	AuthenticationType     types.String `tfsdk:"authentication_type"`
	EndpointURI            types.String `tfsdk:"endpoint_uri"`
	EndpointAuthType       types.String `tfsdk:"endpoint_authentication_type"`
	EndpointAuthProperties types.Map    `tfsdk:"endpoint_authentication_properties"`
}

func (r *customResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_authenticator"
}

func (r *customResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom local authenticator (WSO2 IS 7.1 and later). A custom authenticator is a login step that your own HTTPS service performs, and that WSO2 calls during login. To use it, add it to an application's `authentication_sequence` with `idp = \"LOCAL\"` and `authenticator = <name>`. Custom federated authenticators work differently: you set them up on `wso2is_identity_provider` with an endpoint. To import, use the authenticator name.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Authenticator ID (base64url of the name)."),
			"name": schema.StringAttribute{
				MarkdownDescription: "The authenticator name. It must start with `custom-`. Changing the name replaces the authenticator.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(mustRegexp(`^custom-[A-Za-z0-9_-]+$`), "must start with custom- and contain only letters, digits, - and _")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{MarkdownDescription: "Name shown on the login page.", Required: true},
			"description":  schema.StringAttribute{MarkdownDescription: "Description.", Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"image_url":    schema.StringAttribute{MarkdownDescription: "Logo URL.", Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"enabled":      schema.BoolAttribute{MarkdownDescription: "Whether the authenticator can be used. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"authentication_type": schema.StringAttribute{
				MarkdownDescription: "`IDENTIFICATION` (the service identifies the user, for a first step) or `VERIFICATION` (it verifies an already identified user, for later steps). Changing it replaces the authenticator.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("IDENTIFICATION"),
				Validators:          []validator.String{stringvalidator.OneOf("IDENTIFICATION", "VERIFICATION")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"endpoint_uri": schema.StringAttribute{MarkdownDescription: "HTTPS endpoint of the authentication service.", Required: true},
			"endpoint_authentication_type": schema.StringAttribute{
				MarkdownDescription: "How WSO2 signs in to the service. The allowed values are `NONE`, `BASIC`, `BEARER` and `API_KEY`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("NONE", "BASIC", "BEARER", "API_KEY")},
			},
			"endpoint_authentication_properties": schema.MapAttribute{
				MarkdownDescription: "The credentials for the service. Use `username` and `password`, `accessToken`, or `header` and `value`, to match `endpoint_authentication_type`. The server never returns these values.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *customResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func endpoint(ctx context.Context, m *customModel, diags *diag.Diagnostics) map[string]any {
	props := map[string]any{}
	for k, v := range tfutil.MapOfStrings(ctx, m.EndpointAuthProperties, diags) {
		props[k] = v
	}
	return map[string]any{"uri": m.EndpointURI.ValueString(), "authentication": map[string]any{"type": m.EndpointAuthType.ValueString(), "properties": props}}
}

func (r *customResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan customModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"name": plan.Name.ValueString(), "displayName": plan.DisplayName.ValueString(), "isEnabled": plan.Enabled.ValueBool(),
		"authenticationType": plan.AuthenticationType.ValueString(), "description": plan.Description.ValueString(),
		"endpoint": endpoint(ctx, &plan, &resp.Diagnostics),
	}
	if v := plan.ImageURL.ValueString(); v != "" {
		body["image"] = v
	}
	var out map[string]any
	if _, err := r.client.Post(ctx, r.client.ServerPath("/authenticators/custom"), body, &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create custom authenticator", err)
		return
	}
	id, _ := out["id"].(string)
	if id == "" {
		id = client.AuthenticatorID(plan.Name.ValueString())
	}
	plan.ID = types.StringValue(id)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state customModel
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

func (r *customResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state customModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	body := map[string]any{
		"displayName": plan.DisplayName.ValueString(), "isEnabled": plan.Enabled.ValueBool(), "description": plan.Description.ValueString(),
		"endpoint": endpoint(ctx, &plan, &resp.Diagnostics),
	}
	// An empty image fails URL validation.
	if v := plan.ImageURL.ValueString(); v != "" {
		body["image"] = v
	}
	if _, err := r.client.Put(ctx, r.client.ServerPath("/authenticators/custom/"+url.PathEscape(state.ID.ValueString())), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update custom authenticator", err)
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *customResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state customModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.client.ServerPath("/authenticators/custom/"+url.PathEscape(state.ID.ValueString()))); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete custom authenticator", err)
	}
}

func (r *customResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), client.AuthenticatorID(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("authentication_type"), "IDENTIFICATION")...)
}

func (r *customResource) read(ctx context.Context, m *customModel, diags *diag.Diagnostics) bool {
	var a map[string]any
	if err := r.client.Get(ctx, r.client.ServerPath("/configs/authenticators/"+url.PathEscape(m.ID.ValueString())), nil, &a); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "CNF-60004", "AUT-60001") {
			return false
		}
		tfutil.AddAPIError(diags, "read custom authenticator", err)
		return false
	}
	m.Name = tfutil.GetString(a, "name")
	m.DisplayName = tfutil.GetString(a, "displayName")
	m.Description = tfutil.GetStringOrEmpty(a, "description")
	m.ImageURL = tfutil.GetStringOrEmpty(a, "image")
	m.Enabled = tfutil.GetBool(a, "isEnabled", true)
	ep := tfutil.GetMap(a, "endpoint")
	m.EndpointURI = tfutil.GetString(ep, "uri")
	m.EndpointAuthType = tfutil.GetString(tfutil.GetMap(ep, "authentication"), "type")
	return true
}
