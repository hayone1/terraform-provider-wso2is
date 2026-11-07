// Package branding implements branding preferences and custom text.
package branding

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// target identifies a branding preference: the organization or an application, in a locale.
type target struct {
	Type, Name, Locale, Screen string
}

func (t target) query() url.Values {
	q := url.Values{"type": {t.Type}, "name": {t.Name}, "locale": {t.Locale}}
	if t.Screen != "" {
		q.Set("screen", t.Screen)
	}
	return q
}

func (t target) id() string {
	parts := []string{t.Type, t.Name, t.Locale}
	if t.Screen != "" {
		parts = append(parts, t.Screen)
	}
	return strings.Join(parts, "/")
}

func commonAttrs(c *client.Client) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id": tfutil.IDAttribute("`<type>/<name>/<locale>`."),
		"type": schema.StringAttribute{
			MarkdownDescription: "`ORG` (the organization, default) or `APP` (one application).",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("ORG"),
			Validators:          []validator.String{stringvalidator.OneOf("ORG", "APP")},
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"application_id": schema.StringAttribute{
			MarkdownDescription: "Application ID, for `type = \"APP\"`.",
			Optional:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"locale": schema.StringAttribute{
			MarkdownDescription: "Locale, for example `en-US` (default) or `fr-FR`.",
			Optional:            true,
			Computed:            true,
			Default:             stringdefault.StaticString("en-US"),
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
	}
}

// fillTargetFromID fills target attributes that are missing from the prior state from the
// ID (<type>/<name>/<locale>[/<screen>]).
func fillTargetFromID(id types.String, typ, appID, locale, screen *types.String) {
	parts := tfutil.IDParts(id, 3)
	if parts == nil {
		return
	}
	tfutil.SetStringIfNull(typ, parts[0])
	if parts[0] == "APP" {
		tfutil.SetStringIfNull(appID, parts[1])
	}
	tfutil.SetStringIfNull(locale, parts[2])
	if screen != nil && len(parts) > 3 {
		tfutil.SetStringIfNull(screen, parts[3])
	}
}

func resolveTarget(c *client.Client, typ, appID, locale, screen types.String, diags *diag.Diagnostics) target {
	t := target{Type: typ.ValueString(), Locale: locale.ValueString(), Screen: screen.ValueString()}
	if t.Type == "APP" {
		if appID.ValueString() == "" {
			diags.AddAttributeError(path.Root("application_id"), "Missing application ID", "application_id is required when type is \"APP\".")
		}
		t.Name = appID.ValueString()
	} else {
		t.Name = c.TenantDomain()
		if org := c.OrganizationID(); org != "" {
			t.Name = org
		}
	}
	return t
}

// ---- wso2is_branding_preference ----

var (
	_ resource.ResourceWithConfigure   = (*preferenceResource)(nil)
	_ resource.ResourceWithImportState = (*preferenceResource)(nil)
)

// NewPreferenceResource returns the wso2is_branding_preference resource.
func NewPreferenceResource() resource.Resource { return &preferenceResource{} }

type preferenceResource struct{ client *client.Client }

type preferenceModel struct {
	ID            types.String         `tfsdk:"id"`
	Type          types.String         `tfsdk:"type"`
	ApplicationID types.String         `tfsdk:"application_id"`
	Locale        types.String         `tfsdk:"locale"`
	Preference    jsontypes.Normalized `tfsdk:"preference"`
}

func (r *preferenceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_branding_preference"
}

func (r *preferenceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := commonAttrs(nil)
	attrs["preference"] = schema.StringAttribute{
		MarkdownDescription: "Branding preference document (use `jsonencode`), as in the Console's *Branding* section: `configs` (`isBrandingEnabled`, `removeDefaultBranding`, `selfSignUpEnabled`), `organizationDetails` (`displayName`, `supportEmail`, `siteTitle`, `copyrightText`), `images` (`logo`, `favicon`, `myAccountLogo`), `urls` (`privacyPolicyURL`, `termsOfUseURL`, `cookiePolicyURL`), `theme` (`activeTheme`, `LIGHT`/`DARK` colors, fonts, buttons, inputs, `loginBox`, `footer`), and `layout` (`activeLayout`: `centered`, `left-image`, `right-aligned`, `custom`, ...). The server stores the document as given.",
		Required:            true,
		CustomType:          jsontypes.NormalizedType{},
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the branding of the login, registration, account recovery and My Account pages. The branding applies to the whole organization or to one application. To import, use the ID `ORG/<tenant>/<locale>` or `APP/<application_id>/<locale>`.",
		Attributes:          attrs,
	}
}

func (r *preferenceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *preferenceResource) body(t target, pref string) map[string]any {
	return map[string]any{"type": t.Type, "name": t.Name, "locale": t.Locale, "preference": json.RawMessage(pref)}
}

func (r *preferenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan preferenceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	t := resolveTarget(r.client, plan.Type, plan.ApplicationID, plan.Locale, types.StringNull(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Post(ctx, r.client.ServerPath("/branding-preference"), r.body(t, plan.Preference.ValueString()), nil)
	if client.IsConflict(err) {
		_, err = r.client.Put(ctx, r.client.ServerPath("/branding-preference"), r.body(t, plan.Preference.ValueString()), nil)
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create branding preference", err)
		return
	}
	plan.ID = types.StringValue(t.id())
	r.read(ctx, &plan, t, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *preferenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state preferenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	fillTargetFromID(state.ID, &state.Type, &state.ApplicationID, &state.Locale, nil)
	t := resolveTarget(r.client, state.Type, state.ApplicationID, state.Locale, types.StringNull(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, t, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *preferenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan preferenceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	t := resolveTarget(r.client, plan.Type, plan.ApplicationID, plan.Locale, types.StringNull(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Put(ctx, r.client.ServerPath("/branding-preference"), r.body(t, plan.Preference.ValueString()), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update branding preference", err)
		return
	}
	plan.ID = types.StringValue(t.id())
	r.read(ctx, &plan, t, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *preferenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state preferenceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	t := resolveTarget(r.client, state.Type, state.ApplicationID, state.Locale, types.StringNull(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Do(ctx, client.Request{Method: http.MethodDelete, Path: r.client.ServerPath("/branding-preference"), Query: t.query()})
	if err != nil && !isNotConfigured(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete branding preference", err)
	}
}

func (r *preferenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTarget(ctx, req, resp, 3)
}

func importTarget(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse, n int) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != n {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected %d parts separated by '/', got %q", n, req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), parts[0])...)
	if parts[0] == "APP" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), parts[1])...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("locale"), parts[2])...)
	if n == 4 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("screen"), parts[3])...)
	}
}

// isNotConfigured reports the branding API's "not configured" answers.
func isNotConfigured(err error) bool {
	return client.IsNotFound(err) || client.HasCode(err, "BPM-60002", "BPM-60006", "BPM-60007")
}

func (r *preferenceResource) read(ctx context.Context, m *preferenceModel, t target, diags *diag.Diagnostics) bool {
	var out struct {
		Preference json.RawMessage `json:"preference"`
	}
	if err := r.client.Get(ctx, r.client.ServerPath("/branding-preference"), t.query(), &out); err != nil {
		if isNotConfigured(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read branding preference", err)
		return false
	}
	m.Preference = jsontypes.NewNormalizedValue(string(out.Preference))
	return true
}

// ---- wso2is_branding_text ----

var (
	_ resource.ResourceWithConfigure   = (*textResource)(nil)
	_ resource.ResourceWithImportState = (*textResource)(nil)
)

// NewTextResource returns the wso2is_branding_text resource.
func NewTextResource() resource.Resource { return &textResource{} }

type textResource struct{ client *client.Client }

type textModel struct {
	ID            types.String `tfsdk:"id"`
	Type          types.String `tfsdk:"type"`
	ApplicationID types.String `tfsdk:"application_id"`
	Locale        types.String `tfsdk:"locale"`
	Screen        types.String `tfsdk:"screen"`
	Text          types.Map    `tfsdk:"text"`
}

func (r *textResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_branding_text"
}

func (r *textResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := commonAttrs(nil)
	attrs["id"] = tfutil.IDAttribute("`<type>/<name>/<locale>/<screen>`.")
	attrs["screen"] = schema.StringAttribute{
		MarkdownDescription: "Screen whose texts are customized, for example `common`, `login`, `sign-up`, `password-recovery`, `password-reset`, `password-reset-success`, `email-otp`, `sms-otp`, `totp`, `email-link-expiry`, `myaccount`.",
		Required:            true,
		PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
	}
	attrs["text"] = schema.MapAttribute{
		MarkdownDescription: "Text values by key, for example `{ \"login.heading\" = \"Sign in to Acme\" }`. The keys are listed in the Console's *Text* tab of the branding section.",
		Required:            true,
		ElementType:         types.StringType,
	}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Changes the text on one login or self-service screen, in one language (locale). The text applies to the whole organization or to one application. To import, use the ID `ORG/<tenant>/<locale>/<screen>` or `APP/<application_id>/<locale>/<screen>`.",
		Attributes:          attrs,
	}
}

func (r *textResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *textResource) body(ctx context.Context, t target, m *textModel, diags *diag.Diagnostics) map[string]any {
	texts := map[string]any{}
	for k, v := range tfutil.MapOfStrings(ctx, m.Text, diags) {
		texts[k] = v
	}
	return map[string]any{"type": t.Type, "name": t.Name, "locale": t.Locale, "screen": t.Screen, "preference": map[string]any{"text": texts}}
}

func (r *textResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan textModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	t := resolveTarget(r.client, plan.Type, plan.ApplicationID, plan.Locale, plan.Screen, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Post(ctx, r.client.ServerPath("/branding-preference/text"), r.body(ctx, t, &plan, &resp.Diagnostics), nil)
	if client.IsConflict(err) {
		_, err = r.client.Put(ctx, r.client.ServerPath("/branding-preference/text"), r.body(ctx, t, &plan, &resp.Diagnostics), nil)
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create custom text", err)
		return
	}
	plan.ID = types.StringValue(t.id())
	r.read(ctx, &plan, t, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *textResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state textModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	fillTargetFromID(state.ID, &state.Type, &state.ApplicationID, &state.Locale, &state.Screen)
	t := resolveTarget(r.client, state.Type, state.ApplicationID, state.Locale, state.Screen, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, t, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *textResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan textModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	t := resolveTarget(r.client, plan.Type, plan.ApplicationID, plan.Locale, plan.Screen, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Put(ctx, r.client.ServerPath("/branding-preference/text"), r.body(ctx, t, &plan, &resp.Diagnostics), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update custom text", err)
		return
	}
	plan.ID = types.StringValue(t.id())
	r.read(ctx, &plan, t, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *textResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state textModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	t := resolveTarget(r.client, state.Type, state.ApplicationID, state.Locale, state.Screen, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.client.Do(ctx, client.Request{Method: http.MethodDelete, Path: r.client.ServerPath("/branding-preference/text"), Query: t.query()})
	if err != nil && !isNotConfigured(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete custom text", err)
	}
}

func (r *textResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importTarget(ctx, req, resp, 4)
}

func (r *textResource) read(ctx context.Context, m *textModel, t target, diags *diag.Diagnostics) bool {
	var out struct {
		Preference struct {
			Text map[string]string `json:"text"`
		} `json:"preference"`
	}
	if err := r.client.Get(ctx, r.client.ServerPath("/branding-preference/text"), t.query(), &out); err != nil {
		if isNotConfigured(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read custom text", err)
		return false
	}
	m.Text = tfutil.MapValueOfStrings(out.Preference.Text, false)
	return true
}
