package claim

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// customSCIMDialect is created on demand by wso2is_external_claim and never deleted.
const customSCIMDialect = client.SCIMCustomUser

func mustRegexp(s string) *regexp.Regexp { return regexp.MustCompile(s) }

func stringDefault(s string) defaults.String { return stringdefault.StaticString(s) }

// ---- wso2is_claim_dialect ----

var (
	_ resource.ResourceWithConfigure   = (*dialectResource)(nil)
	_ resource.ResourceWithImportState = (*dialectResource)(nil)
)

// NewDialectResource returns the wso2is_claim_dialect resource.
func NewDialectResource() resource.Resource { return &dialectResource{} }

type dialectResource struct{ client *client.Client }

type dialectModel struct {
	ID         types.String `tfsdk:"id"`
	DialectURI types.String `tfsdk:"dialect_uri"`
}

func (r *dialectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_claim_dialect"
}

func (r *dialectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an external claim dialect. A dialect is a set of claim URIs that another system uses, such as an identity provider or an older application. Map each claim of the dialect to a local claim with `wso2is_external_claim`. Deleting a dialect also deletes its claims. To import a dialect, use its dialect URI.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Dialect ID (base64url of the URI)."),
			"dialect_uri": schema.StringAttribute{
				MarkdownDescription: "Dialect URI, for example `urn:example:claims`. Changing it replaces the dialect.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *dialectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *dialectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dialectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.client.CreateClaimDialect(ctx, plan.DialectURI.ValueString())
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create claim dialect", err)
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *dialectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dialectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tfutil.MissingID(state.ID) {
		resp.State.RemoveResource(ctx)
		return
	}
	d, err := r.client.GetClaimDialect(ctx, state.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		tfutil.AddAPIError(&resp.Diagnostics, "read claim dialect", err)
		return
	}
	state.DialectURI = types.StringValue(d.DialectURI)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dialectResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {}

func (r *dialectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dialectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteClaimDialect(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete claim dialect", err)
	}
}

func (r *dialectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if strings.Contains(id, ":") {
		id = client.DialectID(id)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// ---- wso2is_external_claim ----

var (
	_ resource.ResourceWithConfigure   = (*externalClaimResource)(nil)
	_ resource.ResourceWithImportState = (*externalClaimResource)(nil)
)

// NewExternalClaimResource returns the wso2is_external_claim resource.
func NewExternalClaimResource() resource.Resource { return &externalClaimResource{} }

type externalClaimResource struct{ client *client.Client }

type externalClaimModel struct {
	ID                  types.String `tfsdk:"id"`
	DialectURI          types.String `tfsdk:"dialect_uri"`
	ClaimURI            types.String `tfsdk:"claim_uri"`
	MappedLocalClaimURI types.String `tfsdk:"mapped_local_claim_uri"`
}

func (r *externalClaimResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_claim"
}

func (r *externalClaimResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Maps a claim of an external dialect to a local claim. You can also use the built-in dialects:\n\n* `http://wso2.org/oidc/claim` holds the OIDC claims. Add them to tokens with `wso2is_oidc_scope`.\n* `urn:ietf:params:scim:schemas:core:2.0:User`, `urn:scim:wso2:schema` and `urn:scim:schemas:extension:custom:User` hold SCIM attributes.\n\nA new server has no custom SCIM dialect. The provider creates it when you add the first claim to it, and leaves it in place when you destroy the resource. To import, use the ID `<dialect_uri>|<claim_uri>`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("`<dialect id>/<claim id>`."),
			"dialect_uri": schema.StringAttribute{
				MarkdownDescription: "Dialect URI. Changing it replaces the claim.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"claim_uri": schema.StringAttribute{
				MarkdownDescription: "External claim URI, for example `employee_id` (OIDC) or `urn:scim:schemas:extension:custom:User:employeeId` (SCIM). Changing it replaces the claim.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"mapped_local_claim_uri": schema.StringAttribute{MarkdownDescription: "Local claim URI it maps to.", Required: true},
		},
	}
}

func (r *externalClaimResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func externalIDs(m *externalClaimModel) (string, string) {
	return client.DialectID(m.DialectURI.ValueString()), client.ClaimID(m.ClaimURI.ValueString())
}

func (r *externalClaimResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan externalClaimModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dialectID, _ := externalIDs(&plan)
	if plan.DialectURI.ValueString() == customSCIMDialect {
		// The custom SCIM dialect is listed in /scim2/Schemas but only exists as a
		// claim dialect once something creates it.
		if _, err := r.client.GetClaimDialect(ctx, dialectID); client.IsNotFound(err) {
			if _, err := r.client.CreateClaimDialect(ctx, customSCIMDialect); err != nil && !client.IsConflict(err) {
				tfutil.AddAPIError(&resp.Diagnostics, "create the custom SCIM dialect", err)
				return
			}
		}
	}
	id, err := r.client.CreateExternalClaim(ctx, dialectID, &client.ExternalClaim{ClaimURI: plan.ClaimURI.ValueString(), MappedLocalClaimURI: plan.MappedLocalClaimURI.ValueString()})
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create external claim", err)
		return
	}
	plan.ID = types.StringValue(dialectID + "/" + id)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *externalClaimResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state externalClaimModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tfutil.MissingID(state.ID) {
		resp.State.RemoveResource(ctx)
		return
	}
	dialectID, claimID, _ := strings.Cut(state.ID.ValueString(), "/")
	ec, err := r.client.GetExternalClaim(ctx, dialectID, claimID)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		tfutil.AddAPIError(&resp.Diagnostics, "read external claim", err)
		return
	}
	state.ClaimURI = types.StringValue(ec.ClaimURI)
	state.MappedLocalClaimURI = types.StringValue(ec.MappedLocalClaimURI)
	if ec.ClaimDialectURI != "" {
		state.DialectURI = types.StringValue(ec.ClaimDialectURI)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *externalClaimResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state externalClaimModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dialectID, claimID, _ := strings.Cut(state.ID.ValueString(), "/")
	err := r.client.PutExternalClaim(ctx, dialectID, claimID, &client.ExternalClaim{ClaimURI: plan.ClaimURI.ValueString(), MappedLocalClaimURI: plan.MappedLocalClaimURI.ValueString()})
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update external claim", err)
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *externalClaimResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state externalClaimModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dialectID, claimID, _ := strings.Cut(state.ID.ValueString(), "/")
	if err := r.client.DeleteExternalClaim(ctx, dialectID, claimID); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete external claim", err)
	}
}

func (r *externalClaimResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var dialectID, claimID string
	if d, c, ok := strings.Cut(req.ID, "|"); ok {
		dialectID, claimID = client.DialectID(d), client.ClaimID(c)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("dialect_uri"), d)...)
	} else if d, c, ok := strings.Cut(req.ID, "/"); ok {
		dialectID, claimID = d, c
	} else {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <dialect_uri>|<claim_uri>, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), dialectID+"/"+claimID)...)
}

// ---- wso2is_oidc_scope ----

var (
	_ resource.ResourceWithConfigure   = (*oidcScopeResource)(nil)
	_ resource.ResourceWithImportState = (*oidcScopeResource)(nil)
)

// NewOIDCScopeResource returns the wso2is_oidc_scope resource.
func NewOIDCScopeResource() resource.Resource { return &oidcScopeResource{} }

type oidcScopeResource struct{ client *client.Client }

type oidcScopeModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Description types.String `tfsdk:"description"`
	Claims      types.Set    `tfsdk:"claims"`
}

func (r *oidcScopeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_scope"
}

func (r *oidcScopeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an OpenID Connect scope. When an application requests the scope, the server returns the scope's OIDC claims in ID tokens and from the `/userinfo` endpoint. The claims must exist in the `http://wso2.org/oidc/claim` dialect. You can also import and change the built-in scopes, such as `openid`, `profile`, `email`, `phone` and `address`. To import, use the scope name.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Scope name."),
			"name": schema.StringAttribute{
				MarkdownDescription: "Scope name. Changing it replaces the scope.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name": schema.StringAttribute{MarkdownDescription: "Display name shown on consent pages.", Required: true},
			"description":  tfutil.String("Description.", ""),
			"claims": schema.SetAttribute{
				MarkdownDescription: "OIDC claim names, for example `email`, `given_name`, or custom ones mapped with `wso2is_external_claim`.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			},
		},
	}
}

func (r *oidcScopeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func scopeBody(ctx context.Context, m *oidcScopeModel, diags *diag.Diagnostics) *client.OIDCScope {
	claims := tfutil.Strings(ctx, m.Claims, diags)
	if claims == nil {
		claims = []string{}
	}
	return &client.OIDCScope{Name: m.Name.ValueString(), DisplayName: m.DisplayName.ValueString(), Description: m.Description.ValueString(), Claims: claims}
}

func (r *oidcScopeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan oidcScopeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.CreateOIDCScope(ctx, scopeBody(ctx, &plan, &resp.Diagnostics)); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create OIDC scope", err)
		return
	}
	plan.ID = plan.Name
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oidcScopeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oidcScopeModel
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

func (r *oidcScopeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan oidcScopeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.PutOIDCScope(ctx, plan.Name.ValueString(), scopeBody(ctx, &plan, &resp.Diagnostics)); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update OIDC scope", err)
		return
	}
	plan.ID = plan.Name
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oidcScopeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oidcScopeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteOIDCScope(ctx, state.Name.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete OIDC scope", err)
	}
}

func (r *oidcScopeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func (r *oidcScopeResource) read(ctx context.Context, m *oidcScopeModel, diags *diag.Diagnostics) bool {
	s, err := r.client.GetOIDCScope(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read OIDC scope", err)
		return false
	}
	m.Name = types.StringValue(s.Name)
	if s.Name == "" {
		m.Name = m.ID
	}
	m.DisplayName = types.StringValue(s.DisplayName)
	m.Description = types.StringValue(s.Description)
	claims := append([]string(nil), s.Claims...)
	sort.Strings(claims)
	m.Claims = tfutil.SetOfStrings(claims, false)
	return true
}
