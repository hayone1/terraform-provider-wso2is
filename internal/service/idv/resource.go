// Package idv implements identity verification providers.
package idv

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*idvResource)(nil)
	_ resource.ResourceWithImportState = (*idvResource)(nil)
)

// NewResource returns the wso2is_identity_verification_provider resource.
func NewResource() resource.Resource { return &idvResource{} }

type idvResource struct{ client *client.Client }

type model struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	Description      types.String `tfsdk:"description"`
	ImageURL         types.String `tfsdk:"image_url"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	Properties       types.Map    `tfsdk:"properties"`
	SecretProperties types.Map    `tfsdk:"secret_properties"`
	ClaimMappings    types.Map    `tfsdk:"claim_mappings"`
}

func (r *idvResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_verification_provider"
}

func (r *idvResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an identity verification provider, such as Onfido. The server uses it to check users' identity documents. To import, use the provider's ID.",
		Attributes: map[string]schema.Attribute{
			"id":   tfutil.IDAttribute("Provider ID."),
			"name": schema.StringAttribute{MarkdownDescription: "Provider name.", Required: true},
			"type": schema.StringAttribute{
				MarkdownDescription: "Provider type, for example `ONFIDO` (the connector must be installed). Changing it replaces the provider.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{MarkdownDescription: "Description.", Optional: true, Computed: true, Default: stringdefault.StaticString("")},
			"image_url":   tfutil.OCString("Logo URL. The server sets a default for known types."),
			"enabled":     schema.BoolAttribute{MarkdownDescription: "Whether the provider is enabled. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"properties": schema.MapAttribute{
				MarkdownDescription: "Configuration properties, for example `base_url` and `workflow_id` for Onfido.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"secret_properties": schema.MapAttribute{
				MarkdownDescription: "Confidential configuration properties, for example `token` and `webhook_token` for Onfido.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
			"claim_mappings": schema.MapAttribute{
				MarkdownDescription: "Provider claim by local claim URI, for example `{ \"http://wso2.org/claims/givenname\" = \"first_name\" }`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *idvResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func body(ctx context.Context, m *model, diags *diag.Diagnostics) map[string]any {
	props := []map[string]any{}
	plain := tfutil.MapOfStrings(ctx, m.Properties, diags)
	for _, k := range tfutil.SortedKeys(plain) {
		props = append(props, map[string]any{"key": k, "value": plain[k], "isSecret": false})
	}
	secret := tfutil.MapOfStrings(ctx, m.SecretProperties, diags)
	for _, k := range tfutil.SortedKeys(secret) {
		props = append(props, map[string]any{"key": k, "value": secret[k], "isSecret": true})
	}
	claims := []map[string]string{}
	cm := tfutil.MapOfStrings(ctx, m.ClaimMappings, diags)
	for _, k := range tfutil.SortedKeys(cm) {
		claims = append(claims, map[string]string{"localClaim": k, "idvpClaim": cm[k]})
	}
	b := map[string]any{"name": m.Name.ValueString(), "type": m.Type.ValueString(), "description": m.Description.ValueString(),
		"isEnabled": m.Enabled.ValueBool(), "configProperties": props, "claims": claims}
	if tfutil.Known(m.ImageURL) && m.ImageURL.ValueString() != "" {
		b["image"] = m.ImageURL.ValueString()
	}
	return b
}

func (r *idvResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if _, err := r.client.Post(ctx, r.client.ServerPath("/idv-providers"), body(ctx, &plan, &resp.Diagnostics), &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create identity verification provider", err)
		return
	}
	id, _ := out["id"].(string)
	plan.ID = types.StringValue(id)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *idvResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tfutil.MissingID(state.ID) {
		resp.State.RemoveResource(ctx)
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

func (r *idvResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if _, err := r.client.Put(ctx, r.client.ServerPath("/idv-providers/"+url.PathEscape(state.ID.ValueString())), body(ctx, &plan, &resp.Diagnostics), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update identity verification provider", err)
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *idvResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.client.ServerPath("/idv-providers/"+url.PathEscape(state.ID.ValueString()))); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete identity verification provider", err)
	}
}

func (r *idvResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *idvResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	var p map[string]any
	if err := r.client.Get(ctx, r.client.ServerPath("/idv-providers/"+url.PathEscape(m.ID.ValueString())), nil, &p); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "IDVP-60004") {
			return false
		}
		tfutil.AddAPIError(diags, "read identity verification provider", err)
		return false
	}
	importing := m.Name.IsNull()
	m.Name = tfutil.GetString(p, "name")
	m.Type = tfutil.GetString(p, "type")
	m.Description = tfutil.GetStringOrEmpty(p, "description")
	m.ImageURL = tfutil.GetStringOrEmpty(p, "image")
	m.Enabled = tfutil.GetBool(p, "isEnabled", true)
	plain, secret := map[string]string{}, map[string]string{}
	for _, cp := range tfutil.GetObjects(p, "configProperties") {
		k, _ := cp["key"].(string)
		v, _ := cp["value"].(string)
		if isSecret, _ := cp["isSecret"].(bool); isSecret {
			if pv, ok := m.SecretProperties.Elements()[k].(types.String); ok && (v == "" || v == "******") {
				v = pv.ValueString()
			}
			secret[k] = v
		} else {
			plain[k] = v
		}
	}
	m.Properties = tfutil.MapValueOfStrings(plain, m.Properties.IsNull() && (!importing || len(plain) == 0))
	m.SecretProperties = tfutil.MapValueOfStrings(secret, m.SecretProperties.IsNull())
	cm := map[string]string{}
	for _, c := range tfutil.GetObjects(p, "claims") {
		k, _ := c["localClaim"].(string)
		v, _ := c["idvpClaim"].(string)
		cm[k] = v
	}
	m.ClaimMappings = tfutil.MapValueOfStrings(cm, m.ClaimMappings.IsNull() && (!importing || len(cm) == 0))
	return true
}
