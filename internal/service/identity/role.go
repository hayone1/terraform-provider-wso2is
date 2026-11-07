package identity

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*roleResource)(nil)
	_ resource.ResourceWithImportState = (*roleResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*roleResource)(nil)
)

// NewRoleResource returns the wso2is_role resource.
func NewRoleResource() resource.Resource { return &roleResource{} }

type roleResource struct{ client *client.Client }

type roleModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	AudienceType types.String `tfsdk:"audience_type"`
	AudienceID   types.String `tfsdk:"audience_id"`
	AudienceName types.String `tfsdk:"audience_name"`
	Permissions  types.Set    `tfsdk:"permissions"`
	Shared       types.Bool   `tfsdk:"shared"`
}

func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a role and the API scopes (permissions) that it grants. A role belongs either to the organization or to one application. An application can only have its own roles when it sets `associated_roles.allowed_audience = \"APPLICATION\"`. Assign users and groups to the role with `wso2is_role_assignment`. To import a role, use its ID.",
		Attributes: map[string]schema.Attribute{
			"id":   tfutil.IDAttribute("Role ID."),
			"name": schema.StringAttribute{MarkdownDescription: "Role name, unique within its audience.", Required: true},
			"audience_type": schema.StringAttribute{
				MarkdownDescription: "`organization` (default) or `application`. Changing it replaces the role.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("organization"),
				Validators:          []validator.String{stringvalidator.OneOf("organization", "application")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"audience_id": schema.StringAttribute{
				MarkdownDescription: "Application ID for application roles. For organization roles it is computed (the organization ID). Changing it replaces the role.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString(), stringplanmodifier.RequiresReplaceIfConfigured()},
			},
			"audience_name": tfutil.ComputedString("Name of the organization or application the role belongs to."),
			"permissions": schema.SetAttribute{
				MarkdownDescription: "Scope names granted by the role. For application roles they must belong to APIs the application is authorized for.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			},
			"shared": schema.BoolAttribute{
				MarkdownDescription: "Whether the role is shared from a parent organization.",
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool()},
			},
		},
	}
}

func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *roleResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if plan.AudienceType.ValueString() == "application" && plan.AudienceID.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("audience_id"), "Missing application ID", "audience_id is required when audience_type is \"application\".")
	}
}

func permissions(ctx context.Context, s types.Set, diags *diag.Diagnostics) []client.RolePermission {
	names := tfutil.Strings(ctx, s, diags)
	out := make([]client.RolePermission, 0, len(names))
	for _, n := range names {
		out = append(out, client.RolePermission{Value: n})
	}
	return out
}

func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	role := &client.Role{DisplayName: plan.Name.ValueString(), Permissions: permissions(ctx, plan.Permissions, &resp.Diagnostics)}
	// For organization roles the audience is implied; sending the type without the
	// organization ID is rejected ("Given Organization id: null is invalid").
	if plan.AudienceType.ValueString() == "application" || tfutil.Known(plan.AudienceID) {
		role.Audience = &client.RoleAudience{Type: plan.AudienceType.ValueString(), Value: plan.AudienceID.ValueString()}
	}
	out, err := r.client.CreateRole(ctx, role)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create role", err)
		return
	}
	plan.ID = types.StringValue(out.ID)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Role not found after creation", out.ID)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleModel
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

func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state roleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ops []client.SCIMPatchOperation
	if !plan.Name.Equal(state.Name) {
		ops = append(ops, client.SCIMPatchOperation{Op: "replace", Value: map[string]any{"displayName": plan.Name.ValueString()}})
	}
	if !plan.Permissions.Equal(state.Permissions) {
		ops = append(ops, client.SCIMPatchOperation{Op: "replace", Path: "permissions", Value: permissions(ctx, plan.Permissions, &resp.Diagnostics)})
	}
	if err := r.client.PatchRole(ctx, state.ID.ValueString(), ops); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update role", err)
		return
	}
	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Role not found after update", plan.ID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteRole(ctx, state.ID.ValueString()); err != nil && !client.IsSCIMNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete role", err)
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *roleResource) read(ctx context.Context, m *roleModel, diags *diag.Diagnostics) bool {
	role, err := r.client.GetRole(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsSCIMNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read role", err)
		return false
	}
	m.Name = types.StringValue(role.DisplayName)
	if role.Audience != nil {
		m.AudienceType = types.StringValue(strings.ToLower(role.Audience.Type))
		m.AudienceID = types.StringValue(role.Audience.Value)
		m.AudienceName = types.StringValue(role.Audience.Display)
	}
	perms := make([]string, 0, len(role.Permissions))
	for _, p := range role.Permissions {
		perms = append(perms, p.Value)
	}
	sort.Strings(perms)
	m.Permissions = tfutil.SetOfStrings(perms, false)
	shared := false
	for _, p := range role.Properties {
		if p.Name == "isSharedRole" {
			shared = p.Value == "true"
		}
	}
	m.Shared = types.BoolValue(shared)
	return true
}

// ---- wso2is_role_assignment ----

var (
	_ resource.ResourceWithConfigure   = (*roleAssignmentResource)(nil)
	_ resource.ResourceWithImportState = (*roleAssignmentResource)(nil)
)

// NewRoleAssignmentResource returns the wso2is_role_assignment resource.
func NewRoleAssignmentResource() resource.Resource { return &roleAssignmentResource{} }

type roleAssignmentResource struct{ client *client.Client }

type roleAssignmentModel struct {
	ID       types.String `tfsdk:"id"`
	RoleID   types.String `tfsdk:"role_id"`
	UserIDs  types.Set    `tfsdk:"user_ids"`
	GroupIDs types.Set    `tfsdk:"group_ids"`
}

func (r *roleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_assignment"
}

func (r *roleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the complete list of users and groups that have a role. The provider removes any assignment that is not in the list. Use only one `wso2is_role_assignment` for each role. Every user also has the built-in `everyone` role, which this resource does not manage. To import, use the role ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `role_id`."),
			"role_id": schema.StringAttribute{
				MarkdownDescription: "Role ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of users assigned the role.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			},
			"group_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of groups assigned the role.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
			},
		},
	}
}

func (r *roleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func members(ms []client.SCIMMember) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Value)
	}
	return out
}

func (r *roleAssignmentResource) apply(ctx context.Context, roleID string, users, groups []string, diags *diag.Diagnostics) {
	role, err := r.client.GetRole(ctx, roleID)
	if err != nil {
		if client.IsSCIMNotFound(err) && users == nil && groups == nil {
			return
		}
		tfutil.AddAPIError(diags, "read role", err)
		return
	}
	var ops []client.SCIMPatchOperation
	for _, kind := range []struct {
		name string
		want []string
		have []string
	}{{"users", users, members(role.Users)}, {"groups", groups, members(role.Groups)}} {
		added, removed := diff(kind.want, kind.have)
		if len(added) > 0 {
			vals := make([]client.SCIMMember, 0, len(added))
			for _, id := range added {
				vals = append(vals, client.SCIMMember{Value: id})
			}
			ops = append(ops, client.SCIMPatchOperation{Op: "add", Path: kind.name, Value: vals})
		}
		for _, id := range removed {
			ops = append(ops, client.SCIMPatchOperation{Op: "remove", Path: fmt.Sprintf("%s[value eq %q]", kind.name, id)})
		}
	}
	if err := r.client.PatchRole(ctx, roleID, ops); err != nil {
		tfutil.AddAPIError(diags, "update role assignments", err)
	}
}

func (r *roleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.createOrUpdate(ctx, req.Plan.Get, &resp.State, &resp.Diagnostics)
}

func (r *roleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.createOrUpdate(ctx, req.Plan.Get, &resp.State, &resp.Diagnostics)
}

func (r *roleAssignmentResource) createOrUpdate(ctx context.Context, get func(context.Context, any) diag.Diagnostics, state interface {
	Set(context.Context, any) diag.Diagnostics
}, diags *diag.Diagnostics) {
	var plan roleAssignmentModel
	diags.Append(get(ctx, &plan)...)
	if diags.HasError() {
		return
	}
	users := tfutil.Strings(ctx, plan.UserIDs, diags)
	groups := tfutil.Strings(ctx, plan.GroupIDs, diags)
	if users == nil {
		users = []string{}
	}
	if groups == nil {
		groups = []string{}
	}
	r.apply(ctx, plan.RoleID.ValueString(), users, groups, diags)
	if diags.HasError() {
		return
	}
	plan.ID = plan.RoleID
	if !r.read(ctx, &plan, diags) && !diags.HasError() {
		diags.AddError("Role not found", plan.RoleID.ValueString())
	}
	diags.Append(state.Set(ctx, &plan)...)
}

func (r *roleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state roleAssignmentModel
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

func (r *roleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state roleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, state.RoleID.ValueString(), nil, nil, &resp.Diagnostics)
}

func (r *roleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role_id"), req.ID)...)
}

func (r *roleAssignmentResource) read(ctx context.Context, m *roleAssignmentModel, diags *diag.Diagnostics) bool {
	role, err := r.client.GetRole(ctx, m.RoleID.ValueString())
	if err != nil {
		if client.IsSCIMNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read role", err)
		return false
	}
	users, groups := members(role.Users), members(role.Groups)
	sort.Strings(users)
	sort.Strings(groups)
	m.UserIDs = tfutil.SetOfStrings(users, false)
	m.GroupIDs = tfutil.SetOfStrings(groups, false)
	return true
}
