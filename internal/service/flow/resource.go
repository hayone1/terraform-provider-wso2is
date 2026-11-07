// Package flow implements the flow builder (registration, password recovery and
// invited user registration flows).
package flow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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

var flowTypes = []string{"REGISTRATION", "PASSWORD_RECOVERY", "INVITED_USER_REGISTRATION"}

var (
	_ resource.ResourceWithConfigure   = (*flowResource)(nil)
	_ resource.ResourceWithImportState = (*flowResource)(nil)
)

// NewResource returns the wso2is_flow resource.
func NewResource() resource.Resource { return &flowResource{} }

type flowResource struct{ client *client.Client }

type model struct {
	ID                types.String `tfsdk:"id"`
	FlowType          types.String `tfsdk:"flow_type"`
	Steps             types.String `tfsdk:"steps"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	CompletionConfigs types.Map    `tfsdk:"completion_configs"`
}

func (r *flowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flow"
}

func (r *flowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a flow that the Console's flow builder can also edit (WSO2 IS 7.2 and later). A flow is the self sign-up form and its steps, the password recovery flow, or the invited user registration flow. Use this resource to build a custom sign-up form.\n\nIn a flow, form fields map to claims, and executors do the work of each step. Examples of executors are `PasswordProvisioningExecutor`, `EmailOTPExecutor`, `UserProvisioningExecutor` and federated executors such as `GoogleExecutor`.\n\n`steps` holds the same JSON document that the Console stores (`GET /api/server/v1/flow?flowType=...`). You can design the flow in the Console and copy it with the `wso2is_flow` data source, or write it with `jsonencode`.\n\nWhen you destroy the resource, the provider turns the flow off and deletes it. By default, WSO2 IS 7.3.0 does not allow the API to delete the root organization's flow. In that case, the provider only turns the flow off. The server behaviour guide explains how to allow deletion. To import a flow, use its flow type.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `flow_type`."),
			"flow_type": schema.StringAttribute{
				MarkdownDescription: "`REGISTRATION`, `PASSWORD_RECOVERY` or `INVITED_USER_REGISTRATION`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf(flowTypes...)},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"steps": schema.StringAttribute{
				MarkdownDescription: "The steps of the flow, as a JSON array. Each step has an `id`, a `type` (such as `VIEW`, `REDIRECTION` or `END`), a `size` and a `position` (its place in the Console's editor), and `data`. The `data.components` list holds the step's content: `TYPOGRAPHY` text, and `FORM` blocks with `INPUT` fields and `BUTTON`s. Each field's `config.identifier` is a claim URI, or `password` or `otp`. Each button has an `action`, for example `{ type = \"EXECUTOR\", executor = { name = ... }, next = <step id or \"END\"> }`. The last step is `{ id = \"END\", type = \"END\" }`.",
				Required:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the flow replaces the older, built-in flow. For registration, this means that the flow builds the self sign-up page. The default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"completion_configs": schema.MapAttribute{
				MarkdownDescription: "Settings that apply when the flow finishes, for example `isEmailVerificationEnabled`, `isAutoLoginEnabled`, `isAccountLockOnCreationEnabled` and `isFlowCompletionNotificationEnabled`. The values are the strings `\"true\"` and `\"false\"`. The provider only manages the keys that you list.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *flowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *flowResource) put(ctx context.Context, m *model, diags *diag.Diagnostics) {
	var steps []any
	if err := json.Unmarshal([]byte(m.Steps.ValueString()), &steps); err != nil {
		diags.AddAttributeError(path.Root("steps"), "Invalid steps", "steps must be a JSON array: "+err.Error())
		return
	}
	if _, err := r.client.Put(ctx, r.client.ServerPath("/flow"), map[string]any{"flowType": m.FlowType.ValueString(), "steps": steps}, nil); err != nil {
		tfutil.AddAPIError(diags, "update flow", err)
	}
}

func (r *flowResource) patchConfig(ctx context.Context, m *model, enabled bool, diags *diag.Diagnostics) {
	body := map[string]any{"flowType": m.FlowType.ValueString(), "isEnabled": enabled}
	if cc := tfutil.MapOfStrings(ctx, m.CompletionConfigs, diags); len(cc) > 0 {
		body["flowCompletionConfigs"] = cc
	}
	if _, err := r.client.Patch(ctx, r.client.ServerPath("/flow/config"), body, nil); err != nil {
		tfutil.AddAPIError(diags, "update flow configuration", err)
	}
}

func (r *flowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.put(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.FlowType
	r.patchConfig(ctx, &plan, plan.Enabled.ValueBool(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *flowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

func (r *flowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !StepsEqual(plan.Steps.ValueString(), state.Steps.ValueString()) {
		r.put(ctx, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if !plan.Enabled.Equal(state.Enabled) || !plan.CompletionConfigs.Equal(state.CompletionConfigs) {
		r.patchConfig(ctx, &plan, plan.Enabled.ValueBool(), &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	plan.ID = state.ID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *flowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	disabled := state
	disabled.CompletionConfigs = types.MapNull(types.StringType)
	r.patchConfig(ctx, &disabled, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Do(ctx, client.Request{Method: http.MethodDelete, Path: r.client.ServerPath("/flow"), Query: url.Values{"flowType": {state.FlowType.ValueString()}}})
	switch {
	case err == nil, client.IsNotFound(err):
	case client.StatusCode(err) == http.StatusForbidden && r.client.OrganizationID() == "":
		// WSO2 IS 7.3.0 has no access control rule for DELETE on the root organization's
		// /api/server/v1/flow, so the request is always forbidden.
		resp.Diagnostics.AddWarning("Flow disabled, not deleted",
			"The server does not permit deleting the "+state.FlowType.ValueString()+" flow (WSO2 IS 7.3.0 has no access rule for it), so it was disabled and its steps left in place. "+
				"To allow deletion, add to deployment.toml:\n\n[[resource.access_control]]\ncontext = \"(.*)/api/server/v1/flow(.*)\"\nsecure = true\nhttp_method = \"DELETE\"\nscopes = [\"internal_flow_update\"]")
	default:
		tfutil.AddAPIError(&resp.Diagnostics, "delete flow", err)
	}
}

func (r *flowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("flow_type"), req.ID)...)
}

func (r *flowResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	steps, cfg, ok := readFlow(ctx, r.client, m.FlowType.ValueString(), diags)
	if !ok {
		return false
	}
	if steps == "" {
		// The flow was deleted outside Terraform (the server reports an empty flow).
		return false
	}
	if !tfutil.Known(m.Steps) || !StepsEqual(m.Steps.ValueString(), steps) {
		m.Steps = types.StringValue(steps)
	}
	m.Enabled = tfutil.GetBool(cfg, "isEnabled", false)
	managed := m.CompletionConfigs.Elements()
	out := map[string]string{}
	for k, v := range tfutil.GetMap(cfg, "flowCompletionConfigs") {
		if _, ok := managed[k]; ok {
			s, _ := v.(string)
			out[k] = s
		}
	}
	m.CompletionConfigs = tfutil.MapValueOfStrings(out, m.CompletionConfigs.IsNull())
	return true
}

func readFlow(ctx context.Context, c *client.Client, flowType string, diags *diag.Diagnostics) (string, map[string]any, bool) {
	q := url.Values{"flowType": {flowType}}
	var flow struct {
		Steps json.RawMessage `json:"steps"`
	}
	if err := c.Get(ctx, c.ServerPath("/flow"), q, &flow); err != nil {
		tfutil.AddAPIError(diags, "read flow", err)
		return "", nil, false
	}
	var cfg map[string]any
	if err := c.Get(ctx, c.ServerPath("/flow/config"), q, &cfg); err != nil {
		tfutil.AddAPIError(diags, "read flow configuration", err)
		return "", nil, false
	}
	steps := string(flow.Steps)
	if steps == "null" || steps == "[]" {
		steps = ""
	}
	return steps, cfg, true
}

// StepsEqual compares flow documents the way the server stores them: numbers become
// floating point and empty arrays, empty objects and nulls are dropped.
func StepsEqual(a, b string) bool {
	var va, vb any
	if json.Unmarshal([]byte(a), &va) != nil || json.Unmarshal([]byte(b), &vb) != nil {
		return a == b
	}
	return reflect.DeepEqual(normalize(va), normalize(vb))
}

func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range t {
			n := normalize(e)
			if isEmpty(n) {
				continue
			}
			out[k] = n
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, e := range t {
			out = append(out, normalize(e))
		}
		return out
	}
	return v
}

func isEmpty(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

// ---- data source ----

var _ datasource.DataSourceWithConfigure = (*flowDataSource)(nil)

// NewDataSource returns the wso2is_flow data source.
func NewDataSource() datasource.DataSource { return &flowDataSource{} }

type flowDataSource struct{ client *client.Client }

type dsModel struct {
	FlowType          types.String `tfsdk:"flow_type"`
	Steps             types.String `tfsdk:"steps"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	CompletionConfigs types.Map    `tfsdk:"completion_configs"`
}

func (d *flowDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_flow"
}

func (d *flowDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads a flow and its settings. For example, you can copy a flow that you designed in the Console into a `wso2is_flow` resource with `steps = data.wso2is_flow.signup.steps`, or save it to a file with `local_file`.",
		Attributes: map[string]dsschema.Attribute{
			"flow_type":          dsschema.StringAttribute{Required: true, MarkdownDescription: "`REGISTRATION`, `PASSWORD_RECOVERY` or `INVITED_USER_REGISTRATION`.", Validators: []validator.String{stringvalidator.OneOf(flowTypes...)}},
			"steps":              dsschema.StringAttribute{Computed: true, MarkdownDescription: "JSON array of steps (empty when no flow is defined)."},
			"enabled":            dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the flow is enabled."},
			"completion_configs": dsschema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Flow completion settings."},
		},
	}
}

func (d *flowDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = tfutil.ConfigureDataSource(req, resp)
}

func (d *flowDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m dsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	steps, cfg, ok := readFlow(ctx, d.client, m.FlowType.ValueString(), &resp.Diagnostics)
	if !ok {
		return
	}
	m.Steps = types.StringValue(steps)
	m.Enabled = tfutil.GetBool(cfg, "isEnabled", false)
	out := map[string]string{}
	for k, v := range tfutil.GetMap(cfg, "flowCompletionConfigs") {
		s, _ := v.(string)
		out[k] = s
	}
	m.CompletionConfigs = tfutil.MapValueOfStrings(out, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
