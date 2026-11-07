// Package identity implements users, groups and roles (SCIM 2.0).
package identity

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure        = (*userResource)(nil)
	_ resource.ResourceWithImportState      = (*userResource)(nil)
	_ resource.ResourceWithConfigValidators = (*userResource)(nil)
)

// NewUserResource returns the wso2is_user resource.
func NewUserResource() resource.Resource { return &userResource{} }

type userResource struct{ client *client.Client }

type userModel struct {
	ID                types.String `tfsdk:"id"`
	Username          types.String `tfsdk:"username"`
	UserStore         types.String `tfsdk:"user_store"`
	Password          types.String `tfsdk:"password"`
	PasswordWO        types.String `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64  `tfsdk:"password_wo_version"`
	AskPassword       types.Bool   `tfsdk:"ask_password"`
	GivenName         types.String `tfsdk:"given_name"`
	FamilyName        types.String `tfsdk:"family_name"`
	DisplayName       types.String `tfsdk:"display_name"`
	NickName          types.String `tfsdk:"nick_name"`
	Email             types.String `tfsdk:"email"`
	Emails            types.Map    `tfsdk:"emails"`
	PhoneNumbers      types.Map    `tfsdk:"phone_numbers"`
	Locked            types.Bool   `tfsdk:"locked"`
	Disabled          types.Bool   `tfsdk:"disabled"`
	Attributes        types.Map    `tfsdk:"attributes"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a user through the SCIM 2.0 API.\n\nThe provider only manages the attributes in your configuration. It leaves alone the attributes that the user sets in My Account, or that other systems set.\n\nThe server never sends passwords back. As a result, the provider cannot notice a password that someone changed outside Terraform. Use `password_wo` (Terraform 1.11 or later) to keep the password out of the state, or `ask_password` to let the user choose a password. To import a user, use the user's ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("User ID (SCIM `id`)."),
			"username": schema.StringAttribute{
				MarkdownDescription: "Username, without the user store prefix. Changing it replaces the user (WSO2 does not support renaming).",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_store": schema.StringAttribute{
				MarkdownDescription: "User store domain. The default is `PRIMARY`. Changing it replaces the user.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("PRIMARY"),
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "The user's password. It must meet the tenant's password policy. Terraform stores it in the state. Use `password_wo` instead to keep it out of the state.",
				Optional:            true,
				Sensitive:           true,
			},
			"password_wo": schema.StringAttribute{
				MarkdownDescription: "A write-only password (Terraform 1.11 or later). The provider sends it to the server but never stores it in the state. To set a new password, change it and increase `password_wo_version`.",
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
			},
			"password_wo_version": schema.Int64Attribute{
				MarkdownDescription: "Increase this number to send a new `password_wo`.",
				Optional:            true,
			},
			"ask_password": schema.BoolAttribute{
				MarkdownDescription: "Create the user without a password and email them a link to set one (needs an email address and the Ask Password governance setting). The provider only uses this value when it creates the user.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"given_name":   schema.StringAttribute{MarkdownDescription: "First name (`name.givenName`).", Optional: true},
			"family_name":  schema.StringAttribute{MarkdownDescription: "Last name (`name.familyName`).", Optional: true},
			"display_name": schema.StringAttribute{MarkdownDescription: "Display name.", Optional: true},
			"nick_name":    schema.StringAttribute{MarkdownDescription: "Nickname.", Optional: true},
			"email": schema.StringAttribute{
				MarkdownDescription: "Primary email address (`http://wso2.org/claims/emailaddress`), used for notifications and account recovery.",
				Optional:            true,
			},
			"emails": schema.MapAttribute{
				MarkdownDescription: "More email addresses, by type (`work`, `home` or `other`). For several verified addresses, use the `urn:scim:wso2:schema:emailAddresses` attribute instead.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"phone_numbers": schema.MapAttribute{
				MarkdownDescription: "Phone numbers by type: `mobile`, `home`, `work`, `other`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"locked": schema.BoolAttribute{
				MarkdownDescription: "Whether the account is locked (`urn:scim:wso2:schema:accountLocked`). The server also locks the account after failed logins when account locking is on.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool()},
			},
			"disabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the account is disabled. This sets `urn:scim:wso2:schema:accountDisabled`, which needs the account disabling governance connector (`account.disable.handler`).",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool()},
			},
			"attributes": schema.MapAttribute{
				MarkdownDescription: "Other SCIM attributes, by path. Core attributes use their name, such as `title`, `locale` or `name.middleName`. Extension attributes start with their schema URN, for example `urn:scim:wso2:schema:country`, `urn:ietf:params:scim:schemas:extension:enterprise:2.0:User:department` or `urn:scim:schemas:extension:custom:User:<custom claim>`. All values are strings. The provider converts booleans and numbers with the server's SCIM schema. Separate the values of a multi-valued attribute with commas.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *userResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.Conflicting(path.MatchRoot("password"), path.MatchRoot("password_wo")),
		resourcevalidator.RequiredTogether(path.MatchRoot("password_wo"), path.MatchRoot("password_wo_version")),
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func fullUsername(store, username string) string {
	if store == "" || strings.EqualFold(store, "PRIMARY") {
		return username
	}
	return strings.ToUpper(store) + "/" + username
}

func splitUsername(full string) (store, username string) {
	if i := strings.Index(full, "/"); i > 0 {
		return full[:i], full[i+1:]
	}
	return "PRIMARY", full
}

// userBody builds the SCIM representation of the managed attributes.
func (r *userResource) userBody(ctx context.Context, m *userModel, diags *diag.Diagnostics) map[string]any {
	body := map[string]any{
		"schemas":  []string{client.SCIMCoreUser},
		"userName": fullUsername(m.UserStore.ValueString(), m.Username.ValueString()),
	}
	name := map[string]any{}
	if tfutil.Known(m.GivenName) {
		name["givenName"] = m.GivenName.ValueString()
	}
	if tfutil.Known(m.FamilyName) {
		name["familyName"] = m.FamilyName.ValueString()
	}
	if len(name) > 0 {
		body["name"] = name
	}
	if tfutil.Known(m.DisplayName) {
		body["displayName"] = m.DisplayName.ValueString()
	}
	if tfutil.Known(m.NickName) {
		body["nickName"] = m.NickName.ValueString()
	}
	if emails := emailList(ctx, m, diags); len(emails) > 0 {
		body["emails"] = emails
	}
	if phones := tfutil.MapOfStrings(ctx, m.PhoneNumbers, diags); len(phones) > 0 {
		list := make([]map[string]any, 0, len(phones))
		for _, t := range tfutil.SortedKeys(phones) {
			list = append(list, map[string]any{"type": t, "value": phones[t]})
		}
		body["phoneNumbers"] = list
	}
	wso2 := map[string]any{}
	if tfutil.Known(m.Locked) {
		wso2["accountLocked"] = m.Locked.ValueBool()
	}
	if tfutil.Known(m.Disabled) {
		wso2["accountDisabled"] = m.Disabled.ValueBool()
	}
	if len(wso2) > 0 {
		body[client.SCIMWSO2User] = wso2
	}
	if attrs := tfutil.MapOfStrings(ctx, m.Attributes, diags); len(attrs) > 0 {
		types, err := r.client.SCIMUserAttributeTypes(ctx)
		if err != nil {
			tfutil.AddAPIError(diags, "read SCIM schemas", err)
			return nil
		}
		for _, k := range tfutil.SortedKeys(attrs) {
			v, err := client.SCIMValue(types, k, attrs[k])
			if err != nil {
				diags.AddAttributeError(path.Root("attributes").AtMapKey(k), "Invalid attribute value", err.Error())
				continue
			}
			client.SetSCIM(body, k, v)
		}
	}
	addSchemas(body)
	return body
}

// addSchemas lists every extension schema used in the body.
func addSchemas(body map[string]any) {
	schemas := []string{client.SCIMCoreUser}
	for k := range body {
		if strings.HasPrefix(k, "urn:") {
			schemas = append(schemas, k)
		}
	}
	sort.Strings(schemas[1:])
	body["schemas"] = schemas
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, cfg userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := r.userBody(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	switch {
	case tfutil.Known(plan.Password):
		body["password"] = plan.Password.ValueString()
	case tfutil.Known(cfg.PasswordWO):
		body["password"] = cfg.PasswordWO.ValueString()
	}
	if plan.AskPassword.ValueBool() {
		wso2, _ := body[client.SCIMWSO2User].(map[string]any)
		if wso2 == nil {
			wso2 = map[string]any{}
		}
		wso2["askPassword"] = true
		body[client.SCIMWSO2User] = wso2
		addSchemas(body)
	}
	out, err := r.client.CreateUser(ctx, body)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create user", err)
		return
	}
	id, _ := out["id"].(string)
	// An inbound provisioning user store (tenant-wide, or of the application whose token
	// the provider uses) overrides the DOMAIN/ prefix without an error.
	want, _ := splitUsername(fullUsername(plan.UserStore.ValueString(), plan.Username.ValueString()))
	if got, _ := splitUsername(fmt.Sprint(out["userName"])); id != "" && !strings.EqualFold(got, want) {
		if err := r.client.DeleteUser(ctx, id); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "delete the user created in the wrong user store", err)
		}
		resp.Diagnostics.AddAttributeError(path.Root("user_store"), "User created in the wrong user store",
			fmt.Sprintf("The server created the user in %s instead of %s, so it was deleted again. An inbound provisioning user store is forcing SCIM users into %s: "+
				"clear provisioning_userstore of wso2is_inbound_provisioning_configuration (basic authentication) or the inbound provisioning user store "+
				"of the application whose credentials the provider uses.", got, want, got))
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("User not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
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

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, cfg userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var ops []client.SCIMPatchOperation
	replace := map[string]any{}
	removeIfNull := func(p string, v types.String, old types.String) {
		switch {
		case tfutil.Known(v) && !v.Equal(old):
			client.SetSCIM(replace, p, v.ValueString())
		case v.IsNull() && !old.IsNull():
			// WSO2 accepts `remove` for core sub-attributes but ignores it; an empty
			// value clears them.
			client.SetSCIM(replace, p, "")
		}
	}
	removeIfNull("name.givenName", plan.GivenName, state.GivenName)
	removeIfNull("name.familyName", plan.FamilyName, state.FamilyName)
	removeIfNull("displayName", plan.DisplayName, state.DisplayName)
	removeIfNull("nickName", plan.NickName, state.NickName)

	newBody := r.userBody(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.Email.Equal(state.Email) || !plan.Emails.Equal(state.Emails) {
		if plan.Email.IsNull() && !state.Email.IsNull() {
			ops = append(ops, client.SCIMPatchOperation{Op: "remove", Path: "emails[primary eq true]"})
		}
		oldTyped := tfutil.MapOfStrings(ctx, state.Emails, &resp.Diagnostics)
		newTyped := tfutil.MapOfStrings(ctx, plan.Emails, &resp.Diagnostics)
		for t := range oldTyped {
			if _, ok := newTyped[t]; !ok {
				ops = append(ops, client.SCIMPatchOperation{Op: "remove", Path: fmt.Sprintf("emails[type eq %q]", t)})
			}
		}
		if e, ok := newBody["emails"]; ok {
			replace["emails"] = e
		}
	}
	if !plan.PhoneNumbers.Equal(state.PhoneNumbers) {
		oldPhones := tfutil.MapOfStrings(ctx, state.PhoneNumbers, &resp.Diagnostics)
		newPhones := tfutil.MapOfStrings(ctx, plan.PhoneNumbers, &resp.Diagnostics)
		for t := range oldPhones {
			if _, ok := newPhones[t]; !ok {
				ops = append(ops, client.SCIMPatchOperation{Op: "remove", Path: fmt.Sprintf("phoneNumbers[type eq %q]", t)})
			}
		}
		if len(newPhones) > 0 {
			replace["phoneNumbers"] = newBody["phoneNumbers"]
		}
	}
	wso2 := map[string]any{}
	if tfutil.Known(plan.Locked) && !plan.Locked.Equal(state.Locked) {
		wso2["accountLocked"] = plan.Locked.ValueBool()
	}
	if tfutil.Known(plan.Disabled) && !plan.Disabled.Equal(state.Disabled) {
		wso2["accountDisabled"] = plan.Disabled.ValueBool()
	}
	if len(wso2) > 0 {
		replace[client.SCIMWSO2User] = wso2
	}
	if !plan.Attributes.Equal(state.Attributes) {
		oldAttrs := tfutil.MapOfStrings(ctx, state.Attributes, &resp.Diagnostics)
		newAttrs := tfutil.MapOfStrings(ctx, plan.Attributes, &resp.Diagnostics)
		for k := range oldAttrs {
			if _, ok := newAttrs[k]; !ok {
				if schema, _ := client.SplitSCIMPath(k); schema == "" {
					client.SetSCIM(replace, k, "")
				} else {
					ops = append(ops, client.SCIMPatchOperation{Op: "remove", Path: k})
				}
			}
		}
		for k, v := range newAttrs {
			if oldAttrs[k] != v {
				if val, ok := client.LookupSCIM(newBody, k); ok {
					client.SetSCIM(replace, k, val)
				}
				_ = v
			}
		}
	}
	switch {
	case tfutil.Known(plan.Password) && !plan.Password.Equal(state.Password):
		replace["password"] = plan.Password.ValueString()
	case tfutil.Known(cfg.PasswordWO) && !plan.PasswordWOVersion.Equal(state.PasswordWOVersion):
		replace["password"] = cfg.PasswordWO.ValueString()
	}
	if len(replace) > 0 {
		ops = append(ops, client.SCIMPatchOperation{Op: "replace", Value: replace})
	}
	if err := r.client.PatchUser(ctx, state.ID.ValueString(), ops); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update user", err)
		return
	}
	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("User not found after update", plan.ID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteUser(ctx, state.ID.ValueString()); err != nil && !client.IsSCIMNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete user", err)
	}
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// read refreshes the managed attributes. Attributes that are not in state (not managed)
// are not populated, except on import where the common ones are.
func (r *userResource) read(ctx context.Context, m *userModel, diags *diag.Diagnostics) bool {
	u, err := r.client.GetUser(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsSCIMNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read user", err)
		return false
	}
	importing := m.Username.IsNull()
	userName, _ := u["userName"].(string)
	store, username := splitUsername(userName)
	m.Username = types.StringValue(username)
	if m.UserStore.IsNull() || !strings.EqualFold(m.UserStore.ValueString(), store) {
		m.UserStore = types.StringValue(store)
	}
	if m.AskPassword.IsNull() {
		m.AskPassword = types.BoolValue(false)
	}

	str := func(p string, cur types.String) types.String {
		v, ok := client.LookupSCIM(u, p)
		s, _ := client.SCIMString(v)
		if !ok || s == "" {
			return types.StringNull()
		}
		if cur.IsNull() && !importing {
			// Not managed: leave it out of state.
			return types.StringNull()
		}
		return types.StringValue(s)
	}
	m.GivenName = str("name.givenName", m.GivenName)
	m.FamilyName = str("name.familyName", m.FamilyName)
	m.DisplayName = str("displayName", m.DisplayName)
	m.NickName = str("nickName", m.NickName)

	primary, typed := "", map[string]string{}
	for _, e := range toObjects(u["emails"]) {
		v, _ := e["value"].(string)
		t, _ := e["type"].(string)
		if p, _ := e["primary"].(bool); p || t == "" {
			if primary == "" {
				primary = v
			}
			continue
		}
		typed[t] = v
	}
	if !m.Email.IsNull() || importing {
		m.Email = tfutil.StringValueOrNull(primary)
	}
	if !m.Emails.IsNull() || importing {
		m.Emails = tfutil.MapValueOfStrings(typed, true)
	}
	if !m.PhoneNumbers.IsNull() || importing {
		phones := map[string]string{}
		for _, p := range toObjects(u["phoneNumbers"]) {
			t, _ := p["type"].(string)
			v, _ := p["value"].(string)
			if t != "" && v != "" {
				phones[t] = v
			}
		}
		m.PhoneNumbers = tfutil.MapValueOfStrings(phones, true)
	}
	m.Locked = boolAttr(u, client.SCIMWSO2User+":accountLocked")
	m.Disabled = boolAttr(u, client.SCIMWSO2User+":accountDisabled")

	if !m.Attributes.IsNull() {
		managed := m.Attributes.Elements()
		out := map[string]string{}
		for k := range managed {
			v, ok := client.LookupSCIM(u, k)
			if s, has := client.SCIMString(v); ok && has {
				out[k] = s
			}
		}
		m.Attributes = tfutil.MapValueOfStrings(out, true)
	}
	return true
}

func boolAttr(u map[string]any, p string) types.Bool {
	v, ok := client.LookupSCIM(u, p)
	if !ok {
		return types.BoolValue(false)
	}
	switch t := v.(type) {
	case bool:
		return types.BoolValue(t)
	case string:
		return types.BoolValue(strings.EqualFold(t, "true"))
	}
	return types.BoolValue(false)
}

// toObjects returns the elements of a SCIM multi-valued attribute. WSO2 IS 7.2 and
// earlier return untyped values as plain strings ("emails": ["a@example.com"]); they
// become {"value": ...}.
func toObjects(v any) []map[string]any {
	arr, _ := v.([]any)
	out := make([]map[string]any, 0, len(arr))
	for _, e := range arr {
		switch t := e.(type) {
		case map[string]any:
			out = append(out, t)
		case string:
			out = append(out, map[string]any{"value": t})
		}
	}
	return out
}

// emailList renders the primary and typed emails as the SCIM emails array.
func emailList(ctx context.Context, m *userModel, diags *diag.Diagnostics) []map[string]any {
	var list []map[string]any
	if tfutil.Known(m.Email) && m.Email.ValueString() != "" {
		list = append(list, map[string]any{"value": m.Email.ValueString(), "primary": true})
	}
	typed := tfutil.MapOfStrings(ctx, m.Emails, diags)
	for _, t := range tfutil.SortedKeys(typed) {
		list = append(list, map[string]any{"value": typed[t], "type": t})
	}
	return list
}
