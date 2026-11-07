// Package action implements wso2is_action (extension endpoints called during flows).
package action

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	_ resource.ResourceWithConfigure   = (*actionResource)(nil)
	_ resource.ResourceWithImportState = (*actionResource)(nil)
)

// actionTypes maps the attribute values to the API path segments.
var actionTypes = map[string]string{
	"pre_issue_access_token": "preIssueAccessToken",
	"pre_issue_id_token":     "preIssueIdToken",
	"pre_update_password":    "preUpdatePassword",
	"pre_update_profile":     "preUpdateProfile",
	"pre_registration":       "preRegistration",
}

// NewResource returns the wso2is_action resource.
func NewResource() resource.Resource { return &actionResource{} }

type actionResource struct{ client *client.Client }

type model struct {
	ID                       types.String `tfsdk:"id"`
	Type                     types.String `tfsdk:"type"`
	Name                     types.String `tfsdk:"name"`
	Description              types.String `tfsdk:"description"`
	EndpointURI              types.String `tfsdk:"endpoint_uri"`
	AuthenticationType       types.String `tfsdk:"authentication_type"`
	AuthenticationProperties types.Map    `tfsdk:"authentication_properties"`
	AllowedHeaders           types.Set    `tfsdk:"allowed_headers"`
	AllowedParameters        types.Set    `tfsdk:"allowed_parameters"`
	Rules                    types.List   `tfsdk:"rules"`
	Active                   types.Bool   `tfsdk:"active"`
	PasswordSharingFormat    types.String `tfsdk:"password_sharing_format"`
	PasswordSharingCert      types.String `tfsdk:"password_sharing_certificate"`
	Attributes               types.Set    `tfsdk:"attributes"`
	Version                  types.String `tfsdk:"version"`
}

type ruleModel struct {
	Expressions types.List `tfsdk:"expressions"`
}

type expressionModel struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
}

var (
	expressionTypes = map[string]attr.Type{"field": types.StringType, "operator": types.StringType, "value": types.StringType}
	ruleTypes       = map[string]attr.Type{"expressions": types.ListType{ElemType: types.ObjectType{AttrTypes: expressionTypes}}}
)

func (r *actionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

func (r *actionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an action. An action is an HTTPS endpoint that you run, and that WSO2 Identity Server calls at a fixed point in a flow. For example, the server can call an action before it issues an access token or an ID token, so that the action can add or change claims. Other actions run before a password update, before a profile update, or before registration. By default, a tenant can have one action of each type. To import an action, use the ID `<type>/<id>`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Action ID."),
			"type": schema.StringAttribute{
				MarkdownDescription: "`pre_issue_access_token`, `pre_issue_id_token`, `pre_update_password`, `pre_update_profile` or `pre_registration` (the last needs to be enabled on the server). Changing it replaces the action.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(tfutil.SortedKeys(actionTypes)...)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name":         schema.StringAttribute{MarkdownDescription: "Name.", Required: true},
			"description":  tfutil.String("Description.", ""),
			"endpoint_uri": schema.StringAttribute{MarkdownDescription: "HTTPS endpoint called by the server.", Required: true},
			"authentication_type": schema.StringAttribute{
				MarkdownDescription: "How the server signs in to the endpoint. The allowed values are `NONE`, `BASIC`, `BEARER` and `API_KEY`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("NONE", "BASIC", "BEARER", "API_KEY")},
			},
			"authentication_properties": schema.MapAttribute{
				MarkdownDescription: "The credentials for the endpoint. Use `username` and `password` for `BASIC`, `accessToken` for `BEARER`, or `header` and `value` for `API_KEY`. The server never returns these values, so the provider cannot notice changes made outside Terraform.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
			"allowed_headers":    tfutil.StringSet("Request headers forwarded to the endpoint (token actions only)."),
			"allowed_parameters": tfutil.StringSet("Request parameters forwarded to the endpoint (token actions only)."),
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "Rules that decide when the action runs. The action runs when any rule matches. A rule matches when all of its expressions match. Without rules, the action always runs. Token actions can test the `application` (an application ID) and `grantType` fields. Password and profile actions can test the `flow` field, for example `adminInitiatedPasswordReset` or `userInitiatedPasswordUpdate`.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"expressions": schema.ListNestedAttribute{
						MarkdownDescription: "Expressions, all of which must match.",
						Required:            true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"field":    schema.StringAttribute{MarkdownDescription: "Field, for example `application`, `grantType` or `flow`.", Required: true},
							"operator": schema.StringAttribute{MarkdownDescription: "`equals` or `notEquals`.", Required: true},
							"value":    schema.StringAttribute{MarkdownDescription: "Value to compare with.", Required: true},
						}},
					},
				}},
			},
			"active": schema.BoolAttribute{
				MarkdownDescription: "Whether the action is active. The default is `true`. The server creates actions as inactive, so the provider activates them after it creates them.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"password_sharing_format": schema.StringAttribute{
				MarkdownDescription: "How the server shares the new password with the action, either `PLAIN_TEXT` or `SHA256_HASHED`. Only `pre_update_password` actions use this setting.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("PLAIN_TEXT", "SHA256_HASHED")},
			},
			"password_sharing_certificate": schema.StringAttribute{
				MarkdownDescription: "`pre_update_password` only: PEM certificate used to encrypt the shared password.",
				Optional:            true,
			},
			"attributes": schema.SetAttribute{
				MarkdownDescription: "The claim URIs that trigger the action when they change. The server sends these claims to the action. Only `pre_update_profile` actions use this setting.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"version": tfutil.ComputedString("Action version."),
		},
	}
}

func (r *actionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *actionResource) collection(m *model) string {
	return r.client.ServerPath("/actions/" + actionTypes[m.Type.ValueString()])
}

func (r *actionResource) item(m *model) string {
	return r.collection(m) + "/" + url.PathEscape(m.ID.ValueString())
}

func authBody(ctx context.Context, m *model, diags *diag.Diagnostics) map[string]any {
	auth := map[string]any{"type": m.AuthenticationType.ValueString()}
	if p := tfutil.MapOfStrings(ctx, m.AuthenticationProperties, diags); len(p) > 0 {
		props := map[string]any{}
		for k, v := range p {
			props[k] = v
		}
		auth["properties"] = props
	}
	return auth
}

func rulesBody(ctx context.Context, m *model, diags *diag.Diagnostics) map[string]any {
	rules, ok := tfutil.ElementsAs[ruleModel](ctx, m.Rules, diags)
	if !ok || len(rules) == 0 {
		return nil
	}
	list := []any{}
	for _, rl := range rules {
		exprs, _ := tfutil.ElementsAs[expressionModel](ctx, rl.Expressions, diags)
		el := []any{}
		for _, e := range exprs {
			el = append(el, map[string]any{"field": e.Field.ValueString(), "operator": e.Operator.ValueString(), "value": e.Value.ValueString()})
		}
		list = append(list, map[string]any{"condition": "AND", "expressions": el})
	}
	return map[string]any{"condition": "OR", "rules": list}
}

func typeSpecific(ctx context.Context, m *model, body map[string]any, diags *diag.Diagnostics) {
	if tfutil.Known(m.PasswordSharingFormat) {
		ps := map[string]any{"format": m.PasswordSharingFormat.ValueString()}
		if c := m.PasswordSharingCert.ValueString(); c != "" {
			ps["certificate"] = c
		}
		body["passwordSharing"] = ps
	}
	if tfutil.Known(m.Attributes) {
		body["attributes"] = tfutil.Strings(ctx, m.Attributes, diags)
	}
}

func (r *actionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{
		"name":        plan.Name.ValueString(),
		"description": plan.Description.ValueString(),
		"endpoint": map[string]any{
			"uri":            plan.EndpointURI.ValueString(),
			"authentication": authBody(ctx, &plan, &resp.Diagnostics),
		},
	}
	// Only token actions accept forwarded headers and parameters; the others reject the
	// fields even when empty.
	ep := body["endpoint"].(map[string]any)
	if h := tfutil.Strings(ctx, plan.AllowedHeaders, &resp.Diagnostics); len(h) > 0 {
		ep["allowedHeaders"] = h
	}
	if p := tfutil.Strings(ctx, plan.AllowedParameters, &resp.Diagnostics); len(p) > 0 {
		ep["allowedParameters"] = p
	}
	if rb := rulesBody(ctx, &plan, &resp.Diagnostics); rb != nil {
		body["rule"] = rb
	}
	typeSpecific(ctx, &plan, body, &resp.Diagnostics)
	var out map[string]any
	if _, err := r.client.Post(ctx, r.collection(&plan), body, &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create action", err)
		return
	}
	id, _ := out["id"].(string)
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	r.setActive(ctx, &plan, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *actionResource) setActive(ctx context.Context, m *model, currentlyActive bool, diags *diag.Diagnostics) {
	want := m.Active.ValueBool()
	if want == currentlyActive {
		return
	}
	op := "/deactivate"
	if want {
		op = "/activate"
	}
	if _, err := r.client.Post(ctx, r.item(m)+op, nil, nil); err != nil {
		tfutil.AddAPIError(diags, strings.TrimPrefix(op, "/")+" action", err)
	}
}

func (r *actionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
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

func (r *actionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	body := map[string]any{}
	if !plan.Name.Equal(state.Name) {
		body["name"] = plan.Name.ValueString()
	}
	if !plan.Description.Equal(state.Description) {
		body["description"] = plan.Description.ValueString()
	}
	endpoint := map[string]any{}
	if !plan.EndpointURI.Equal(state.EndpointURI) {
		endpoint["uri"] = plan.EndpointURI.ValueString()
	}
	if !plan.AuthenticationType.Equal(state.AuthenticationType) || !plan.AuthenticationProperties.Equal(state.AuthenticationProperties) {
		endpoint["authentication"] = authBody(ctx, &plan, &resp.Diagnostics)
	}
	if !plan.AllowedHeaders.Equal(state.AllowedHeaders) {
		endpoint["allowedHeaders"] = nonNil(tfutil.Strings(ctx, plan.AllowedHeaders, &resp.Diagnostics))
	}
	if !plan.AllowedParameters.Equal(state.AllowedParameters) {
		endpoint["allowedParameters"] = nonNil(tfutil.Strings(ctx, plan.AllowedParameters, &resp.Diagnostics))
	}
	if len(endpoint) > 0 {
		body["endpoint"] = endpoint
	}
	if !plan.Rules.Equal(state.Rules) {
		rb := rulesBody(ctx, &plan, &resp.Diagnostics)
		if rb == nil {
			rb = map[string]any{}
		}
		body["rule"] = rb
	}
	if !plan.PasswordSharingFormat.Equal(state.PasswordSharingFormat) || !plan.PasswordSharingCert.Equal(state.PasswordSharingCert) || !plan.Attributes.Equal(state.Attributes) {
		typeSpecific(ctx, &plan, body, &resp.Diagnostics)
	}
	if len(body) > 0 {
		if _, err := r.client.Patch(ctx, r.item(&plan), body, nil); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update action", err)
			return
		}
	}
	r.setActive(ctx, &plan, state.Active.ValueBool(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *actionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.item(&state)); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete action", err)
	}
}

func (r *actionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	typ, id, ok := strings.Cut(req.ID, "/")
	if _, valid := actionTypes[typ]; !ok || !valid {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <type>/<id> with type one of %s, got %q", strings.Join(tfutil.SortedKeys(actionTypes), ", "), req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), typ)...)
}

func (r *actionResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	var a map[string]any
	if err := r.client.Get(ctx, r.item(m), nil, &a); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "ACTION-60007", "ACTION-60009") {
			return false
		}
		tfutil.AddAPIError(diags, "read action", err)
		return false
	}
	importing := m.Name.IsNull()
	m.Name = tfutil.GetString(a, "name")
	m.Description = tfutil.GetStringOrEmpty(a, "description")
	m.Version = tfutil.GetStringOrEmpty(a, "version")
	m.Active = types.BoolValue(tfutil.GetStringOrEmpty(a, "status").ValueString() == "ACTIVE")
	ep := tfutil.GetMap(a, "endpoint")
	m.EndpointURI = tfutil.GetString(ep, "uri")
	m.AuthenticationType = tfutil.GetString(tfutil.GetMap(ep, "authentication"), "type")
	m.AllowedHeaders = tfutil.SetOfStrings(tfutil.GetStrings(ep, "allowedHeaders"), false)
	m.AllowedParameters = tfutil.SetOfStrings(tfutil.GetStrings(ep, "allowedParameters"), false)

	rules := []ruleModel{}
	for _, rl := range tfutil.GetObjects(tfutil.GetMap(a, "rule"), "rules") {
		exprs := []expressionModel{}
		for _, e := range tfutil.GetObjects(rl, "expressions") {
			exprs = append(exprs, expressionModel{Field: tfutil.GetString(e, "field"), Operator: tfutil.GetString(e, "operator"), Value: tfutil.GetString(e, "value")})
		}
		rules = append(rules, ruleModel{Expressions: tfutil.ListFrom(ctx, expressionTypes, exprs, diags)})
	}
	if len(rules) == 0 && (m.Rules.IsNull() || importing) {
		m.Rules = types.ListNull(types.ObjectType{AttrTypes: ruleTypes})
	} else {
		m.Rules = tfutil.ListFrom(ctx, ruleTypes, rules, diags)
	}
	if ps := tfutil.GetMap(a, "passwordSharing"); ps != nil {
		m.PasswordSharingFormat = tfutil.GetString(ps, "format")
		if c := tfutil.GetString(ps, "certificate"); tfutil.Known(c) && c.ValueString() != "" && (m.PasswordSharingCert.IsNull() || importing) {
			m.PasswordSharingCert = c
		}
	}
	if attrs := tfutil.GetStrings(a, "attributes"); len(attrs) > 0 || !m.Attributes.IsNull() {
		m.Attributes = tfutil.SetOfStrings(attrs, m.Attributes.IsNull() && len(attrs) == 0)
	}
	return true
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
