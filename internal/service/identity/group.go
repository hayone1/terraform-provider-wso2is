package identity

import (
	"context"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*groupResource)(nil)
	_ resource.ResourceWithImportState = (*groupResource)(nil)
)

// NewGroupResource returns the wso2is_group resource.
func NewGroupResource() resource.Resource { return &groupResource{} }

type groupResource struct{ client *client.Client }

type groupModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	UserStore types.String `tfsdk:"user_store"`
}

func (r *groupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *groupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a group of users. Manage the members with `wso2is_group_members`, and the roles with `wso2is_role_assignment`. By default, the server requires group names to be 3 to 30 characters long, without spaces. To import a group, use its ID.",
		Attributes: map[string]schema.Attribute{
			"id":   tfutil.IDAttribute("Group ID."),
			"name": schema.StringAttribute{MarkdownDescription: "Group name, without the user store prefix.", Required: true},
			"user_store": schema.StringAttribute{
				MarkdownDescription: "User store domain. The default is `PRIMARY`. Changing it replaces the group.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("PRIMARY"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *groupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *groupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.client.CreateGroup(ctx, fullUsername(plan.UserStore.ValueString(), plan.Name.ValueString()), nil)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create group", err)
		return
	}
	plan.ID = types.StringValue(g.ID)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Group not found after creation", g.ID)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupModel
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

func (r *groupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// PATCH (not PUT, which drops the members).
	err := r.client.PatchGroup(ctx, state.ID.ValueString(), []client.SCIMPatchOperation{{
		Op: "replace", Value: map[string]any{"displayName": fullUsername(plan.UserStore.ValueString(), plan.Name.ValueString())},
	}})
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "rename group", err)
		return
	}
	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Group not found after update", plan.ID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteGroup(ctx, state.ID.ValueString()); err != nil && !client.IsSCIMNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete group", err)
	}
}

func (r *groupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *groupResource) read(ctx context.Context, m *groupModel, diags *diag.Diagnostics) bool {
	g, err := r.client.GetGroup(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsSCIMNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read group", err)
		return false
	}
	store, name := splitUsername(g.DisplayName)
	m.Name = types.StringValue(name)
	if m.UserStore.IsNull() || !strings.EqualFold(m.UserStore.ValueString(), store) {
		m.UserStore = types.StringValue(store)
	}
	return true
}

// ---- wso2is_group_members ----

var (
	_ resource.ResourceWithConfigure   = (*groupMembersResource)(nil)
	_ resource.ResourceWithImportState = (*groupMembersResource)(nil)
)

// NewGroupMembersResource returns the wso2is_group_members resource.
func NewGroupMembersResource() resource.Resource { return &groupMembersResource{} }

type groupMembersResource struct{ client *client.Client }

type groupMembersModel struct {
	ID      types.String `tfsdk:"id"`
	GroupID types.String `tfsdk:"group_id"`
	UserIDs types.Set    `tfsdk:"user_ids"`
}

func (r *groupMembersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_members"
}

func (r *groupMembersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the complete list of members of a group. The provider removes any user who is not in the list. Use only one `wso2is_group_members` for each group. To import, use the group ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `group_id`."),
			"group_id": schema.StringAttribute{
				MarkdownDescription: "Group ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_ids": schema.SetAttribute{MarkdownDescription: "IDs of the member users.", Required: true, ElementType: types.StringType},
		},
	}
}

func (r *groupMembersResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *groupMembersResource) apply(ctx context.Context, groupID string, want []string, diags *diag.Diagnostics) {
	g, err := r.client.GetGroup(ctx, groupID)
	if err != nil {
		if client.IsSCIMNotFound(err) && want == nil {
			// Removing the members of a deleted group: nothing to do.
			return
		}
		tfutil.AddAPIError(diags, "read group", err)
		return
	}
	have := make([]string, 0, len(g.Members))
	for _, mem := range g.Members {
		have = append(have, mem.Value)
	}
	added, removed := diff(want, have)
	var ops []client.SCIMPatchOperation
	if len(added) > 0 {
		// WSO2 rejects member additions without the member's username as `display`.
		members := make([]client.SCIMMember, 0, len(added))
		for _, id := range added {
			u, err := r.client.GetUser(ctx, id)
			if err != nil {
				tfutil.AddAPIError(diags, "read member user "+id, err)
				return
			}
			userName, _ := u["userName"].(string)
			members = append(members, client.SCIMMember{Value: id, Display: userName})
		}
		ops = append(ops, client.SCIMPatchOperation{Op: "add", Value: map[string]any{"members": members}})
	}
	for _, id := range removed {
		ops = append(ops, client.SCIMPatchOperation{Op: "remove", Path: `members[value eq "` + id + `"]`})
	}
	if err := r.client.PatchGroup(ctx, groupID, ops); err != nil {
		tfutil.AddAPIError(diags, "update group members", err)
	}
}

func (r *groupMembersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMembersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, plan.GroupID.ValueString(), nonNilStrings(tfutil.Strings(ctx, plan.UserIDs, &resp.Diagnostics)), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.GroupID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupMembersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMembersModel
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

func (r *groupMembersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan groupMembersModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, plan.GroupID.ValueString(), nonNilStrings(tfutil.Strings(ctx, plan.UserIDs, &resp.Diagnostics)), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.GroupID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *groupMembersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMembersModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, state.GroupID.ValueString(), nil, &resp.Diagnostics)
}

func (r *groupMembersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_id"), req.ID)...)
}

func (r *groupMembersResource) read(ctx context.Context, m *groupMembersModel, diags *diag.Diagnostics) bool {
	g, err := r.client.GetGroup(ctx, m.GroupID.ValueString())
	if err != nil {
		if client.IsSCIMNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read group", err)
		return false
	}
	ids := make([]string, 0, len(g.Members))
	for _, mem := range g.Members {
		if mem.Type == "" || strings.EqualFold(mem.Type, "User") {
			ids = append(ids, mem.Value)
		}
	}
	sort.Strings(ids)
	m.UserIDs = tfutil.SetOfStrings(ids, false)
	return true
}

func diff(want, have []string) (added, removed []string) {
	hs := map[string]bool{}
	for _, h := range have {
		hs[h] = true
	}
	ws := map[string]bool{}
	for _, w := range want {
		ws[w] = true
		if !hs[w] {
			added = append(added, w)
		}
	}
	for _, h := range have {
		if !ws[h] {
			removed = append(removed, h)
		}
	}
	return added, removed
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
