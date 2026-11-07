// Package organization implements organizations (B2B) and application sharing.
package organization

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*orgResource)(nil)
	_ resource.ResourceWithImportState = (*orgResource)(nil)
)

// NewResource returns the wso2is_organization resource.
func NewResource() resource.Resource { return &orgResource{} }

type orgResource struct{ client *client.Client }

type orgModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Handle      types.String `tfsdk:"handle"`
	Description types.String `tfsdk:"description"`
	ParentID    types.String `tfsdk:"parent_id"`
	Type        types.String `tfsdk:"type"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Attributes  types.Map    `tfsdk:"attributes"`
	Version     types.String `tfsdk:"version"`
}

func (r *orgResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *orgResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a sub-organization (a B2B organization). The organization sits under the root organization or under another sub-organization. Share applications with it with `wso2is_application_share`. To manage objects inside it, add a provider block with an `alias` that sets `organization_id`. The server only deletes an organization after its child organizations. To import, use the organization ID.",
		Attributes: map[string]schema.Attribute{
			"id":   tfutil.IDAttribute("Organization ID."),
			"name": schema.StringAttribute{MarkdownDescription: "Organization name.", Required: true},
			"handle": schema.StringAttribute{
				MarkdownDescription: "Organization handle (used in URLs, WSO2 IS 7.1+). The default is the server's choice (the ID). The server only accepts this value at creation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString(), stringplanmodifier.RequiresReplaceIfConfigured()},
			},
			"description": tfutil.String("Description.", ""),
			"parent_id":   tfutil.ComputedString("The ID of the parent organization, which is the organization that the provider manages. WSO2 only creates an organization from its parent. To create a nested organization, use a provider alias whose `organization_id` is the parent."),
			"type": schema.StringAttribute{
				MarkdownDescription: "`TENANT` (default) or `STRUCTURAL`. The server only accepts this value at creation.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("TENANT"),
				Validators:          []validator.String{stringvalidator.OneOf("TENANT", "STRUCTURAL")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the organization is active. The default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"attributes": schema.MapAttribute{
				MarkdownDescription: "Custom organization attributes.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"version": tfutil.ComputedString("Organization version."),
		},
	}
}

func (r *orgResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func attributes(ctx context.Context, m types.Map, diags *diag.Diagnostics) []client.Attribute {
	attrs := tfutil.MapOfStrings(ctx, m, diags)
	out := make([]client.Attribute, 0, len(attrs))
	for _, k := range tfutil.SortedKeys(attrs) {
		out = append(out, client.Attribute{Key: k, Value: attrs[k]})
	}
	return out
}

func (r *orgResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o := &client.Organization{
		Name: plan.Name.ValueString(), Description: plan.Description.ValueString(), Type: plan.Type.ValueString(),
		OrgHandle: plan.Handle.ValueString(), Attributes: attributes(ctx, plan.Attributes, &resp.Diagnostics),
	}
	if !tfutil.Known(plan.Handle) {
		o.OrgHandle = ""
	}
	id, err := r.client.CreateOrganization(ctx, o)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create organization", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	plan.ID = types.StringValue(id)
	if !plan.Enabled.ValueBool() {
		r.put(ctx, &plan, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Organization not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgResource) put(ctx context.Context, m *orgModel, diags *diag.Diagnostics) {
	status := "ACTIVE"
	if !m.Enabled.ValueBool() {
		status = "DISABLED"
	}
	body := map[string]any{"name": m.Name.ValueString(), "description": m.Description.ValueString(), "status": status, "attributes": attributes(ctx, m.Attributes, diags)}
	if err := r.client.PutOrganization(ctx, m.ID.ValueString(), body); err != nil {
		tfutil.AddAPIError(diags, "update organization", err)
	}
}

func (r *orgResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgModel
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

func (r *orgResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state orgModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	r.put(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Organization not found after update", plan.ID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *orgResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteOrganization(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete organization", err)
	}
}

func (r *orgResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *orgResource) read(ctx context.Context, m *orgModel, diags *diag.Diagnostics) bool {
	o, err := r.client.GetOrganization(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read organization", err)
		return false
	}
	m.Name = types.StringValue(o.Name)
	m.Handle = types.StringValue(o.OrgHandle)
	m.Description = types.StringValue(o.Description)
	m.Type = types.StringValue(o.Type)
	m.Enabled = types.BoolValue(o.Status != "DISABLED")
	m.Version = types.StringValue(o.Version)
	if o.Parent != nil {
		m.ParentID = types.StringValue(o.Parent.ID)
	}
	attrs := map[string]string{}
	for _, a := range o.Attributes {
		attrs[a.Key] = a.Value
	}
	m.Attributes = tfutil.MapValueOfStrings(attrs, m.Attributes.IsNull())
	return true
}

// ---- wso2is_application_share ----

var (
	_ resource.ResourceWithConfigure   = (*shareResource)(nil)
	_ resource.ResourceWithImportState = (*shareResource)(nil)
)

// NewShareResource returns the wso2is_application_share resource.
func NewShareResource() resource.Resource { return &shareResource{} }

type shareResource struct{ client *client.Client }

type shareModel struct {
	ID              types.String `tfsdk:"id"`
	ApplicationID   types.String `tfsdk:"application_id"`
	ShareWithAll    types.Bool   `tfsdk:"share_with_all_children"`
	OrganizationIDs types.Set    `tfsdk:"organization_ids"`
}

func (r *shareResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_share"
}

func (r *shareResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Shares an application with sub-organizations. Users of those organizations can then sign in to the application, and each organization gets its own copy of it. The resource manages the complete list of organizations that the application is shared with. To import, use the application ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `application_id`."),
			"application_id": schema.StringAttribute{
				MarkdownDescription: "Application ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"share_with_all_children": schema.BoolAttribute{
				MarkdownDescription: "Share with every current and future child organization. Conflicts with `organization_ids`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"organization_ids": schema.SetAttribute{
				MarkdownDescription: "IDs of the organizations to share the application with.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{setvalidator.ConflictsWith(path.MatchRoot("share_with_all_children"))},
				PlanModifiers:       []planmodifier.Set{tfutil.KeepStateSet()},
			},
		},
	}
}

func (r *shareResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *shareResource) apply(ctx context.Context, m *shareModel, prev []string, diags *diag.Diagnostics) {
	appID := m.ApplicationID.ValueString()
	if m.ShareWithAll.ValueBool() {
		if err := r.client.ShareApplication(ctx, appID, true, nil); err != nil {
			tfutil.AddAPIError(diags, "share application", err)
		}
		return
	}
	want := tfutil.Strings(ctx, m.OrganizationIDs, diags)
	wantSet := map[string]bool{}
	for _, id := range want {
		wantSet[id] = true
	}
	var add []string
	prevSet := map[string]bool{}
	for _, id := range prev {
		prevSet[id] = true
	}
	for _, id := range want {
		if !prevSet[id] {
			add = append(add, id)
		}
	}
	if len(add) > 0 {
		if err := r.client.ShareApplication(ctx, appID, false, add); err != nil {
			tfutil.AddAPIError(diags, "share application", err)
			return
		}
	}
	for _, id := range prev {
		if !wantSet[id] {
			if err := r.client.UnshareApplication(ctx, appID, id); err != nil && !client.IsNotFound(err) {
				tfutil.AddAPIError(diags, "stop sharing application with "+id, err)
				return
			}
		}
	}
	// Sharing is applied in the background (on a fresh server the organization is listed a
	// few seconds later); wait, so that what reads the share sees the organizations.
	err := tfutil.WaitFor(ctx, 2*time.Minute, func() (bool, error) {
		ids, ok := r.current(ctx, appID, diags)
		if !ok || len(ids) != len(wantSet) {
			return false, nil
		}
		for _, id := range ids {
			if !wantSet[id] {
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		diags.AddError("Application sharing did not complete", fmt.Sprintf("The organizations the application is shared with did not match the configuration within 2 minutes: %s", err))
	}
}

func (r *shareResource) current(ctx context.Context, appID string, diags *diag.Diagnostics) ([]string, bool) {
	orgs, err := r.client.GetApplicationSharedOrganizations(ctx, appID)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, false
		}
		tfutil.AddAPIError(diags, "read application sharing", err)
		return nil, false
	}
	// The server answers an empty list, not 404, for an application that does not exist.
	if len(orgs) == 0 && r.client.ApplicationMissing(ctx, appID) {
		return nil, false
	}
	ids := make([]string, 0, len(orgs))
	for _, o := range orgs {
		ids = append(ids, o.ID)
	}
	sort.Strings(ids)
	return ids, true
}

func (r *shareResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan shareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	prev, _ := r.current(ctx, plan.ApplicationID.ValueString(), &resp.Diagnostics)
	r.apply(ctx, &plan, prev, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.ApplicationID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *shareResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state shareModel
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

func (r *shareResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan shareModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	prev, _ := r.current(ctx, plan.ApplicationID.ValueString(), &resp.Diagnostics)
	r.apply(ctx, &plan, prev, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = plan.ApplicationID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *shareResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state shareModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.UnshareApplicationAll(ctx, state.ApplicationID.ValueString())
	if err != nil && !client.IsNotFound(err) && !r.client.ApplicationMissing(ctx, state.ApplicationID.ValueString()) {
		tfutil.AddAPIError(&resp.Diagnostics, "stop sharing application", err)
	}
}

func (r *shareResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("share_with_all_children"), false)...)
}

func (r *shareResource) read(ctx context.Context, m *shareModel, diags *diag.Diagnostics) bool {
	ids, ok := r.current(ctx, m.ApplicationID.ValueString(), diags)
	if !ok {
		return false
	}
	m.OrganizationIDs = tfutil.SetOfStrings(ids, false)
	return true
}

// ---- data source ----

var _ datasource.DataSourceWithConfigValidators = (*orgDataSource)(nil)

// NewDataSource returns the wso2is_organization data source.
func NewDataSource() datasource.DataSource { return &orgDataSource{} }

type orgDataSource struct{ client *client.Client }

type orgDSModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Handle      types.String `tfsdk:"handle"`
	Description types.String `tfsdk:"description"`
	ParentID    types.String `tfsdk:"parent_id"`
	Type        types.String `tfsdk:"type"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Attributes  types.Map    `tfsdk:"attributes"`
}

func (d *orgDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *orgDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Looks up an organization by ID or name. The search covers all organizations below the provider's organization.",
		Attributes: map[string]dsschema.Attribute{
			"id":          dsschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Organization ID."},
			"name":        dsschema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Organization name."},
			"handle":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Organization handle."},
			"description": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
			"parent_id":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Parent organization ID."},
			"type":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "`TENANT` or `STRUCTURAL`."},
			"enabled":     dsschema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is active."},
			"attributes":  dsschema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Custom attributes."},
		},
	}
}

func (d *orgDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("name"))}
}

func (d *orgDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = tfutil.ConfigureDataSource(req, resp)
}

func (d *orgDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m orgDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()
	if id == "" {
		orgs, err := client.FindByEq(ctx, func(ctx context.Context, filter string) ([]client.Organization, error) {
			return d.client.ListOrganizations(ctx, filter, true)
		}, "name", m.Name.ValueString())
		if err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "list organizations", err)
			return
		}
		var matches []string
		for _, o := range orgs {
			if o.Name == m.Name.ValueString() {
				matches = append(matches, o.ID)
			}
		}
		if len(matches) != 1 {
			resp.Diagnostics.AddError("Organization lookup failed", fmt.Sprintf("found %d organizations named %q; look it up by ID instead", len(matches), m.Name.ValueString()))
			return
		}
		id = matches[0]
	}
	o, err := d.client.GetOrganization(ctx, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read organization", err)
		return
	}
	m.ID, m.Name, m.Handle, m.Description = types.StringValue(o.ID), types.StringValue(o.Name), types.StringValue(o.OrgHandle), types.StringValue(o.Description)
	m.Type, m.Enabled = types.StringValue(o.Type), types.BoolValue(o.Status != "DISABLED")
	m.ParentID = types.StringNull()
	if o.Parent != nil {
		m.ParentID = types.StringValue(o.Parent.ID)
	}
	attrs := map[string]string{}
	for _, a := range o.Attributes {
		attrs[a.Key] = a.Value
	}
	m.Attributes = tfutil.MapValueOfStrings(attrs, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
