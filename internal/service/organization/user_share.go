package organization

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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

const (
	policySelectedOnly   = "SELECTED_ORG_ONLY"
	policyWithChildren   = "SELECTED_ORG_WITH_ALL_EXISTING_AND_FUTURE_CHILDREN"
	policyAllOrgs        = "ALL_EXISTING_AND_FUTURE_ORGS"
	userShareWaitTimeout = 2 * time.Minute
)

var (
	_ resource.ResourceWithConfigure   = (*userShareResource)(nil)
	_ resource.ResourceWithImportState = (*userShareResource)(nil)
)

// NewUserShareResource returns the wso2is_user_share resource.
func NewUserShareResource() resource.Resource { return &userShareResource{} }

type userShareResource struct{ client *client.Client }

type userShareModel struct {
	ID            types.String `tfsdk:"id"`
	UserID        types.String `tfsdk:"user_id"`
	ShareWithAll  types.Object `tfsdk:"share_with_all"`
	Organizations types.Set    `tfsdk:"organizations"`
}

type shareAllModel struct {
	Roles types.Set `tfsdk:"roles"`
}

type shareOrgModel struct {
	OrganizationID  types.String `tfsdk:"organization_id"`
	IncludeChildren types.Bool   `tfsdk:"include_children"`
	Roles           types.Set    `tfsdk:"roles"`
}

type shareRoleModel struct {
	Name         types.String `tfsdk:"name"`
	AudienceType types.String `tfsdk:"audience_type"`
	AudienceName types.String `tfsdk:"audience_name"`
}

var (
	shareRoleTypes = map[string]attr.Type{"name": types.StringType, "audience_type": types.StringType, "audience_name": types.StringType}
	shareRoleSet   = types.SetType{ElemType: types.ObjectType{AttrTypes: shareRoleTypes}}
	shareAllTypes  = map[string]attr.Type{"roles": shareRoleSet}
	shareOrgTypes  = map[string]attr.Type{"organization_id": types.StringType, "include_children": types.BoolType, "roles": shareRoleSet}
)

// Attributes inside set elements have no defaults: the framework matches set elements by
// value and misapplies them. read keeps null versus empty as configured instead.
func shareRolesAttribute() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		MarkdownDescription: "Roles the shared user gets in the organization(s). None when omitted.",
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{MarkdownDescription: "Role name.", Required: true},
			"audience_type": schema.StringAttribute{
				MarkdownDescription: "`application` for a role of a shared application, `organization` for an organization role.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("application", "organization")},
			},
			"audience_name": schema.StringAttribute{MarkdownDescription: "Name of the application, or of the organization for organization roles.", Required: true},
		}},
	}
}

func (r *userShareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_share"
}

func (r *userShareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shares a user of the root organization with sub-organizations (WSO2 IS 7.3 and later). The user can also get roles in those organizations. The same account can then sign in to the organizations' applications. The resource manages all shares of the user. The server applies changes in the background, and the provider waits until they are visible. To import, use the user ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `user_id`."),
			"user_id": schema.StringAttribute{
				MarkdownDescription: "ID of a user of the root organization.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"share_with_all": schema.SingleNestedAttribute{
				MarkdownDescription: "Share with every current and future organization. Conflicts with `organizations`.",
				Optional:            true,
				Attributes:          map[string]schema.Attribute{"roles": shareRolesAttribute()},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(path.MatchRoot("share_with_all"), path.MatchRoot("organizations")),
				},
			},
			"organizations": schema.SetNestedAttribute{
				MarkdownDescription: "Organizations to share the user with.",
				Optional:            true,
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"organization_id": schema.StringAttribute{MarkdownDescription: "Organization ID.", Required: true},
					"include_children": schema.BoolAttribute{
						MarkdownDescription: "Also share with the organization's current and future child organizations. `false` when omitted.",
						Optional:            true,
					},
					"roles": shareRolesAttribute(),
				}},
			},
		},
	}
}

func (r *userShareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

// ---- API model ----

type apiShareRole struct {
	DisplayName string `json:"displayName"`
	Audience    struct {
		Display string `json:"display"`
		Type    string `json:"type"`
	} `json:"audience"`
}

type apiRoleAssignment struct {
	Mode  string         `json:"mode"`
	Roles []apiShareRole `json:"roles,omitempty"`
}

type apiSharingMode struct {
	Policy         string            `json:"policy"`
	RoleAssignment apiRoleAssignment `json:"roleAssignment"`
}

type apiSharedOrg struct {
	OrgID       string          `json:"orgId"`
	ParentOrgID string          `json:"parentOrgId"`
	SharingMode *apiSharingMode `json:"sharingMode"`
	Roles       []apiShareRole  `json:"roles"`
}

type apiUserShares struct {
	SharingMode   *apiSharingMode `json:"sharingMode"`
	Organizations []apiSharedOrg  `json:"organizations"`
}

func roleAssignment(ctx context.Context, roles types.Set, diags *diag.Diagnostics) apiRoleAssignment {
	models, _ := tfutil.ElementsAs[shareRoleModel](ctx, roles, diags)
	if len(models) == 0 {
		return apiRoleAssignment{Mode: "NONE"}
	}
	ra := apiRoleAssignment{Mode: "SELECTED"}
	for _, m := range models {
		var role apiShareRole
		role.DisplayName = m.Name.ValueString()
		role.Audience.Display = m.AudienceName.ValueString()
		role.Audience.Type = m.AudienceType.ValueString()
		ra.Roles = append(ra.Roles, role)
	}
	return ra
}

func roleSet(ctx context.Context, roles []apiShareRole, diags *diag.Diagnostics) types.Set {
	models := make([]shareRoleModel, 0, len(roles))
	for _, r := range roles {
		models = append(models, shareRoleModel{Name: types.StringValue(r.DisplayName), AudienceType: types.StringValue(r.Audience.Type), AudienceName: types.StringValue(r.Audience.Display)})
	}
	return tfutil.SetFrom(ctx, shareRoleTypes, models, diags)
}

// roleKey is an order-independent fingerprint of a role list, for waiting.
func roleKey(roles []apiShareRole) string {
	keys := make([]string, 0, len(roles))
	for _, r := range roles {
		keys = append(keys, r.Audience.Type+"|"+r.Audience.Display+"|"+r.DisplayName)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func (r *userShareResource) userCriteria(userID string) map[string]any {
	return map[string]any{"userIds": []string{userID}}
}

func (r *userShareResource) post(ctx context.Context, p string, body map[string]any) error {
	_, err := r.client.Post(ctx, r.client.ServerV2Path(p), body, nil)
	return err
}

func (r *userShareResource) get(ctx context.Context, userID string) (*apiUserShares, error) {
	var out apiUserShares
	q := url.Values{"attributes": {"roles,sharingMode"}, "recursive": {"true"}}
	if err := r.client.Get(ctx, r.client.ServerV2Path("/users/"+url.PathEscape(userID)+"/share"), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// apply shares the user as planned, after removing what the prior state shared and the
// plan does not.
func (r *userShareResource) apply(ctx context.Context, plan, prior *userShareModel, diags *diag.Diagnostics) {
	userID := plan.UserID.ValueString()
	planOrgs, _ := tfutil.ElementsAs[shareOrgModel](ctx, plan.Organizations, diags)
	var priorOrgs []shareOrgModel
	priorAll := false
	if prior != nil {
		priorOrgs, _ = tfutil.ElementsAs[shareOrgModel](ctx, prior.Organizations, diags)
		priorAll = !prior.ShareWithAll.IsNull()
	}
	if diags.HasError() {
		return
	}
	planAll := !plan.ShareWithAll.IsNull()

	// Leaving "share with all" or dropping organizations.
	if priorAll && !planAll {
		if err := r.post(ctx, "/users/unshare-with-all", map[string]any{"userCriteria": r.userCriteria(userID)}); err != nil {
			tfutil.AddAPIError(diags, "stop sharing the user with all organizations", err)
			return
		}
		r.waitFor(ctx, userID, func(s *apiUserShares) bool { return s.SharingMode == nil && len(s.Organizations) == 0 }, diags)
	}
	keep := map[string]bool{}
	for _, o := range planOrgs {
		keep[o.OrganizationID.ValueString()] = true
	}
	var drop []string
	for _, o := range priorOrgs {
		if id := o.OrganizationID.ValueString(); !keep[id] {
			drop = append(drop, id)
		}
	}
	if len(drop) > 0 {
		if err := r.post(ctx, "/users/unshare", map[string]any{"userCriteria": r.userCriteria(userID), "orgIds": drop}); err != nil {
			tfutil.AddAPIError(diags, "stop sharing the user", err)
			return
		}
	}
	if diags.HasError() {
		return
	}

	if planAll {
		all, _ := tfutil.ObjectAs[shareAllModel](ctx, plan.ShareWithAll, diags)
		ra := roleAssignment(ctx, all.Roles, diags)
		if err := r.post(ctx, "/users/share-with-all", map[string]any{"userCriteria": r.userCriteria(userID), "policy": policyAllOrgs, "roleAssignment": ra}); err != nil {
			tfutil.AddAPIError(diags, "share the user with all organizations", err)
			return
		}
		want := roleKey(ra.Roles)
		r.waitFor(ctx, userID, func(s *apiUserShares) bool {
			return s.SharingMode != nil && s.SharingMode.Policy == policyAllOrgs && roleKey(s.SharingMode.RoleAssignment.Roles) == want
		}, diags)
		return
	}

	orgs := make([]map[string]any, 0, len(planOrgs))
	want := map[string]string{}
	for _, o := range planOrgs {
		policy := policySelectedOnly
		if o.IncludeChildren.ValueBool() {
			policy = policyWithChildren
		}
		ra := roleAssignment(ctx, o.Roles, diags)
		orgs = append(orgs, map[string]any{"orgId": o.OrganizationID.ValueString(), "policy": policy, "roleAssignment": ra})
		want[o.OrganizationID.ValueString()] = policy + "/" + roleKey(ra.Roles)
	}
	if err := r.post(ctx, "/users/share", map[string]any{"userCriteria": r.userCriteria(userID), "organizations": orgs}); err != nil {
		tfutil.AddAPIError(diags, "share the user", err)
		return
	}
	r.waitFor(ctx, userID, func(s *apiUserShares) bool {
		got := map[string]string{}
		for _, o := range s.Organizations {
			policy := policySelectedOnly
			if o.SharingMode != nil && o.SharingMode.Policy != "" {
				policy = o.SharingMode.Policy
			}
			got[o.OrgID] = policy + "/" + roleKey(o.Roles)
		}
		for id, w := range want {
			if got[id] != w {
				return false
			}
		}
		for _, id := range drop {
			if _, ok := got[id]; ok {
				return false
			}
		}
		return true
	}, diags)
}

func (r *userShareResource) waitFor(ctx context.Context, userID string, done func(*apiUserShares) bool, diags *diag.Diagnostics) {
	err := tfutil.WaitFor(ctx, userShareWaitTimeout, func() (bool, error) {
		s, err := r.get(ctx, userID)
		if err != nil {
			return false, err
		}
		return done(s), nil
	})
	if err != nil {
		diags.AddError("User sharing did not complete",
			fmt.Sprintf("The server accepted the request but did not show the expected shares within %s: %s. "+
				"The server processes sharing in the background and skips roles it cannot find (check role and audience names) without reporting an error.", userShareWaitTimeout, err))
	}
}

func (r *userShareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.UserID
	r.apply(ctx, &plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), plan.UserID)...)
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userShareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userShareModel
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

func (r *userShareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userShareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.UserID
	r.apply(ctx, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userShareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userShareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userID := state.UserID.ValueString()
	var err error
	if !state.ShareWithAll.IsNull() {
		err = r.post(ctx, "/users/unshare-with-all", map[string]any{"userCriteria": r.userCriteria(userID)})
	} else {
		orgs, _ := tfutil.ElementsAs[shareOrgModel](ctx, state.Organizations, &resp.Diagnostics)
		ids := make([]string, 0, len(orgs))
		for _, o := range orgs {
			ids = append(ids, o.OrganizationID.ValueString())
		}
		err = r.post(ctx, "/users/unshare", map[string]any{"userCriteria": r.userCriteria(userID), "orgIds": ids})
	}
	if err != nil {
		if !client.IsNotFound(err) {
			tfutil.AddAPIError(&resp.Diagnostics, "stop sharing the user", err)
		}
		return
	}
	// Wait, so that deleting the organizations or the user right after does not race the
	// background job.
	r.waitFor(ctx, userID, func(s *apiUserShares) bool { return s.SharingMode == nil && len(s.Organizations) == 0 }, &resp.Diagnostics)
}

func (r *userShareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), req.ID)...)
}

func (r *userShareResource) read(ctx context.Context, m *userShareModel, diags *diag.Diagnostics) bool {
	s, err := r.get(ctx, m.UserID.ValueString())
	if err != nil {
		if !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "read user shares", err)
		}
		return false
	}
	m.ID = m.UserID
	// Prior values, to keep omitted attributes null when the server reports the default.
	priorAllRolesNull := true
	if prior, ok := tfutil.ObjectAs[shareAllModel](ctx, m.ShareWithAll, diags); ok {
		priorAllRolesNull = prior.Roles.IsNull()
	}
	priorOrgs := map[string]shareOrgModel{}
	if list, ok := tfutil.ElementsAs[shareOrgModel](ctx, m.Organizations, diags); ok {
		for _, o := range list {
			priorOrgs[o.OrganizationID.ValueString()] = o
		}
	}
	roles := func(api []apiShareRole, keepNull bool) types.Set {
		if len(api) == 0 && keepNull {
			return types.SetNull(types.ObjectType{AttrTypes: shareRoleTypes})
		}
		return roleSet(ctx, api, diags)
	}
	if s.SharingMode != nil && s.SharingMode.Policy == policyAllOrgs {
		m.ShareWithAll = tfutil.ObjectFrom(ctx, shareAllTypes, shareAllModel{Roles: roles(s.SharingMode.RoleAssignment.Roles, priorAllRolesNull)}, diags)
		m.Organizations = types.SetNull(types.ObjectType{AttrTypes: shareOrgTypes})
		return true
	}
	// Organizations shared through a parent's "with children" policy are not listed.
	byID := map[string]apiSharedOrg{}
	for _, o := range s.Organizations {
		byID[o.OrgID] = o
	}
	inherited := func(o apiSharedOrg) bool {
		for p, ok := byID[o.ParentOrgID]; ok; p, ok = byID[p.ParentOrgID] {
			if p.SharingMode != nil && p.SharingMode.Policy == policyWithChildren {
				return true
			}
		}
		return false
	}
	var orgs []shareOrgModel
	for _, o := range s.Organizations {
		if inherited(o) {
			continue
		}
		prior, known := priorOrgs[o.OrgID]
		children := types.BoolValue(o.SharingMode != nil && o.SharingMode.Policy == policyWithChildren)
		if !children.ValueBool() && (!known || prior.IncludeChildren.IsNull()) {
			children = types.BoolNull()
		}
		orgs = append(orgs, shareOrgModel{
			OrganizationID:  types.StringValue(o.OrgID),
			IncludeChildren: children,
			Roles:           roles(o.Roles, !known || prior.Roles.IsNull()),
		})
	}
	if len(orgs) == 0 {
		return false
	}
	m.ShareWithAll = types.ObjectNull(shareAllTypes)
	m.Organizations = tfutil.SetFrom(ctx, shareOrgTypes, orgs, diags)
	return true
}
