// Package userstore implements wso2is_userstore (secondary user stores).
package userstore

import (
	"context"
	"strings"
	"time"

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
	_ resource.ResourceWithConfigure   = (*userstoreResource)(nil)
	_ resource.ResourceWithImportState = (*userstoreResource)(nil)
)

// NewResource returns the wso2is_userstore resource.
func NewResource() resource.Resource { return &userstoreResource{} }

type userstoreResource struct{ client *client.Client }

type model struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Type                   types.String `tfsdk:"type"`
	TypeID                 types.String `tfsdk:"type_id"`
	Description            types.String `tfsdk:"description"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	Properties             types.Map    `tfsdk:"properties"`
	SecretProperties       types.Map    `tfsdk:"secret_properties"`
	ClaimAttributeMappings types.Map    `tfsdk:"claim_attribute_mappings"`
}

func (r *userstoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_userstore"
}

func (r *userstoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a secondary user store. A user store can be a JDBC database, a read-only or read-write LDAP directory, or Active Directory. Users and groups in a secondary user store have the store's domain name as a prefix, for example `CUSTOMERS/alice`. In `wso2is_user`, set `user_store = \"CUSTOMERS\"`.\n\nThe server deploys user stores in the background, and the provider waits until the server reports the change. The provider only manages the property keys in your configuration. To import, use the user store's domain name.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("User store ID (base64url of the domain name)."),
			"name": schema.StringAttribute{
				MarkdownDescription: "Domain name, for example `CUSTOMERS`. Changing it replaces the user store.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "User store manager type: `UniqueIDJDBCUserStoreManager`, `UniqueIDReadOnlyLDAPUserStoreManager`, `UniqueIDReadWriteLDAPUserStoreManager` or `UniqueIDActiveDirectoryUserStoreManager`. Changing it replaces the user store.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type_id":     tfutil.ComputedString("Type ID used by the API."),
			"description": tfutil.String("Description.", ""),
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the user store is enabled (the `Disabled` property). The default is `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"properties": schema.MapAttribute{
				MarkdownDescription: "The user store's properties. A JDBC store needs `url`, `userName` and `driverName` (for example `org.postgresql.Driver`). An LDAP store needs properties such as `ConnectionURL`, `ConnectionName`, `UserSearchBase`, `UserNameAttribute` and `GroupSearchBase`. The Console and `GET /userstores/meta/types/{type-id}` list every property.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"secret_properties": schema.MapAttribute{
				MarkdownDescription: "Confidential properties, such as `password` for JDBC or `ConnectionPassword` for LDAP. The server returns them encrypted, so the provider cannot notice changes made outside Terraform.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
			"claim_attribute_mappings": schema.MapAttribute{
				MarkdownDescription: "User store attribute by local claim URI, for claims that use a different attribute in this user store.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *userstoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *userstoreResource) typeID(ctx context.Context, name string, diags *diag.Diagnostics) string {
	types, err := r.client.ListUserstoreTypes(ctx)
	if err != nil {
		tfutil.AddAPIError(diags, "list user store types", err)
		return ""
	}
	for _, t := range types {
		if t.TypeName == name || t.TypeID == name || t.ClassName == name {
			return t.TypeID
		}
	}
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.TypeName)
	}
	diags.AddAttributeError(path.Root("type"), "Unknown user store type", "Available types: "+strings.Join(names, ", "))
	return ""
}

func body(ctx context.Context, m *model, typeID string, server map[string]string, prevManaged map[string]bool, diags *diag.Diagnostics) map[string]any {
	merged := map[string]string{}
	for k, v := range server {
		if !prevManaged[k] {
			merged[k] = v
		}
	}
	for k, v := range tfutil.MapOfStrings(ctx, m.Properties, diags) {
		merged[k] = v
	}
	for k, v := range tfutil.MapOfStrings(ctx, m.SecretProperties, diags) {
		merged[k] = v
	}
	merged["Disabled"] = boolString(!m.Enabled.ValueBool())
	props := make([]map[string]string, 0, len(merged))
	for _, k := range tfutil.SortedKeys(merged) {
		props = append(props, map[string]string{"name": k, "value": merged[k]})
	}
	b := map[string]any{"typeId": typeID, "name": m.Name.ValueString(), "description": m.Description.ValueString(), "properties": props}
	if !m.ClaimAttributeMappings.IsNull() {
		cams := []map[string]string{}
		mappings := tfutil.MapOfStrings(ctx, m.ClaimAttributeMappings, diags)
		for _, k := range tfutil.SortedKeys(mappings) {
			cams = append(cams, map[string]string{"claimURI": k, "mappedAttribute": mappings[k]})
		}
		b["claimAttributeMappings"] = cams
	}
	return b
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func (r *userstoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	typeID := r.typeID(ctx, plan.Type.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := r.client.CreateUserstore(ctx, body(ctx, &plan, typeID, nil, nil, &resp.Diagnostics))
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create user store", err)
		return
	}
	plan.ID, plan.TypeID = types.StringValue(id), types.StringValue(typeID)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("User store not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userstoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

func (r *userstoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	cur, err := r.client.GetUserstore(ctx, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read user store", err)
		return
	}
	server := propertyMap(cur)
	prev := map[string]bool{}
	for k := range state.Properties.Elements() {
		prev[k] = true
	}
	for k := range state.SecretProperties.Elements() {
		prev[k] = true
	}
	b := body(ctx, &plan, state.TypeID.ValueString(), server, prev, &resp.Diagnostics)
	if err := r.client.PutUserstore(ctx, id, b); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update user store", err)
		return
	}
	// The new configuration is deployed asynchronously: wait until it is served.
	want := tfutil.MapOfStrings(ctx, plan.Properties, &resp.Diagnostics)
	err = tfutil.WaitFor(ctx, 2*time.Minute, func() (bool, error) {
		cur, err := r.client.GetUserstore(ctx, id)
		if err != nil {
			return false, err
		}
		if tfutil.GetStringOrEmpty(cur, "description").ValueString() != plan.Description.ValueString() {
			return false, nil
		}
		got := propertyMap(cur)
		for k, v := range want {
			if got[k] != v {
				return false, nil
			}
		}
		return strings.EqualFold(got["Disabled"], boolString(!plan.Enabled.ValueBool())) || got["Disabled"] == "" && plan.Enabled.ValueBool(), nil
	})
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "wait for the user store update", err)
		return
	}
	plan.ID, plan.TypeID = state.ID, state.TypeID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("User store not found after update", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userstoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteUserstore(ctx, state.ID.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete user store", err)
	}
}

func (r *userstoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), client.UserstoreID(req.ID))...)
}

func propertyMap(us map[string]any) map[string]string {
	out := map[string]string{}
	for _, p := range tfutil.GetObjects(us, "properties") {
		k, _ := p["name"].(string)
		v, _ := p["value"].(string)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func (r *userstoreResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	us, err := r.client.GetUserstore(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read user store", err)
		return false
	}
	importing := m.Name.IsNull()
	m.Name = tfutil.GetString(us, "name")
	if t := tfutil.GetString(us, "typeName"); tfutil.Known(t) && t.ValueString() != "" {
		if m.Type.IsNull() || (m.Type.ValueString() != t.ValueString() && m.Type.ValueString() != tfutil.GetString(us, "typeId").ValueString()) {
			m.Type = t
		}
	}
	m.TypeID = tfutil.GetString(us, "typeId")
	if m.TypeID.IsNull() {
		m.TypeID = types.StringValue("")
	}
	m.Description = tfutil.GetStringOrEmpty(us, "description")

	props := propertyMap(us)
	m.Enabled = types.BoolValue(!strings.EqualFold(props["Disabled"], "true"))
	managed := m.Properties.Elements()
	secrets := m.SecretProperties.Elements()
	plain := map[string]string{}
	for k, v := range props {
		if k == "Disabled" {
			continue
		}
		if _, ok := managed[k]; ok || (importing && !isSecret(k)) {
			plain[k] = v
		}
	}
	m.Properties = tfutil.MapValueOfStrings(plain, false)
	// Secrets come back encrypted; keep the configured values.
	if len(secrets) == 0 {
		m.SecretProperties = types.MapNull(types.StringType)
	}

	if !m.ClaimAttributeMappings.IsNull() {
		managedCAM := m.ClaimAttributeMappings.Elements()
		cams := map[string]string{}
		for _, cam := range tfutil.GetObjects(us, "claimAttributeMappings") {
			k, _ := cam["claimURI"].(string)
			v, _ := cam["mappedAttribute"].(string)
			if _, ok := managedCAM[k]; ok {
				cams[k] = v
			}
		}
		m.ClaimAttributeMappings = tfutil.MapValueOfStrings(cams, true)
	}
	return true
}

func isSecret(k string) bool {
	l := strings.ToLower(k)
	return strings.Contains(l, "password") || strings.Contains(l, "secret")
}
