// Package claim implements local claims, claim dialects, external claims and OIDC scopes.
package claim

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*claimResource)(nil)
	_ resource.ResourceWithImportState = (*claimResource)(nil)
)

// NewClaimResource returns the wso2is_claim resource (local claims).
func NewClaimResource() resource.Resource { return &claimResource{} }

type claimResource struct{ client *client.Client }

type claimModel struct {
	ID                                types.String `tfsdk:"id"`
	ClaimURI                          types.String `tfsdk:"claim_uri"`
	DisplayName                       types.String `tfsdk:"display_name"`
	Description                       types.String `tfsdk:"description"`
	DisplayOrder                      types.Int64  `tfsdk:"display_order"`
	ReadOnly                          types.Bool   `tfsdk:"read_only"`
	Required                          types.Bool   `tfsdk:"required"`
	SupportedByDefault                types.Bool   `tfsdk:"supported_by_default"`
	Regex                             types.String `tfsdk:"regex"`
	DataType                          types.String `tfsdk:"data_type"`
	SubAttributes                     types.List   `tfsdk:"sub_attributes"`
	CanonicalValues                   types.List   `tfsdk:"canonical_values"`
	MultiValued                       types.Bool   `tfsdk:"multi_valued"`
	InputType                         types.String `tfsdk:"input_type"`
	UniquenessScope                   types.String `tfsdk:"uniqueness_scope"`
	SharedProfileValueResolvingMethod types.String `tfsdk:"shared_profile_value_resolving_method"`
	ManagedInUserStore                types.Bool   `tfsdk:"managed_in_user_store"`
	AttributeMappings                 types.Map    `tfsdk:"attribute_mappings"`
	ExcludedUserStores                types.Set    `tfsdk:"excluded_user_stores"`
	Properties                        types.Map    `tfsdk:"properties"`
	Profiles                          types.Map    `tfsdk:"profiles"`
}

type canonicalValueModel struct {
	Label types.String `tfsdk:"label"`
	Value types.String `tfsdk:"value"`
}

type profileModel struct {
	ReadOnly           types.Bool `tfsdk:"read_only"`
	Required           types.Bool `tfsdk:"required"`
	SupportedByDefault types.Bool `tfsdk:"supported_by_default"`
}

func (r *claimResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_claim"
}

func claimSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manages a local claim. A local claim is a user attribute in the `http://wso2.org/claims` dialect. The resource also sets where each user store keeps the attribute.\n\nApplications do not see local claims directly. To send the attribute to applications, map it in the OIDC dialect with `wso2is_external_claim` (`dialect_uri = \"http://wso2.org/oidc/claim\"`), and add it to an OIDC scope with `wso2is_oidc_scope`. To show it in the SCIM APIs, map it in a SCIM dialect too. To import a claim, use its claim URI.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Claim ID (base64url of the claim URI)."),
			"claim_uri": schema.StringAttribute{
				MarkdownDescription: "Claim URI, for example `http://wso2.org/claims/employeeId`. Changing it replaces the claim.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.RegexMatches(mustRegexp(`^http://wso2\.org/claims/\S+$`), "must start with http://wso2.org/claims/")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"display_name":         schema.StringAttribute{MarkdownDescription: "Name shown in forms and the Console.", Required: true},
			"description":          tfutil.String("Description.", ""),
			"display_order":        tfutil.Int64("Position of the attribute in profile and registration forms.", 0),
			"read_only":            tfutil.Bool("Users cannot change the value.", false),
			"required":             tfutil.Bool("A value is mandatory in profiles.", false),
			"supported_by_default": tfutil.Bool("Shown in user profiles (and self registration) by default.", false),
			"regex":                tfutil.String("Regular expression values must match.", ""),
			"data_type": schema.StringAttribute{
				MarkdownDescription: "`string`, `boolean`, `integer`, `decimal`, `date_time` or `complex`. The default is `string`.",
				Optional:            true,
				Computed:            true,
				Default:             stringDefault("string"),
				Validators:          []validator.String{stringvalidator.OneOf("string", "boolean", "integer", "decimal", "date_time", "complex")},
			},
			"sub_attributes": tfutil.StringList("Sub-attribute claim URIs of a `complex` attribute."),
			"canonical_values": tfutil.ObjectList("Allowed values (shown as a drop-down).", map[string]schema.Attribute{
				"label": schema.StringAttribute{MarkdownDescription: "Display label.", Required: true},
				"value": schema.StringAttribute{MarkdownDescription: "Stored value.", Required: true},
			}),
			"multi_valued":                          tfutil.Bool("The attribute holds several values.", false),
			"input_type":                            tfutil.OCString("Input control used in forms, for example `text_input`, `dropdown`, `checkbox_group`, `radio_group`, `date_picker`, `toggle`."),
			"uniqueness_scope":                      tfutil.OCString("Uniqueness validation: `NONE`, `WITHIN_USERSTORE` or `ACROSS_USERSTORES`."),
			"shared_profile_value_resolving_method": tfutil.OCString("How the value is resolved for users shared with sub-organizations: `FromOrigin`, `FromSharedProfile` or `FromFirstFoundInHierarchy`."),
			"managed_in_user_store":                 tfutil.OCBool("Whether the value is stored in the user store (otherwise in the identity database)."),
			"attribute_mappings": schema.MapAttribute{
				MarkdownDescription: "User store attribute (column or LDAP attribute) by user store domain, for example `{ PRIMARY = \"employeeId\" }`.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"excluded_user_stores": tfutil.StringSet("User stores where the attribute is not available."),
			"properties": schema.MapAttribute{
				MarkdownDescription: "Additional claim properties. Only the keys listed here are managed.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"profiles": schema.MapNestedAttribute{
				MarkdownDescription: "Per-profile overrides (WSO2 IS 7.1+), keyed by profile: `console`, `endUser`, `selfRegistration`.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"read_only":            schema.BoolAttribute{MarkdownDescription: "Users cannot change the value in this profile.", Optional: true},
					"required":             schema.BoolAttribute{MarkdownDescription: "Required in this profile.", Optional: true},
					"supported_by_default": schema.BoolAttribute{MarkdownDescription: "Shown in this profile.", Optional: true},
				}},
			},
		},
	}
}

var (
	claimAttrTypes     = claimSchema().Type().(types.ObjectType).AttrTypes
	canonicalValueType = tfutil.NestedAttrTypes(claimAttrTypes, "canonical_values")
	profileTypes       = claimAttrTypes["profiles"].(types.MapType).ElemType.(types.ObjectType).AttrTypes
)

func (r *claimResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = claimSchema()
}

func (r *claimResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

// expand renders the managed fields of a local claim.
func expandClaim(ctx context.Context, m *claimModel, diags *diag.Diagnostics) map[string]any {
	j := tfutil.JSON{}
	j.Str("claimURI", m.ClaimURI).Str("displayName", m.DisplayName).Str("description", m.Description)
	j.Int("displayOrder", m.DisplayOrder).Bool("readOnly", m.ReadOnly).Bool("required", m.Required)
	j.Bool("supportedByDefault", m.SupportedByDefault).Str("regEx", m.Regex).Str("dataType", m.DataType)
	j.Strs(ctx, "subAttributes", m.SubAttributes, diags).Bool("multiValued", m.MultiValued)
	j.Str("uniquenessScope", m.UniquenessScope).Str("sharedProfileValueResolvingMethod", m.SharedProfileValueResolvingMethod)
	j.Bool("managedInUserStore", m.ManagedInUserStore)
	if tfutil.Known(m.InputType) && m.InputType.ValueString() != "" {
		j["inputFormat"] = map[string]any{"inputType": m.InputType.ValueString()}
	}
	if cvs, ok := tfutil.ElementsAs[canonicalValueModel](ctx, m.CanonicalValues, diags); ok {
		list := make([]map[string]any, 0, len(cvs))
		for _, cv := range cvs {
			list = append(list, map[string]any{"label": cv.Label.ValueString(), "value": cv.Value.ValueString()})
		}
		j["canonicalValues"] = list
	}
	mappings := tfutil.MapOfStrings(ctx, m.AttributeMappings, diags)
	list := make([]map[string]any, 0, len(mappings))
	for _, us := range tfutil.SortedKeys(mappings) {
		list = append(list, map[string]any{"userstore": us, "mappedAttribute": mappings[us]})
	}
	j["attributeMapping"] = list

	props := []map[string]any{}
	for k, v := range tfutil.MapOfStrings(ctx, m.Properties, diags) {
		props = append(props, map[string]any{"key": k, "value": v})
	}
	if tfutil.Known(m.ExcludedUserStores) {
		ex := tfutil.Strings(ctx, m.ExcludedUserStores, diags)
		sort.Strings(ex)
		props = append(props, map[string]any{"key": "ExcludedUserStores", "value": strings.Join(ex, ",")})
	}
	sort.Slice(props, func(i, k int) bool { return props[i]["key"].(string) < props[k]["key"].(string) })
	j["properties"] = props

	if !m.Profiles.IsNull() && !m.Profiles.IsUnknown() {
		profiles := map[string]any{}
		var pm map[string]profileModel
		diags.Append(m.Profiles.ElementsAs(ctx, &pm, false)...)
		for name, p := range pm {
			profiles[name] = tfutil.JSON{}.Bool("readOnly", p.ReadOnly).Bool("required", p.Required).Bool("supportedByDefault", p.SupportedByDefault).Map()
		}
		j["profiles"] = profiles
	}
	return j.Map()
}

func (r *claimResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan claimModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := client.PruneEmpty(expandClaim(ctx, &plan, &resp.Diagnostics), nil)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.client.CreateLocalClaim(ctx, body)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create claim", err)
		return
	}
	plan.ID = types.StringValue(id)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Claim not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *claimResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state claimModel
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

func (r *claimResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state claimModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	cur, err := r.client.GetLocalClaim(ctx, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read claim", err)
		return
	}
	for _, k := range []string{"id", "dialectURI"} {
		delete(cur, k)
	}
	diffs := client.DiffJSON(expandClaim(ctx, &plan, &resp.Diagnostics), expandClaim(ctx, &state, &resp.Diagnostics))
	if props, ok := diffs["properties"]; ok {
		// Keep server-side properties that are not managed here.
		diffs["properties"] = mergeProperties(cur["properties"], props, tfutil.MapOfStrings(ctx, state.Properties, &resp.Diagnostics))
	}
	body := client.MergeJSON(cur, diffs)
	if err := r.client.PutLocalClaim(ctx, id, body); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update claim", err)
		return
	}
	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Claim not found after update", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// mergeProperties combines the server's properties with the managed ones: managed keys
// take the new values, keys removed from the configuration are dropped, others stay.
func mergeProperties(server, managed any, previouslyManaged map[string]string) []map[string]any {
	out := map[string]string{}
	for _, p := range toObjects(server) {
		k, _ := p["key"].(string)
		v, _ := p["value"].(string)
		if _, was := previouslyManaged[k]; was {
			continue
		}
		out[k] = v
	}
	switch m := managed.(type) {
	case []map[string]any:
		for _, p := range m {
			out[p["key"].(string)], _ = p["value"].(string)
		}
	case []any:
		for _, p := range toObjects(m) {
			k, _ := p["key"].(string)
			out[k], _ = p["value"].(string)
		}
	}
	list := make([]map[string]any, 0, len(out))
	for _, k := range tfutil.SortedKeys(out) {
		list = append(list, map[string]any{"key": k, "value": out[k]})
	}
	return list
}

func (r *claimResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state claimModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteLocalClaim(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete claim", err)
	}
}

func (r *claimResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if strings.HasPrefix(id, "http") {
		id = client.ClaimID(id)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *claimResource) read(ctx context.Context, m *claimModel, diags *diag.Diagnostics) bool {
	c, err := r.client.GetLocalClaim(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read claim", err)
		return false
	}
	m.ClaimURI = tfutil.GetString(c, "claimURI")
	m.DisplayName = tfutil.GetString(c, "displayName")
	m.Description = tfutil.GetStringOrEmpty(c, "description")
	m.DisplayOrder = tfutil.GetInt64(c, "displayOrder")
	if m.DisplayOrder.IsNull() {
		m.DisplayOrder = types.Int64Value(0)
	}
	m.ReadOnly = tfutil.GetBool(c, "readOnly", false)
	m.Required = tfutil.GetBool(c, "required", false)
	m.SupportedByDefault = tfutil.GetBool(c, "supportedByDefault", false)
	m.Regex = tfutil.GetStringOrEmpty(c, "regEx")
	m.DataType = tfutil.GetString(c, "dataType")
	if m.DataType.IsNull() || m.DataType.ValueString() == "" {
		m.DataType = types.StringValue("string")
	}
	m.SubAttributes = tfutil.ListOfStrings(tfutil.GetStrings(c, "subAttributes"), false)
	m.MultiValued = tfutil.GetBool(c, "multiValued", false)
	m.InputType = tfutil.GetStringOrEmpty(tfutil.GetMap(c, "inputFormat"), "inputType")
	m.UniquenessScope = tfutil.GetStringOrEmpty(c, "uniquenessScope")
	m.SharedProfileValueResolvingMethod = tfutil.GetStringOrEmpty(c, "sharedProfileValueResolvingMethod")
	m.ManagedInUserStore = tfutil.GetBool(c, "managedInUserStore", true)

	cvs := []canonicalValueModel{}
	for _, cv := range tfutil.GetObjects(c, "canonicalValues") {
		cvs = append(cvs, canonicalValueModel{Label: tfutil.GetString(cv, "label"), Value: tfutil.GetString(cv, "value")})
	}
	m.CanonicalValues = tfutil.ListFrom(ctx, canonicalValueType, cvs, diags)

	mappings := map[string]string{}
	for _, am := range tfutil.GetObjects(c, "attributeMapping") {
		us, _ := am["userstore"].(string)
		at, _ := am["mappedAttribute"].(string)
		mappings[us] = at
	}
	m.AttributeMappings = tfutil.MapValueOfStrings(mappings, false)

	managed := m.Properties.Elements()
	props := map[string]string{}
	excluded := []string{}
	for _, p := range tfutil.GetObjects(c, "properties") {
		k, _ := p["key"].(string)
		v, _ := p["value"].(string)
		if k == "ExcludedUserStores" {
			for _, us := range strings.Split(v, ",") {
				if us = strings.TrimSpace(us); us != "" {
					excluded = append(excluded, us)
				}
			}
			continue
		}
		if _, ok := managed[k]; ok {
			props[k] = v
		}
	}
	if !m.Properties.IsNull() {
		m.Properties = tfutil.MapValueOfStrings(props, true)
	}
	sort.Strings(excluded)
	m.ExcludedUserStores = tfutil.SetOfStrings(excluded, false)

	if !m.Profiles.IsNull() {
		profiles := map[string]attr.Value{}
		for name, raw := range tfutil.GetMap(c, "profiles") {
			p, _ := raw.(map[string]any)
			pm := profileModel{ReadOnly: optBool(p, "readOnly"), Required: optBool(p, "required"), SupportedByDefault: optBool(p, "supportedByDefault")}
			profiles[name] = tfutil.ObjectFrom(ctx, profileTypes, pm, diags)
		}
		v, d := types.MapValue(types.ObjectType{AttrTypes: profileTypes}, profiles)
		diags.Append(d...)
		m.Profiles = v
	}
	return !diags.HasError()
}

func optBool(m map[string]any, k string) types.Bool {
	if b, ok := m[k].(bool); ok {
		return types.BoolValue(b)
	}
	return types.BoolNull()
}

func toObjects(v any) []map[string]any {
	arr, _ := v.([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
