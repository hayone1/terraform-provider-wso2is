// Package lookup implements read-only data sources for looking up existing objects
// (including built-in ones such as the Console application or the admin role).
package lookup

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// base is embedded by every data source in this package.
type base struct{ client *client.Client }

func (b *base) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	b.client = tfutil.ConfigureDataSource(req, resp)
}

func lookupKey(desc string) schema.StringAttribute {
	return schema.StringAttribute{MarkdownDescription: desc, Optional: true, Computed: true}
}

func computed(desc string) schema.StringAttribute {
	return schema.StringAttribute{MarkdownDescription: desc, Computed: true}
}

func exactlyOne(names ...string) []datasource.ConfigValidator {
	exprs := make([]path.Expression, 0, len(names))
	for _, n := range names {
		exprs = append(exprs, path.MatchRoot(n))
	}
	return []datasource.ConfigValidator{datasourcevalidator.ExactlyOneOf(exprs...)}
}

func notFound(diags *diag.Diagnostics, kind string, n int, by string) {
	if n == 0 {
		diags.AddError(kind+" not found", fmt.Sprintf("no %s matches %s", strings.ToLower(kind), by))
		return
	}
	diags.AddError("Multiple "+strings.ToLower(kind)+"s found", fmt.Sprintf("%d %ss match %s; use a more specific lookup", n, strings.ToLower(kind), by))
}

// ---- wso2is_application ----

// NewApplicationDataSource returns the wso2is_application data source.
func NewApplicationDataSource() datasource.DataSource { return &applicationDS{} }

type applicationDS struct{ base }

type applicationModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	ClientID        types.String `tfsdk:"client_id"`
	Description     types.String `tfsdk:"description"`
	Issuer          types.String `tfsdk:"issuer"`
	TemplateID      types.String `tfsdk:"template_id"`
	AccessURL       types.String `tfsdk:"access_url"`
	IsManagementApp types.Bool   `tfsdk:"is_management_app"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Protocols       types.Set    `tfsdk:"protocols"`
}

func (d *applicationDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (d *applicationDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an application by ID, name or OAuth2 client ID, for example the built-in Console or My Account application.",
		Attributes: map[string]schema.Attribute{
			"id":                lookupKey("Application ID."),
			"name":              lookupKey("Application name."),
			"client_id":         lookupKey("OAuth2 client ID."),
			"description":       computed("Description."),
			"issuer":            computed("SAML issuer."),
			"template_id":       computed("Console template."),
			"access_url":        computed("Access URL."),
			"is_management_app": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it can call the management APIs."},
			"enabled":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether users can sign in."},
			"protocols":         schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Configured inbound protocols: `oauth2`, `samlsso`, `passivests`, `wstrust`."},
		},
	}
}

func (d *applicationDS) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return exactlyOne("id", "name", "client_id")
}

func (d *applicationDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m applicationModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()
	if id == "" {
		field, value := "name", m.Name.ValueString()
		if value == "" {
			field, value = "clientId", m.ClientID.ValueString()
		}
		items, err := client.FindByEq(ctx, d.client.ListApplications, field, value)
		if err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "list applications", err)
			return
		}
		var matches []string
		for _, it := range items {
			if (field == "name" && it.Name == value) || (field == "clientId" && it.ClientID == value) {
				matches = append(matches, it.ID)
			}
		}
		if len(matches) != 1 {
			notFound(&resp.Diagnostics, "Application", len(matches), field+" "+value)
			return
		}
		id = matches[0]
	}
	app, err := d.client.GetApplication(ctx, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read application", err)
		return
	}
	m.ID, m.Name, m.ClientID = types.StringValue(app.ID), types.StringValue(app.Name), types.StringValue(app.ClientID)
	m.Description = types.StringValue(tfutil.Deref(app.Description))
	m.Issuer = types.StringValue(app.Issuer)
	m.TemplateID = types.StringValue(tfutil.Deref(app.TemplateID))
	m.AccessURL = types.StringValue(tfutil.Deref(app.AccessURL))
	m.IsManagementApp = tfutil.BoolValueOr(app.IsManagementApp, false)
	m.Enabled = tfutil.BoolValueOr(app.ApplicationEnabled, true)
	protos := make([]string, 0, len(app.InboundProtocols))
	for _, p := range app.InboundProtocols {
		protos = append(protos, p.Type)
	}
	m.Protocols = tfutil.SetOfStrings(protos, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---- wso2is_identity_provider ----

// NewIdentityProviderDataSource returns the wso2is_identity_provider data source.
func NewIdentityProviderDataSource() datasource.DataSource { return &idpDS{} }

type idpDS struct{ base }

type idpModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	IsPrimary            types.Bool   `tfsdk:"is_primary"`
	DefaultAuthenticator types.String `tfsdk:"default_authenticator"`
	Authenticators       types.Set    `tfsdk:"authenticators"`
}

func (d *idpDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_provider"
}

func (d *idpDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an identity provider (a connection in the Console) by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":                    lookupKey("Identity provider ID."),
			"name":                  lookupKey("Name."),
			"description":           computed("Description."),
			"enabled":               schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is enabled."},
			"is_primary":            schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is the primary identity provider."},
			"default_authenticator": computed("Default federated authenticator name."),
			"authenticators":        schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Federated authenticator names."},
		},
	}
}

func (d *idpDS) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return exactlyOne("id", "name")
}

func (d *idpDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m idpModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()
	if id == "" {
		items, err := client.FindByEq(ctx, d.client.ListIdentityProviders, "name", m.Name.ValueString())
		if err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "list identity providers", err)
			return
		}
		var matches []string
		for _, it := range items {
			if it.Name == m.Name.ValueString() {
				matches = append(matches, it.ID)
			}
		}
		if len(matches) != 1 {
			notFound(&resp.Diagnostics, "Identity provider", len(matches), "name "+m.Name.ValueString())
			return
		}
		id = matches[0]
	}
	idp, err := d.client.GetIdentityProvider(ctx, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read identity provider", err)
		return
	}
	m.ID, m.Name = tfutil.GetString(idp, "id"), tfutil.GetString(idp, "name")
	m.Description = tfutil.GetStringOrEmpty(idp, "description")
	m.Enabled = tfutil.GetBool(idp, "isEnabled", true)
	m.IsPrimary = tfutil.GetBool(idp, "isPrimary", false)
	fa := tfutil.GetMap(idp, "federatedAuthenticators")
	def, _ := fa["defaultAuthenticatorId"].(string)
	names := []string{}
	for _, a := range tfutil.GetObjects(fa, "authenticators") {
		n, _ := a["name"].(string)
		names = append(names, n)
		if aid, _ := a["authenticatorId"].(string); aid == def {
			m.DefaultAuthenticator = types.StringValue(n)
		}
	}
	if m.DefaultAuthenticator.IsNull() || m.DefaultAuthenticator.IsUnknown() {
		m.DefaultAuthenticator = types.StringValue("")
	}
	m.Authenticators = tfutil.SetOfStrings(names, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---- wso2is_user ----

// NewUserDataSource returns the wso2is_user data source.
func NewUserDataSource() datasource.DataSource { return &userDS{} }

type userDS struct{ base }

type userModel struct {
	ID         types.String `tfsdk:"id"`
	Username   types.String `tfsdk:"username"`
	UserStore  types.String `tfsdk:"user_store"`
	Email      types.String `tfsdk:"email"`
	GivenName  types.String `tfsdk:"given_name"`
	FamilyName types.String `tfsdk:"family_name"`
	GroupIDs   types.Set    `tfsdk:"group_ids"`
	RoleIDs    types.Set    `tfsdk:"role_ids"`
}

func (d *userDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *userDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a user by ID or username.",
		Attributes: map[string]schema.Attribute{
			"id":          lookupKey("User ID."),
			"username":    lookupKey("Username (without the user store prefix)."),
			"user_store":  schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "User store domain of `username`. The default is `PRIMARY`."},
			"email":       computed("Primary email."),
			"given_name":  computed("First name."),
			"family_name": computed("Last name."),
			"group_ids":   schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IDs of the user's groups."},
			"role_ids":    schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IDs of the user's roles (including through groups)."},
		},
	}
}

func (d *userDS) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return exactlyOne("id", "username")
}

func (d *userDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m userModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var u map[string]any
	var err error
	if id := m.ID.ValueString(); id != "" {
		u, err = d.client.GetUser(ctx, id)
	} else {
		name := m.Username.ValueString()
		if us := m.UserStore.ValueString(); us != "" && !strings.EqualFold(us, "PRIMARY") {
			name = strings.ToUpper(us) + "/" + name
		}
		var users []map[string]any
		users, err = client.FindByEq(ctx, d.client.FindUsers, "userName", name)
		if err == nil {
			// User names are case-insensitive in the default user stores.
			var matches []string
			for _, cand := range users {
				if un, _ := cand["userName"].(string); strings.EqualFold(un, name) {
					id, _ := cand["id"].(string)
					matches = append(matches, id)
				}
			}
			if len(matches) != 1 {
				notFound(&resp.Diagnostics, "User", len(matches), "username "+name)
				return
			}
			u, err = d.client.GetUser(ctx, matches[0])
		}
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read user", err)
		return
	}
	m.ID = tfutil.GetString(u, "id")
	full, _ := u["userName"].(string)
	m.UserStore, m.Username = types.StringValue("PRIMARY"), types.StringValue(full)
	if i := strings.Index(full, "/"); i > 0 {
		m.UserStore, m.Username = types.StringValue(full[:i]), types.StringValue(full[i+1:])
	}
	m.Email = types.StringValue("")
	for _, e := range tfutil.GetObjects(u, "emails") {
		if p, _ := e["primary"].(bool); p || m.Email.ValueString() == "" {
			v, _ := e["value"].(string)
			m.Email = types.StringValue(v)
		}
	}
	name := tfutil.GetMap(u, "name")
	m.GivenName, m.FamilyName = tfutil.GetStringOrEmpty(name, "givenName"), tfutil.GetStringOrEmpty(name, "familyName")
	m.GroupIDs = refs(u, "groups")
	m.RoleIDs = refs(u, "roles")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func refs(u map[string]any, k string) types.Set {
	ids := []string{}
	for _, r := range tfutil.GetObjects(u, k) {
		if v, _ := r["value"].(string); v != "" {
			ids = append(ids, v)
		}
	}
	sort.Strings(ids)
	return tfutil.SetOfStrings(ids, false)
}

// ---- wso2is_group ----

// NewGroupDataSource returns the wso2is_group data source.
func NewGroupDataSource() datasource.DataSource { return &groupDS{} }

type groupDS struct{ base }

type groupModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	UserStore types.String `tfsdk:"user_store"`
	MemberIDs types.Set    `tfsdk:"member_ids"`
}

func (d *groupDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a group by ID or name.",
		Attributes: map[string]schema.Attribute{
			"id":         lookupKey("Group ID."),
			"name":       lookupKey("Group name (without the user store prefix)."),
			"user_store": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "User store domain of `name`. The default is `PRIMARY`."},
			"member_ids": schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "IDs of member users."},
		},
	}
}

func (d *groupDS) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return exactlyOne("id", "name")
}

func (d *groupDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m groupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var g *client.Group
	var err error
	if id := m.ID.ValueString(); id != "" {
		g, err = d.client.GetGroup(ctx, id)
	} else {
		name := m.Name.ValueString()
		if us := m.UserStore.ValueString(); us != "" && !strings.EqualFold(us, "PRIMARY") {
			name = strings.ToUpper(us) + "/" + name
		}
		var groups []client.Group
		groups, err = client.FindByEq(ctx, d.client.FindGroups, "displayName", name)
		if err == nil {
			var matches []string
			for _, cand := range groups {
				if strings.EqualFold(cand.DisplayName, name) {
					matches = append(matches, cand.ID)
				}
			}
			if len(matches) != 1 {
				notFound(&resp.Diagnostics, "Group", len(matches), "name "+name)
				return
			}
			g, err = d.client.GetGroup(ctx, matches[0])
		}
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read group", err)
		return
	}
	m.ID = types.StringValue(g.ID)
	m.UserStore, m.Name = types.StringValue("PRIMARY"), types.StringValue(g.DisplayName)
	if i := strings.Index(g.DisplayName, "/"); i > 0 {
		m.UserStore, m.Name = types.StringValue(g.DisplayName[:i]), types.StringValue(g.DisplayName[i+1:])
	}
	ids := []string{}
	for _, mem := range g.Members {
		ids = append(ids, mem.Value)
	}
	m.MemberIDs = tfutil.SetOfStrings(ids, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---- wso2is_role ----

// NewRoleDataSource returns the wso2is_role data source.
func NewRoleDataSource() datasource.DataSource { return &roleDS{} }

type roleDS struct{ base }

type roleModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	AudienceType types.String `tfsdk:"audience_type"`
	AudienceID   types.String `tfsdk:"audience_id"`
	Permissions  types.Set    `tfsdk:"permissions"`
}

func (d *roleDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (d *roleDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a role by ID or name, for example the built-in `admin` or `everyone` role. When you look up an application role by name, also set `audience_id`.",
		Attributes: map[string]schema.Attribute{
			"id":            lookupKey("Role ID."),
			"name":          lookupKey("Role name."),
			"audience_type": computed("`organization` or `application`."),
			"audience_id":   schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Organization or application ID the role belongs to (narrows a name lookup)."},
			"permissions":   schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Scopes granted by the role."},
		},
	}
}

func (d *roleDS) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return exactlyOne("id", "name")
}

func (d *roleDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m roleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()
	if id == "" {
		roles, err := client.FindByEq(ctx, d.client.FindRoles, "displayName", m.Name.ValueString())
		if err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "list roles", err)
			return
		}
		var matches []string
		for _, r := range roles {
			if r.DisplayName != m.Name.ValueString() {
				continue
			}
			if aud := m.AudienceID.ValueString(); aud != "" && (r.Audience == nil || r.Audience.Value != aud) {
				continue
			}
			matches = append(matches, r.ID)
		}
		if len(matches) != 1 {
			notFound(&resp.Diagnostics, "Role", len(matches), "name "+m.Name.ValueString())
			return
		}
		id = matches[0]
	}
	role, err := d.client.GetRole(ctx, id)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read role", err)
		return
	}
	m.ID, m.Name = types.StringValue(role.ID), types.StringValue(role.DisplayName)
	m.AudienceType, m.AudienceID = types.StringValue(""), types.StringValue("")
	if role.Audience != nil {
		m.AudienceType, m.AudienceID = types.StringValue(strings.ToLower(role.Audience.Type)), types.StringValue(role.Audience.Value)
	}
	perms := []string{}
	for _, p := range role.Permissions {
		perms = append(perms, p.Value)
	}
	m.Permissions = tfutil.SetOfStrings(perms, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---- wso2is_claim ----

// NewClaimDataSource returns the wso2is_claim data source.
func NewClaimDataSource() datasource.DataSource { return &claimDS{} }

type claimDS struct{ base }

type claimModel struct {
	ClaimURI           types.String `tfsdk:"claim_uri"`
	ID                 types.String `tfsdk:"id"`
	DisplayName        types.String `tfsdk:"display_name"`
	Description        types.String `tfsdk:"description"`
	Required           types.Bool   `tfsdk:"required"`
	ReadOnly           types.Bool   `tfsdk:"read_only"`
	SupportedByDefault types.Bool   `tfsdk:"supported_by_default"`
	DataType           types.String `tfsdk:"data_type"`
	AttributeMappings  types.Map    `tfsdk:"attribute_mappings"`
}

func (d *claimDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_claim"
}

func (d *claimDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a local claim by its claim URI.",
		Attributes: map[string]schema.Attribute{
			"claim_uri":            schema.StringAttribute{Required: true, MarkdownDescription: "Claim URI, for example `http://wso2.org/claims/emailaddress`."},
			"id":                   computed("Claim ID."),
			"display_name":         computed("Display name."),
			"description":          computed("Description."),
			"required":             schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a value is required."},
			"read_only":            schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether users can change it."},
			"supported_by_default": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is shown in profiles."},
			"data_type":            computed("Data type."),
			"attribute_mappings":   schema.MapAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "User store attribute by user store domain."},
		},
	}
}

func (d *claimDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m claimModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := d.client.GetLocalClaim(ctx, client.ClaimID(m.ClaimURI.ValueString()))
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read claim", err)
		return
	}
	m.ID = tfutil.GetString(c, "id")
	m.DisplayName = tfutil.GetStringOrEmpty(c, "displayName")
	m.Description = tfutil.GetStringOrEmpty(c, "description")
	m.Required = tfutil.GetBool(c, "required", false)
	m.ReadOnly = tfutil.GetBool(c, "readOnly", false)
	m.SupportedByDefault = tfutil.GetBool(c, "supportedByDefault", false)
	m.DataType = tfutil.GetStringOrEmpty(c, "dataType")
	mappings := map[string]string{}
	for _, am := range tfutil.GetObjects(c, "attributeMapping") {
		us, _ := am["userstore"].(string)
		at, _ := am["mappedAttribute"].(string)
		mappings[us] = at
	}
	m.AttributeMappings = tfutil.MapValueOfStrings(mappings, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
