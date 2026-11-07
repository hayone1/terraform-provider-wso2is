// Package vc implements OpenID for Verifiable Credential Issuance (OID4VCI) credential
// templates.
package vc

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*templateResource)(nil)
	_ resource.ResourceWithImportState = (*templateResource)(nil)
)

// NewTemplateResource returns the wso2is_vc_template resource.
func NewTemplateResource() resource.Resource { return &templateResource{} }

type templateResource struct{ client *client.Client }

type templateModel struct {
	ID          types.String `tfsdk:"id"`
	Identifier  types.String `tfsdk:"identifier"`
	DisplayName types.String `tfsdk:"display_name"`
	Format      types.String `tfsdk:"format"`
	Claims      types.Map    `tfsdk:"claims"`
	ExpiresIn   types.Int64  `tfsdk:"expires_in"`
	OfferID     types.String `tfsdk:"offer_id"`
}

type vcClaim struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	ClaimURI string `json:"claimUri"`
}

type vcTemplate struct {
	ID          string    `json:"id,omitempty"`
	Identifier  string    `json:"identifier,omitempty"`
	DisplayName string    `json:"displayName"`
	Format      string    `json:"format"`
	Claims      []vcClaim `json:"claims"`
	ExpiresIn   int64     `json:"expiresIn"`
	OfferID     *string   `json:"offerId,omitempty"`
}

func (r *templateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vc_template"
}

func (r *templateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a verifiable credential template (WSO2 IS 7.3 and later, OpenID for Verifiable Credential Issuance). The template describes a credential that digital wallets can request: its format, the user attributes that it carries and how long it is valid. The server creates a credential offer for the template. To import, use the template ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Template ID."),
			"identifier": schema.StringAttribute{
				MarkdownDescription: "Stable identifier of the credential type (published as `credential_configuration_id`), for example `EmployeeBadge`. Changing it replaces the template.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{MarkdownDescription: "Name shown to users and wallets.", Required: true},
			"format": schema.StringAttribute{
				MarkdownDescription: "Credential format: `jwt_vc_json` (default) or `dc+sd-jwt` (selective disclosure).",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("jwt_vc_json"),
				Validators:          []validator.String{stringvalidator.OneOf("jwt_vc_json", "dc+sd-jwt")},
			},
			"claims": schema.MapAttribute{
				MarkdownDescription: "The claims in the credential. Each key is a claim name in the credential. Each value is a claim of the `http://wso2.org/vc/claim` dialect, for example `{ given_name = \"given_name\", mail = \"email\" }`. To use a custom attribute, first map it into that dialect with `wso2is_external_claim`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Map{mapvalidator.SizeAtLeast(1)},
			},
			"expires_in": schema.Int64Attribute{
				MarkdownDescription: "Lifetime of issued credentials in seconds (at least 60).",
				Required:            true,
				Validators:          []validator.Int64{int64validator.AtLeast(60)},
			},
			"offer_id": schema.StringAttribute{
				MarkdownDescription: "ID of the credential offer the server generated for the template.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString()},
			},
		},
	}
}

func (r *templateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *templateResource) templatePath(id string) string {
	return r.client.ServerPath("/vc-templates/" + url.PathEscape(id))
}

func templateBody(ctx context.Context, m *templateModel, diags *diag.Diagnostics) *vcTemplate {
	claims := tfutil.MapOfStrings(ctx, m.Claims, diags)
	t := &vcTemplate{DisplayName: m.DisplayName.ValueString(), Format: m.Format.ValueString(), ExpiresIn: m.ExpiresIn.ValueInt64(), Claims: []vcClaim{}}
	for _, name := range tfutil.SortedKeys(claims) {
		t.Claims = append(t.Claims, vcClaim{Name: name, Type: "LOCAL", ClaimURI: claims[name]})
	}
	return t
}

// claimHint explains the most common rejection.
func claimHint(diags *diag.Diagnostics, err error) {
	if client.HasCode(err, "VCM-60009") {
		diags.AddAttributeError(path.Root("claims"), "Invalid credential claim",
			"Each claim must name a claim of the http://wso2.org/vc/claim dialect (for example given_name or email), not a local claim URI.")
	}
}

func (r *templateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := templateBody(ctx, &plan, &resp.Diagnostics)
	body.Identifier = plan.Identifier.ValueString()
	var out vcTemplate
	if _, err := r.client.Post(ctx, r.client.ServerPath("/vc-templates"), body, &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create verifiable credential template", err)
		claimHint(&resp.Diagnostics, err)
		return
	}
	plan.ID = types.StringValue(out.ID)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Template not found after creation", out.ID)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state templateModel
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

func (r *templateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if _, err := r.client.Put(ctx, r.templatePath(state.ID.ValueString()), templateBody(ctx, &plan, &resp.Diagnostics), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update verifiable credential template", err)
		claimHint(&resp.Diagnostics, err)
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state templateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.templatePath(state.ID.ValueString())); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete verifiable credential template", err)
	}
}

func (r *templateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *templateResource) read(ctx context.Context, m *templateModel, diags *diag.Diagnostics) bool {
	var t vcTemplate
	if err := r.client.Get(ctx, r.templatePath(m.ID.ValueString()), nil, &t); err != nil {
		if !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "read verifiable credential template", err)
		}
		return false
	}
	claims := make(map[string]string, len(t.Claims))
	for _, c := range t.Claims {
		claims[c.Name] = c.ClaimURI
	}
	m.Identifier = types.StringValue(t.Identifier)
	m.DisplayName = types.StringValue(t.DisplayName)
	m.Format = types.StringValue(t.Format)
	m.Claims = tfutil.MapValueOfStrings(claims, false)
	m.ExpiresIn = types.Int64Value(t.ExpiresIn)
	m.OfferID = tfutil.StringValue(t.OfferID)
	return true
}
