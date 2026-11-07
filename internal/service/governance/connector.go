// Package governance implements identity governance connectors (login and registration
// policies) and input validation rules.
package governance

import (
	"context"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*connectorResource)(nil)
	_ resource.ResourceWithImportState = (*connectorResource)(nil)
)

// NewConnectorResource returns the wso2is_governance_connector resource.
func NewConnectorResource() resource.Resource { return &connectorResource{} }

type connectorResource struct{ client *client.Client }

type connectorModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	CategoryID       types.String `tfsdk:"category_id"`
	Properties       types.Map    `tfsdk:"properties"`
	RestoreOnDestroy types.Bool   `tfsdk:"restore_on_destroy"`
}

func (r *connectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_connector"
}

func (r *connectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an identity governance connector. Governance connectors hold the tenant-wide login, registration, account recovery and account policies. In the Console, they are under **Login & Registration**.\n\nThe resource only manages the properties that you list. If you remove a property from the configuration, the provider sets it back to the server default. When you destroy the resource, the provider sets all listed properties back to their defaults, unless `restore_on_destroy = false`.\n\nExamples of connectors are `account.lock.handler`, `account.disable.handler`, `self-sign-up`, `lite-user-sign-up`, `user-email-verification`, `account-recovery`, `admin-forced-password-reset`, `passwordHistory`, `passwordExpiry`, `multiattribute.login.handler`, `sso.login.recaptcha`, `suspension.notification`, `pii-controller`, `user-claim-update` and `organization-self-service`. MFA connectors include `email-otp-authenticator`, `sms-otp-authenticator`, `totp`, `backup-code-authenticator` and `FIDOAuthenticator`. To see a connector's properties, use the `wso2is_governance_connector` data source. To import, use the connector name.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Connector ID."),
			"name": schema.StringAttribute{
				MarkdownDescription: "Connector name, for example `account.lock.handler`.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"category_id": tfutil.ComputedString("Category the connector belongs to."),
			"properties": schema.MapAttribute{
				MarkdownDescription: "Property values by property name, for example `{ \"account.lock.handler.lock.on.max.failed.attempts.enable\" = \"true\" }`. Values are strings.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"restore_on_destroy": schema.BoolAttribute{
				MarkdownDescription: "Revert the managed properties to the server defaults when the resource is destroyed. The default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
		},
	}
}

func (r *connectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *connectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cat, id, err := r.client.FindGovernanceConnector(ctx, plan.Name.ValueString())
	if err != nil {
		tfutil.AddAPIAttributeError(&resp.Diagnostics, path.Root("name"), "find governance connector", err)
		return
	}
	props := tfutil.MapOfStrings(ctx, plan.Properties, &resp.Diagnostics)
	if err := r.client.UpdateGovernanceConnector(ctx, cat, id, props); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update governance connector", err)
		return
	}
	r.waitApplied(ctx, cat, id, props)
	plan.ID, plan.CategoryID = types.StringValue(id), types.StringValue(cat)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tfutil.MissingID(state.ID) {
		resp.State.RemoveResource(ctx)
		return
	}
	if state.CategoryID.ValueString() == "" {
		// The prior state can hold only the ID and the arguments (see tfutil.SetStringIfNull).
		cat, id, err := r.client.FindGovernanceConnector(ctx, state.ID.ValueString())
		if err != nil {
			if client.IsNotFound(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			tfutil.AddAPIError(&resp.Diagnostics, "find governance connector", err)
			return
		}
		state.ID, state.CategoryID = types.StringValue(id), types.StringValue(cat)
	}
	tfutil.SetBoolIfNull(&state.RestoreOnDestroy, true)
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state connectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	newProps := tfutil.MapOfStrings(ctx, plan.Properties, &resp.Diagnostics)
	oldProps := tfutil.MapOfStrings(ctx, state.Properties, &resp.Diagnostics)
	changed := map[string]string{}
	for k, v := range newProps {
		if old, ok := oldProps[k]; !ok || old != v {
			changed[k] = v
		}
	}
	var removed []string
	for k := range oldProps {
		if _, ok := newProps[k]; !ok {
			removed = append(removed, k)
		}
	}
	cat, id := state.CategoryID.ValueString(), state.ID.ValueString()
	if err := r.client.RevertGovernanceProperties(ctx, cat, id, removed); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "revert governance connector properties", err)
		return
	}
	if err := r.client.UpdateGovernanceConnector(ctx, cat, id, changed); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update governance connector", err)
		return
	}
	r.waitApplied(ctx, cat, id, changed)
	plan.ID, plan.CategoryID = state.ID, state.CategoryID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !state.RestoreOnDestroy.ValueBool() {
		return
	}
	names := tfutil.SortedKeys(tfutil.MapOfStrings(ctx, state.Properties, &resp.Diagnostics))
	if err := r.client.RevertGovernanceProperties(ctx, state.CategoryID.ValueString(), state.ID.ValueString(), names); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "revert governance connector properties", err)
	}
}

func (r *connectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cat, id, err := r.client.FindGovernanceConnector(ctx, req.ID)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "find governance connector", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("category_id"), cat)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("restore_on_destroy"), true)...)
}

// waitApplied re-reads the connector for a few seconds until it reports the values just
// written: a read right after an update occasionally returns the previous values.
func (r *connectorResource) waitApplied(ctx context.Context, cat, id string, want map[string]string) {
	if len(want) == 0 {
		return
	}
	_ = tfutil.WaitFor(ctx, 15*time.Second, func() (bool, error) {
		// A failed read counts as "not yet"; the read after the wait reports errors.
		con, err := r.client.GetGovernanceConnector(ctx, cat, id)
		if err != nil || con == nil {
			return false, nil //nolint:nilerr // retried until the timeout
		}
		got := make(map[string]string, len(con.Properties))
		for _, p := range con.Properties {
			got[p.Name] = p.Value
		}
		for k, v := range want {
			if got[k] != v {
				return false, nil
			}
		}
		return true, nil
	})
}

func (r *connectorResource) read(ctx context.Context, m *connectorModel, diags *diag.Diagnostics) bool {
	con, err := r.client.GetGovernanceConnector(ctx, m.CategoryID.ValueString(), m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read governance connector", err)
		return false
	}
	m.Name = types.StringValue(con.Name)
	managed := m.Properties.Elements()
	importing := m.Properties.IsNull()
	out := map[string]string{}
	for _, p := range con.Properties {
		if _, ok := managed[p.Name]; ok || importing {
			out[p.Name] = p.Value
		}
	}
	m.Properties = tfutil.MapValueOfStrings(out, false)
	return true
}

// ---- data source ----

var _ datasource.DataSourceWithConfigure = (*connectorDataSource)(nil)

// NewConnectorDataSource returns the wso2is_governance_connector data source.
func NewConnectorDataSource() datasource.DataSource { return &connectorDataSource{} }

type connectorDataSource struct{ client *client.Client }

type connectorDSModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	CategoryID   types.String `tfsdk:"category_id"`
	Category     types.String `tfsdk:"category"`
	FriendlyName types.String `tfsdk:"friendly_name"`
	Properties   types.Map    `tfsdk:"properties"`
	Details      types.List   `tfsdk:"property_details"`
}

type propertyDetailModel struct {
	Name        types.String `tfsdk:"name"`
	Value       types.String `tfsdk:"value"`
	DisplayName types.String `tfsdk:"display_name"`
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
}

func (d *connectorDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_governance_connector"
}

func (d *connectorDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		MarkdownDescription: "Reads a governance connector with all of its properties, their current values and their descriptions. Use it to find property names for `wso2is_governance_connector`.",
		Attributes: map[string]dsschema.Attribute{
			"id":            dsschema.StringAttribute{Computed: true, MarkdownDescription: "Connector ID."},
			"name":          dsschema.StringAttribute{Required: true, MarkdownDescription: "Connector name, for example `self-sign-up`."},
			"category_id":   dsschema.StringAttribute{Computed: true, MarkdownDescription: "Category ID."},
			"category":      dsschema.StringAttribute{Computed: true, MarkdownDescription: "Category name."},
			"friendly_name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
			"properties":    dsschema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Current values by property name."},
			"property_details": dsschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Every property with its metadata.",
				NestedObject: dsschema.NestedAttributeObject{Attributes: map[string]dsschema.Attribute{
					"name":         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Property name."},
					"value":        dsschema.StringAttribute{Computed: true, MarkdownDescription: "Current value."},
					"display_name": dsschema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
					"description":  dsschema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
					"type":         dsschema.StringAttribute{Computed: true, MarkdownDescription: "Data type."},
				}},
			},
		},
	}
}

func (d *connectorDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = tfutil.ConfigureDataSource(req, resp)
}

func (d *connectorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m connectorDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	cat, id, err := d.client.FindGovernanceConnector(ctx, m.Name.ValueString())
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "find governance connector", err)
		return
	}
	con, err := d.client.GetGovernanceConnector(ctx, cat, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read governance connector", err)
		return
	}
	m.ID, m.CategoryID = types.StringValue(id), types.StringValue(cat)
	m.Category = types.StringValue(con.Category)
	m.FriendlyName = types.StringValue(con.FriendlyName)
	props := map[string]string{}
	details := make([]propertyDetailModel, 0, len(con.Properties))
	sort.Slice(con.Properties, func(i, j int) bool { return con.Properties[i].Name < con.Properties[j].Name })
	for _, p := range con.Properties {
		props[p.Name] = p.Value
		typ := ""
		if p.Meta != nil {
			typ = p.Meta.Type
		}
		details = append(details, propertyDetailModel{
			Name: types.StringValue(p.Name), Value: types.StringValue(p.Value), DisplayName: types.StringValue(p.DisplayName),
			Description: types.StringValue(p.Description), Type: types.StringValue(typ),
		})
	}
	m.Properties = tfutil.MapValueOfStrings(props, false)
	lv, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: map[string]attr.Type{
		"name": types.StringType, "value": types.StringType, "display_name": types.StringType, "description": types.StringType, "type": types.StringType,
	}}, details)
	resp.Diagnostics.Append(diags...)
	m.Details = lv
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
