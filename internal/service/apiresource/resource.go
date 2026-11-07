// Package apiresource implements wso2is_api_resource and its data source.
package apiresource

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*apiResourceResource)(nil)
	_ resource.ResourceWithImportState = (*apiResourceResource)(nil)
)

// NewResource returns the wso2is_api_resource resource.
func NewResource() resource.Resource { return &apiResourceResource{} }

type apiResourceResource struct{ client *client.Client }

type model struct {
	ID                        types.String `tfsdk:"id"`
	Name                      types.String `tfsdk:"name"`
	Identifier                types.String `tfsdk:"identifier"`
	Description               types.String `tfsdk:"description"`
	RequiresAuthorization     types.Bool   `tfsdk:"requires_authorization"`
	Type                      types.String `tfsdk:"type"`
	Scopes                    types.Set    `tfsdk:"scopes"`
	ScopeIDs                  types.Map    `tfsdk:"scope_ids"`
	AuthorizationDetailsTypes types.Set    `tfsdk:"authorization_details_types"`
}

type scopeModel struct {
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Description types.String `tfsdk:"description"`
}

type adtModel struct {
	Type        types.String         `tfsdk:"type"`
	Name        types.String         `tfsdk:"name"`
	Description types.String         `tfsdk:"description"`
	Schema      jsontypes.Normalized `tfsdk:"schema"`
}

func (r *apiResourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_resource"
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manages an API resource and its scopes. An API resource represents one of your APIs, and each scope is one permission in that API, such as `orders:read`. Use `wso2is_application_authorized_api` to let an application request the scopes, and `wso2is_role` to grant the scopes to users. To import an API resource, use its ID.",
		Attributes: map[string]schema.Attribute{
			"id":   tfutil.IDAttribute("API resource ID."),
			"name": schema.StringAttribute{MarkdownDescription: "Display name.", Required: true},
			"identifier": schema.StringAttribute{
				MarkdownDescription: "Unique identifier, usually the API's base URL (it becomes the `aud` of tokens). Changing it replaces the API resource.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{MarkdownDescription: "Description.", Optional: true},
			"requires_authorization": schema.BoolAttribute{
				MarkdownDescription: "Whether applications must be authorized for the API before they can request its scopes. The server only accepts this value at creation. The default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefaultTrue(),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"type": tfutil.ComputedString("The type of the API resource. APIs that you create with this resource have the type `BUSINESS`."),
			"scopes": schema.SetNestedAttribute{
				MarkdownDescription: "Scopes of the API. Scope names must be unique across the tenant. Changes replace the whole scope set on the server.",
				Optional:            true,
				Computed:            true,
				Default:             emptyScopes(),
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":         schema.StringAttribute{MarkdownDescription: "Scope name, as requested by applications, for example `orders:read`.", Required: true},
					"display_name": schema.StringAttribute{MarkdownDescription: "Display name.", Required: true},
					"description":  schema.StringAttribute{MarkdownDescription: "Description.", Optional: true},
				}},
			},
			"scope_ids": schema.MapAttribute{
				MarkdownDescription: "Scope IDs by scope name.",
				Computed:            true,
				ElementType:         types.StringType,
				PlanModifiers:       []planmodifier.Map{tfutil.MapStateUnlessChanged(path.MatchRoot("scopes"))},
			},
			"authorization_details_types": schema.SetNestedAttribute{
				MarkdownDescription: "Rich authorization request (RFC 9396) types accepted by the API.",
				Optional:            true,
				Computed:            true,
				Default:             emptyADTs(),
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"type":        schema.StringAttribute{MarkdownDescription: "Unique type value used in `authorization_details`.", Required: true},
					"name":        schema.StringAttribute{MarkdownDescription: "Display name.", Required: true},
					"description": schema.StringAttribute{MarkdownDescription: "Description.", Required: true},
					"schema":      schema.StringAttribute{MarkdownDescription: "JSON Schema the authorization details must match (use `jsonencode`).", Required: true, CustomType: jsontypes.NormalizedType{}},
				}},
			},
		},
	}
}

var (
	scopeTypes = func() map[string]attr.Type {
		return tfutil.NestedAttrTypes(resourceSchema().Type().(types.ObjectType).AttrTypes, "scopes")
	}()
	adtTypes = func() map[string]attr.Type {
		return tfutil.NestedAttrTypes(resourceSchema().Type().(types.ObjectType).AttrTypes, "authorization_details_types")
	}()
)

func (r *apiResourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *apiResourceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func expandScopes(ctx context.Context, set types.Set, diags *diag.Diagnostics) []client.APIScope {
	scopes, _ := tfutil.ElementsAs[scopeModel](ctx, set, diags)
	out := make([]client.APIScope, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, client.APIScope{Name: s.Name.ValueString(), DisplayName: s.DisplayName.ValueString(), Description: s.Description.ValueString()})
	}
	return out
}

func expandADTs(ctx context.Context, set types.Set, diags *diag.Diagnostics) []client.AuthorizationDetailsType {
	items, _ := tfutil.ElementsAs[adtModel](ctx, set, diags)
	out := make([]client.AuthorizationDetailsType, 0, len(items))
	for _, a := range items {
		out = append(out, client.AuthorizationDetailsType{Type: a.Type.ValueString(), Name: a.Name.ValueString(), Description: a.Description.ValueString(), Schema: json.RawMessage(a.Schema.ValueString())})
	}
	return out
}

func (r *apiResourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := &client.APIResource{
		Name:                      plan.Name.ValueString(),
		Identifier:                plan.Identifier.ValueString(),
		Description:               plan.Description.ValueString(),
		RequiresAuthorization:     tfutil.BoolPtr(plan.RequiresAuthorization),
		Scopes:                    expandScopes(ctx, plan.Scopes, &resp.Diagnostics),
		AuthorizationDetailsTypes: expandADTs(ctx, plan.AuthorizationDetailsTypes, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.client.CreateAPIResource(ctx, body)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create API resource", err)
		return
	}
	plan.ID = types.StringValue(id)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("API resource not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiResourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

func (r *apiResourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !plan.Name.Equal(state.Name) || !plan.Description.Equal(state.Description) {
		if err := r.client.PatchAPIResource(ctx, id, plan.Name.ValueString(), plan.Description.ValueString()); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update API resource", err)
			return
		}
	}
	if !scopesEqual(ctx, plan.Scopes, state.Scopes, &resp.Diagnostics) {
		if err := r.client.PutAPIScopes(ctx, id, expandScopes(ctx, plan.Scopes, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update API resource scopes", err)
			return
		}
	}
	if !adtsEqual(ctx, plan.AuthorizationDetailsTypes, state.AuthorizationDetailsTypes, &resp.Diagnostics) {
		if err := r.client.PutAuthorizationDetailsTypes(ctx, id, expandADTs(ctx, plan.AuthorizationDetailsTypes, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update API resource authorization details types", err)
			return
		}
	}
	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("API resource not found after update", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiResourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAPIResource(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete API resource", err)
	}
}

func (r *apiResourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *apiResourceResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	a, err := r.client.GetAPIResource(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read API resource", err)
		return false
	}
	m.Name = types.StringValue(a.Name)
	m.Identifier = types.StringValue(a.Identifier)
	m.Description = tfutil.StringValueOrNull(a.Description)
	m.RequiresAuthorization = tfutil.BoolValueOr(a.RequiresAuthorization, true)
	m.Type = types.StringValue(a.Type)
	m.Scopes = flattenScopes(ctx, a.Scopes, diags)
	ids := map[string]string{}
	for _, sc := range a.Scopes {
		ids[sc.Name] = sc.ID
	}
	m.ScopeIDs = tfutil.MapValueOfStrings(ids, false)
	m.AuthorizationDetailsTypes = flattenADTs(ctx, a.AuthorizationDetailsTypes, diags)
	return !diags.HasError()
}

func flattenScopes(ctx context.Context, in []client.APIScope, diags *diag.Diagnostics) types.Set {
	out := make([]scopeModel, 0, len(in))
	for _, s := range in {
		out = append(out, scopeModel{
			Name: types.StringValue(s.Name), DisplayName: types.StringValue(s.DisplayName),
			Description: tfutil.StringValueOrNull(s.Description),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name.ValueString() < out[j].Name.ValueString() })
	return tfutil.SetFrom(ctx, scopeTypes, out, diags)
}

func flattenADTs(ctx context.Context, in []client.AuthorizationDetailsType, diags *diag.Diagnostics) types.Set {
	out := make([]adtModel, 0, len(in))
	for _, a := range in {
		out = append(out, adtModel{
			Type: types.StringValue(a.Type), Name: types.StringValue(a.Name),
			Description: types.StringValue(a.Description), Schema: jsontypes.NewNormalizedValue(string(a.Schema)),
		})
	}
	return tfutil.SetFrom(ctx, adtTypes, out, diags)
}

// scopesEqual compares scopes ignoring the computed IDs.
func scopesEqual(ctx context.Context, a, b types.Set, diags *diag.Diagnostics) bool {
	key := func(s types.Set) map[string]client.APIScope {
		m := map[string]client.APIScope{}
		for _, sc := range expandScopes(ctx, s, diags) {
			m[sc.Name] = sc
		}
		return m
	}
	ka, kb := key(a), key(b)
	if len(ka) != len(kb) {
		return false
	}
	for k, v := range ka {
		if kb[k] != v {
			return false
		}
	}
	return true
}

func adtsEqual(ctx context.Context, a, b types.Set, diags *diag.Diagnostics) bool {
	ea, _ := json.Marshal(expandADTs(ctx, a, diags))
	eb, _ := json.Marshal(expandADTs(ctx, b, diags))
	return string(ea) == string(eb)
}

func booldefaultTrue() defaults.Bool { return booldefault.StaticBool(true) }

func emptyScopes() defaults.Set {
	return setdefault.StaticValue(types.SetValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
		"name": types.StringType, "display_name": types.StringType, "description": types.StringType,
	}}, nil))
}

func emptyADTs() defaults.Set {
	return setdefault.StaticValue(types.SetValueMust(types.ObjectType{AttrTypes: map[string]attr.Type{
		"type": types.StringType, "name": types.StringType, "description": types.StringType, "schema": jsontypes.NormalizedType{},
	}}, nil))
}
