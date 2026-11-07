// Package workflow implements approval workflows and their associations.
package workflow

import (
	"context"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*workflowResource)(nil)
	_ resource.ResourceWithImportState = (*workflowResource)(nil)
)

// NewResource returns the wso2is_workflow resource.
func NewResource() resource.Resource { return &workflowResource{} }

type workflowResource struct{ client *client.Client }

type workflowModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	Engine                 types.String `tfsdk:"engine"`
	Template               types.String `tfsdk:"template"`
	Steps                  types.List   `tfsdk:"steps"`
	InitiatorNotifications types.Object `tfsdk:"initiator_notifications"`
	ApproverNotifications  types.Object `tfsdk:"approver_notifications"`
}

type stepModel struct {
	RoleIDs types.Set `tfsdk:"role_ids"`
	UserIDs types.Set `tfsdk:"user_ids"`
}

type notificationsModel struct {
	Channels types.Set `tfsdk:"channels"`
	Events   types.Set `tfsdk:"events"`
}

var (
	stepTypes          = map[string]attr.Type{"role_ids": types.SetType{ElemType: types.StringType}, "user_ids": types.SetType{ElemType: types.StringType}}
	notificationsTypes = map[string]attr.Type{"channels": types.SetType{ElemType: types.StringType}, "events": types.SetType{ElemType: types.StringType}}
)

func (r *workflowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow"
}

func notificationsAttr(desc string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: desc,
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"channels": schema.SetAttribute{MarkdownDescription: "Notification channels, for example `email`.", Required: true, ElementType: types.StringType},
			"events":   schema.SetAttribute{MarkdownDescription: "Events that trigger a notification.", Required: true, ElementType: types.StringType},
		},
	}
}

func (r *workflowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an approval workflow. A workflow is a sequence of approval steps. Users with given roles, or given users, approve each step. Attach the workflow to user and role operations with `wso2is_workflow_association`.\n\nWhile a workflow is attached to an operation, such as `ADD_USER`, the operation only finishes after someone approves it. Terraform resources that perform the operation, such as `wso2is_user`, then fail, because the object does not exist yet. To import, use the workflow ID.",
		Attributes: map[string]schema.Attribute{
			"id":          tfutil.IDAttribute("Workflow ID."),
			"name":        schema.StringAttribute{MarkdownDescription: "Workflow name.", Required: true},
			"description": tfutil.String("Description.", ""),
			"engine":      schema.StringAttribute{MarkdownDescription: "Workflow engine. The default is `WorkflowEngine`.", Optional: true, Computed: true, Default: stringdefault.StaticString("WorkflowEngine")},
			"template":    schema.StringAttribute{MarkdownDescription: "Workflow template. The default is `MultiStepApprovalTemplate`.", Optional: true, Computed: true, Default: stringdefault.StaticString("MultiStepApprovalTemplate")},
			"steps": schema.ListNestedAttribute{
				MarkdownDescription: "Approval steps, in order. A step is approved by any user holding one of `role_ids` or listed in `user_ids`.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"role_ids": schema.SetAttribute{MarkdownDescription: "IDs of roles whose users can approve.", Optional: true, ElementType: types.StringType},
					"user_ids": schema.SetAttribute{MarkdownDescription: "IDs of users who can approve.", Optional: true, ElementType: types.StringType},
				}},
			},
			"initiator_notifications": notificationsAttr("Notifications to the user who started the operation (events such as `onApproval`, `onRejection`)."),
			"approver_notifications":  notificationsAttr("Notifications to approvers (events such as `onAssignment`, `onRelease`)."),
		},
	}
}

func (r *workflowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func notifications(ctx context.Context, obj types.Object, diags *diag.Diagnostics) map[string]any {
	n, ok := tfutil.ObjectAs[notificationsModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	return map[string]any{"channels": tfutil.Strings(ctx, n.Channels, diags), "events": tfutil.Strings(ctx, n.Events, diags)}
}

func workflowBody(ctx context.Context, m *workflowModel, diags *diag.Diagnostics) map[string]any {
	steps, _ := tfutil.ElementsAs[stepModel](ctx, m.Steps, diags)
	list := []any{}
	for i, s := range steps {
		opts := []any{}
		if ids := tfutil.Strings(ctx, s.RoleIDs, diags); len(ids) > 0 {
			opts = append(opts, map[string]any{"entity": "roles", "values": ids})
		}
		if ids := tfutil.Strings(ctx, s.UserIDs, diags); len(ids) > 0 {
			opts = append(opts, map[string]any{"entity": "users", "values": ids})
		}
		list = append(list, map[string]any{"step": i + 1, "options": opts})
	}
	tmpl := map[string]any{"name": m.Template.ValueString(), "steps": list}
	if n := notifications(ctx, m.ApproverNotifications, diags); n != nil {
		tmpl["notificationsForApprovers"] = n
	}
	b := map[string]any{"name": m.Name.ValueString(), "description": m.Description.ValueString(), "engine": m.Engine.ValueString(), "template": tmpl}
	if n := notifications(ctx, m.InitiatorNotifications, diags); n != nil {
		b["notificationsForInitiator"] = n
	}
	return b
}

func (r *workflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workflowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if _, err := r.client.Post(ctx, r.client.ServerPath("/workflows"), workflowBody(ctx, &plan, &resp.Diagnostics), &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create workflow", err)
		return
	}
	id, _ := out["id"].(string)
	plan.ID = types.StringValue(id)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workflowModel
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

func (r *workflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state workflowModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	if _, err := r.client.Put(ctx, r.client.ServerPath("/workflows/"+url.PathEscape(state.ID.ValueString())), workflowBody(ctx, &plan, &resp.Diagnostics), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update workflow", err)
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workflowModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.client.ServerPath("/workflows/"+url.PathEscape(state.ID.ValueString()))); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete workflow", err)
	}
}

func (r *workflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func readNotifications(ctx context.Context, n map[string]any, prior types.Object, diags *diag.Diagnostics) types.Object {
	if n == nil || (len(tfutil.GetStrings(n, "channels")) == 0 && len(tfutil.GetStrings(n, "events")) == 0) {
		if prior.IsNull() {
			return types.ObjectNull(notificationsTypes)
		}
	}
	return tfutil.ObjectFrom(ctx, notificationsTypes, notificationsModel{
		Channels: tfutil.SetOfStrings(tfutil.GetStrings(n, "channels"), false),
		Events:   tfutil.SetOfStrings(tfutil.GetStrings(n, "events"), false),
	}, diags)
}

func (r *workflowResource) read(ctx context.Context, m *workflowModel, diags *diag.Diagnostics) bool {
	var w map[string]any
	if err := r.client.Get(ctx, r.client.ServerPath("/workflows/"+url.PathEscape(m.ID.ValueString())), nil, &w); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "WF-60002", "WF-10001") {
			return false
		}
		tfutil.AddAPIError(diags, "read workflow", err)
		return false
	}
	m.Name = tfutil.GetString(w, "name")
	m.Description = tfutil.GetStringOrEmpty(w, "description")
	m.Engine = tfutil.GetString(w, "engine")
	tmpl := tfutil.GetMap(w, "template")
	m.Template = tfutil.GetString(tmpl, "name")
	stepsRaw := tfutil.GetObjects(tmpl, "steps")
	sort.SliceStable(stepsRaw, func(i, j int) bool {
		a, _ := stepsRaw[i]["step"].(float64)
		b, _ := stepsRaw[j]["step"].(float64)
		return a < b
	})
	steps := []stepModel{}
	for _, s := range stepsRaw {
		var roles, users []string
		for _, o := range tfutil.GetObjects(s, "options") {
			switch o["entity"] {
			case "roles":
				roles = append(roles, tfutil.GetStrings(o, "values")...)
			case "users":
				users = append(users, tfutil.GetStrings(o, "values")...)
			}
		}
		steps = append(steps, stepModel{RoleIDs: tfutil.SetOfStrings(roles, true), UserIDs: tfutil.SetOfStrings(users, true)})
	}
	m.Steps = tfutil.ListFrom(ctx, stepTypes, steps, diags)
	m.InitiatorNotifications = readNotifications(ctx, tfutil.GetMap(w, "notificationsForInitiator"), m.InitiatorNotifications, diags)
	m.ApproverNotifications = readNotifications(ctx, tfutil.GetMap(tmpl, "notificationsForApprovers"), m.ApproverNotifications, diags)
	return true
}

// ---- wso2is_workflow_association ----

var (
	_ resource.ResourceWithConfigure   = (*associationResource)(nil)
	_ resource.ResourceWithImportState = (*associationResource)(nil)
)

// NewAssociationResource returns the wso2is_workflow_association resource.
func NewAssociationResource() resource.Resource { return &associationResource{} }

type associationResource struct{ client *client.Client }

type associationModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Operation  types.String `tfsdk:"operation"`
	WorkflowID types.String `tfsdk:"workflow_id"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Rules      types.List   `tfsdk:"rules"`
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

func (r *associationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow_association"
}

func (r *associationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Makes a user or role operation wait for approval through a `wso2is_workflow`. To import, use the association ID.",
		Attributes: map[string]schema.Attribute{
			"id":   tfutil.IDAttribute("Association ID."),
			"name": schema.StringAttribute{MarkdownDescription: "Association name.", Required: true},
			"operation": schema.StringAttribute{
				MarkdownDescription: "Operation: `ADD_USER`, `SELF_REGISTER_USER`, `DELETE_USER`, `UPDATE_ROLES_OF_USERS`, `ADD_ROLE`, `DELETE_ROLE`, `UPDATE_ROLE_NAME`, `UPDATE_USERS_OF_ROLES`, `DELETE_USER_CLAIMS` or `UPDATE_USER_CLAIMS`.",
				Required:            true,
				Validators: []validator.String{stringvalidator.OneOf("ADD_USER", "SELF_REGISTER_USER", "DELETE_USER", "UPDATE_ROLES_OF_USERS", "ADD_ROLE", "DELETE_ROLE",
					"UPDATE_ROLE_NAME", "UPDATE_USERS_OF_ROLES", "DELETE_USER_CLAIMS", "UPDATE_USER_CLAIMS")},
			},
			"workflow_id": schema.StringAttribute{MarkdownDescription: "Workflow ID.", Required: true},
			"enabled":     schema.BoolAttribute{MarkdownDescription: "Whether the association is active. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"rules": schema.ListNestedAttribute{
				MarkdownDescription: "Rules that decide when an operation needs approval. Approval is needed when any rule matches. A rule matches when all of its expressions match. Without rules, every operation needs approval.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"expressions": schema.ListNestedAttribute{
						Required:            true,
						MarkdownDescription: "Expressions, all of which must match.",
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"field":    schema.StringAttribute{MarkdownDescription: "Field, for example `role` or `user.groups`.", Required: true},
							"operator": schema.StringAttribute{MarkdownDescription: "`equals` or `notEquals`.", Required: true},
							"value":    schema.StringAttribute{MarkdownDescription: "Value to compare with.", Required: true},
						}},
					},
				}},
			},
		},
	}
}

func (r *associationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func associationBody(ctx context.Context, m *associationModel, diags *diag.Diagnostics) map[string]any {
	b := map[string]any{"associationName": m.Name.ValueString(), "operation": m.Operation.ValueString(), "workflowId": m.WorkflowID.ValueString(), "isEnabled": m.Enabled.ValueBool()}
	if rules, ok := tfutil.ElementsAs[ruleModel](ctx, m.Rules, diags); ok && len(rules) > 0 {
		list := []any{}
		for _, rl := range rules {
			exprs, _ := tfutil.ElementsAs[expressionModel](ctx, rl.Expressions, diags)
			el := []any{}
			for _, e := range exprs {
				el = append(el, map[string]any{"field": e.Field.ValueString(), "operator": e.Operator.ValueString(), "value": e.Value.ValueString()})
			}
			list = append(list, map[string]any{"condition": "AND", "expressions": el})
		}
		b["rule"] = map[string]any{"condition": "OR", "rules": list}
	}
	return b
}

func (r *associationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan associationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if _, err := r.client.Post(ctx, r.client.ServerPath("/workflow-associations"), associationBody(ctx, &plan, &resp.Diagnostics), &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create workflow association", err)
		return
	}
	id, _ := out["id"].(string)
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	// The server ignores isEnabled=false on creation.
	if !plan.Enabled.ValueBool() {
		if _, err := r.client.Patch(ctx, r.client.ServerPath("/workflow-associations/"+url.PathEscape(id)), map[string]any{"isEnabled": false}, nil); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "disable workflow association", err)
			return
		}
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *associationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state associationModel
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

func (r *associationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state associationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	body := associationBody(ctx, &plan, &resp.Diagnostics)
	if _, ok := body["rule"]; !ok && !state.Rules.IsNull() {
		body["rule"] = map[string]any{}
	}
	if _, err := r.client.Patch(ctx, r.client.ServerPath("/workflow-associations/"+url.PathEscape(state.ID.ValueString())), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update workflow association", err)
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *associationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state associationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := r.client.ServerPath("/workflow-associations/" + url.PathEscape(state.ID.ValueString()))
	err := r.client.Delete(ctx, p)
	switch {
	case err == nil, client.IsNotFound(err):
	case client.HasCode(err, "WF-51005"):
		// The last association of a workflow cannot be deleted; it goes away with the
		// workflow. Disable it so it has no effect until then.
		if _, err := r.client.Patch(ctx, p, map[string]any{"isEnabled": false}, nil); err != nil && !client.IsNotFound(err) {
			tfutil.AddAPIError(&resp.Diagnostics, "disable workflow association", err)
			return
		}
		resp.Diagnostics.AddWarning("Workflow association disabled, not deleted",
			"WSO2 does not delete the last association of a workflow; it was disabled and is removed when its workflow is deleted.")
	default:
		tfutil.AddAPIError(&resp.Diagnostics, "delete workflow association", err)
	}
}

func (r *associationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *associationResource) read(ctx context.Context, m *associationModel, diags *diag.Diagnostics) bool {
	var a map[string]any
	if err := r.client.Get(ctx, r.client.ServerPath("/workflow-associations/"+url.PathEscape(m.ID.ValueString())), nil, &a); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "WF-60003", "WF-10002") {
			return false
		}
		tfutil.AddAPIError(diags, "read workflow association", err)
		return false
	}
	m.Name = tfutil.GetString(a, "associationName")
	m.Operation = tfutil.GetString(a, "operation")
	m.Enabled = tfutil.GetBool(a, "isEnabled", true)
	rules := []ruleModel{}
	for _, rl := range tfutil.GetObjects(tfutil.GetMap(a, "rule"), "rules") {
		exprs := []expressionModel{}
		for _, e := range tfutil.GetObjects(rl, "expressions") {
			exprs = append(exprs, expressionModel{Field: tfutil.GetString(e, "field"), Operator: tfutil.GetString(e, "operator"), Value: tfutil.GetString(e, "value")})
		}
		rules = append(rules, ruleModel{Expressions: tfutil.ListFrom(ctx, expressionTypes, exprs, diags)})
	}
	if len(rules) == 0 && m.Rules.IsNull() {
		m.Rules = types.ListNull(types.ObjectType{AttrTypes: ruleTypes})
	} else {
		m.Rules = tfutil.ListFrom(ctx, ruleTypes, rules, diags)
	}
	return true
}
