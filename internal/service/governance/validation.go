package governance

import (
	"context"
	"net/url"

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
	_ resource.ResourceWithConfigure      = (*validationResource)(nil)
	_ resource.ResourceWithImportState    = (*validationResource)(nil)
	_ resource.ResourceWithValidateConfig = (*validationResource)(nil)
)

// NewValidationRulesResource returns the wso2is_validation_rules resource.
func NewValidationRulesResource() resource.Resource { return &validationResource{} }

type validationResource struct{ client *client.Client }

type validationModel struct {
	ID               types.String `tfsdk:"id"`
	Field            types.String `tfsdk:"field"`
	Rules            types.Set    `tfsdk:"rules"`
	Regex            types.String `tfsdk:"regex"`
	RestoreOnDestroy types.Bool   `tfsdk:"restore_on_destroy"`
}

type ruleModel struct {
	Validator  types.String `tfsdk:"validator"`
	Properties types.Map    `tfsdk:"properties"`
}

var ruleTypes = map[string]attr.Type{"validator": types.StringType, "properties": types.MapType{ElemType: types.StringType}}

func (r *validationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_validation_rules"
}

func (r *validationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets the validation rules for usernames or passwords. In the Console, these rules are under **Login & Registration > Password Validation** and **Username Validation**. Use either rule validators or a single regular expression. When you destroy the resource, the provider sets the field's rules back to the server defaults, unless `restore_on_destroy = false`. To import, use the field name.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `field`."),
			"field": schema.StringAttribute{
				MarkdownDescription: "`password` or `username`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("password", "username")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"rules": schema.SetNestedAttribute{
				MarkdownDescription: "Rule validators, for example `LengthValidator` (`min.length`, `max.length`), `NumeralValidator`, `UpperCaseValidator`, `LowerCaseValidator`, " +
					"`SpecialCharacterValidator` (`min.length`), `UniqueCharacterValidator` (`min.unique.character`), `RepeatedCharacterValidator` (`max.consecutive.character`), " +
					"`AlphanumericValidator` (`enable.validator`, `enable.special.characters`), `EmailFormatValidator` (`enable.validator`).",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"validator":  schema.StringAttribute{MarkdownDescription: "Validator name.", Required: true},
					"properties": schema.MapAttribute{MarkdownDescription: "Validator properties.", Required: true, ElementType: types.StringType},
				}},
			},
			"regex": schema.StringAttribute{
				MarkdownDescription: "A JavaScript regular expression that the value must match (the server's `JsRegExValidator`). You cannot use it together with `rules`. Only passwords accept a regular expression. The server rejects one for `username`.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("rules")), stringvalidator.AtLeastOneOf(path.MatchRoot("rules"))},
			},
			"restore_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Revert the field's rules to the server defaults on destroy. The default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *validationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m validationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if m.Field.ValueString() == "username" && tfutil.Known(m.Regex) {
		resp.Diagnostics.AddAttributeError(path.Root("regex"), "Regular expression not supported for usernames",
			"The server does not accept JsRegExValidator for the username field (IVM-60022). Use rules, for example LengthValidator and AlphanumericValidator.")
	}
}

func (r *validationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *validationResource) put(ctx context.Context, m *validationModel, diags *diag.Diagnostics) {
	body := map[string]any{}
	if tfutil.Known(m.Regex) {
		body["regEx"] = []any{map[string]any{"validator": "JsRegExValidator", "properties": []any{map[string]any{"key": "regex", "value": m.Regex.ValueString()}}}}
	} else {
		rules, _ := tfutil.ElementsAs[ruleModel](ctx, m.Rules, diags)
		list := []any{}
		for _, rl := range rules {
			props := []any{}
			p := tfutil.MapOfStrings(ctx, rl.Properties, diags)
			for _, k := range tfutil.SortedKeys(p) {
				props = append(props, map[string]any{"key": k, "value": p[k]})
			}
			list = append(list, map[string]any{"validator": rl.Validator.ValueString(), "properties": props})
		}
		body["rules"] = list
	}
	if _, err := r.client.Put(ctx, r.client.ServerPath("/validation-rules/"+url.PathEscape(m.Field.ValueString())), body, nil); err != nil {
		tfutil.AddAPIError(diags, "update validation rules", err)
	}
}

func (r *validationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan validationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.Field
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *validationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state validationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *validationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan validationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.Field
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *validationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state validationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !state.RestoreOnDestroy.ValueBool() {
		return
	}
	if _, err := r.client.Post(ctx, r.client.ServerPath("/validation-rules/revert"), map[string]any{"fields": []string{state.Field.ValueString()}}, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "revert validation rules", err)
	}
}

func (r *validationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("field"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("restore_on_destroy"), true)...)
}

func (r *validationResource) read(ctx context.Context, m *validationModel, diags *diag.Diagnostics) {
	var cfg map[string]any
	if err := r.client.Get(ctx, r.client.ServerPath("/validation-rules/"+url.PathEscape(m.Field.ValueString())), nil, &cfg); err != nil {
		tfutil.AddAPIError(diags, "read validation rules", err)
		return
	}
	m.Regex = types.StringNull()
	for _, re := range tfutil.GetObjects(cfg, "regEx") {
		for _, p := range tfutil.GetObjects(re, "properties") {
			if p["key"] == "regex" {
				m.Regex = tfutil.GetString(p, "value")
			}
		}
	}
	rules := []ruleModel{}
	for _, rl := range tfutil.GetObjects(cfg, "rules") {
		props := map[string]string{}
		for _, p := range tfutil.GetObjects(rl, "properties") {
			k, _ := p["key"].(string)
			v, _ := p["value"].(string)
			props[k] = v
		}
		rules = append(rules, ruleModel{Validator: tfutil.GetString(rl, "validator"), Properties: tfutil.MapValueOfStrings(props, false)})
	}
	if len(rules) == 0 && m.Rules.IsNull() {
		m.Rules = types.SetNull(types.ObjectType{AttrTypes: ruleTypes})
	} else if m.Regex.IsNull() {
		m.Rules = tfutil.SetFrom(ctx, ruleTypes, rules, diags)
	} else {
		m.Rules = types.SetNull(types.ObjectType{AttrTypes: ruleTypes})
	}
	if m.RestoreOnDestroy.IsNull() {
		m.RestoreOnDestroy = types.BoolValue(true)
	}
}
