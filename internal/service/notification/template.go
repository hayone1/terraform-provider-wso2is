package notification

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

func stringDefault(s string) defaults.String { return stringdefault.StaticString(s) }

func keepString() []planmodifier.String { return []planmodifier.String{tfutil.KeepStateString()} }

// ---- wso2is_notification_template ----

var (
	_ resource.ResourceWithConfigure   = (*templateResource)(nil)
	_ resource.ResourceWithImportState = (*templateResource)(nil)
)

// NewTemplateResource returns the wso2is_notification_template resource.
func NewTemplateResource() resource.Resource { return &templateResource{} }

type templateResource struct{ client *client.Client }

type templateModel struct {
	ID            types.String `tfsdk:"id"`
	Channel       types.String `tfsdk:"channel"`
	Type          types.String `tfsdk:"type"`
	Locale        types.String `tfsdk:"locale"`
	ApplicationID types.String `tfsdk:"application_id"`
	ContentType   types.String `tfsdk:"content_type"`
	Subject       types.String `tfsdk:"subject"`
	Body          types.String `tfsdk:"body"`
	Footer        types.String `tfsdk:"footer"`
}

func (r *templateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_template"
}

func requiresReplace() []planmodifier.String {
	return []planmodifier.String{stringplanmodifier.RequiresReplace()}
}

func (r *templateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Customizes an email or SMS notification template in one language (locale). The template applies to the whole organization or to one application. When you destroy the resource, the server goes back to the system template.\n\nTemplates use placeholders, such as `{{user-name}}`, `{{confirmation-code}}`, `{{organization-name}}` and `{{ url:...}}`. Template types include `AccountConfirmation`, `AskPassword`, `PasswordReset`, `passwordResetSucess`, `ResendAccountConfirmation`, `EmailOTP`, `SMSOTP`, `AccountLockFailedAttempt`, `AccountUnlockAdmin`, `AccountDisable`, `OrganizationUserInvitation`, `magicLink` and `TemporaryPassword`. You can also use your own types from `wso2is_notification_template_type`. To import, use the ID `<channel>/<type>/<locale>` or `<channel>/<type>/<locale>/<application_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("`<channel>/<type>/<locale>[/<application_id>]`."),
			"channel": schema.StringAttribute{
				MarkdownDescription: "`email` or `sms`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("email", "sms")},
				PlanModifiers:       requiresReplace(),
			},
			"type":   schema.StringAttribute{MarkdownDescription: "Template type name, for example `AccountConfirmation`.", Required: true, PlanModifiers: requiresReplace()},
			"locale": schema.StringAttribute{MarkdownDescription: "Locale such as `en_US` or `fr_FR`. The default is `en_US`.", Optional: true, Computed: true, Default: stringDefault("en_US"), PlanModifiers: requiresReplace()},
			"application_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the application that this template is for. When it is not set, the template is the organization's template.",
				Optional:            true,
				PlanModifiers:       requiresReplace(),
			},
			"content_type": schema.StringAttribute{MarkdownDescription: "Email content type. The default is `text/html`.", Optional: true, Computed: true, Default: stringDefault("text/html")},
			"subject":      schema.StringAttribute{MarkdownDescription: "Email subject (email templates).", Optional: true},
			"body":         schema.StringAttribute{MarkdownDescription: "Message body (HTML for email).", Required: true},
			"footer":       schema.StringAttribute{MarkdownDescription: "Email footer.", Optional: true},
		},
	}
}

func (r *templateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func templateBody(m *templateModel, withLocale bool) map[string]any {
	b := map[string]any{"body": m.Body.ValueString()}
	if m.Channel.ValueString() == "email" {
		b["contentType"] = m.ContentType.ValueString()
		b["subject"] = m.Subject.ValueString()
		if v := m.Footer.ValueString(); v != "" {
			b["footer"] = v
		}
	}
	if withLocale {
		b["locale"] = m.Locale.ValueString()
	}
	return b
}

func (r *templateResource) path(m *templateModel, withLocale bool) string {
	locale := ""
	if withLocale {
		locale = m.Locale.ValueString()
	}
	return r.client.NotificationTemplatePath(m.Channel.ValueString(), m.Type.ValueString(), m.ApplicationID.ValueString(), locale)
}

func (r *templateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Channel.ValueString() == "email" && plan.Subject.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("subject"), "Missing subject", "Email templates need a subject.")
		return
	}
	_, err := r.client.Post(ctx, r.path(&plan, false), templateBody(&plan, true), nil)
	if client.IsConflict(err) {
		// A template for the locale already exists (created in the Console): take it over.
		_, err = r.client.Put(ctx, r.path(&plan, true), templateBody(&plan, false), nil)
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create notification template", err)
		return
	}
	plan.ID = types.StringValue(templateID(&plan))
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func templateID(m *templateModel) string {
	id := m.Channel.ValueString() + "/" + m.Type.ValueString() + "/" + m.Locale.ValueString()
	if v := m.ApplicationID.ValueString(); v != "" {
		id += "/" + v
	}
	return id
}

func (r *templateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state templateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tfutil.MissingID(state.ID) {
		resp.State.RemoveResource(ctx)
		return
	}
	// ID: <channel>/<type>/<locale>[/<application_id>].
	if parts := tfutil.IDParts(state.ID, 3); parts != nil {
		tfutil.SetStringIfNull(&state.Channel, parts[0])
		tfutil.SetStringIfNull(&state.Type, parts[1])
		tfutil.SetStringIfNull(&state.Locale, parts[2])
		if len(parts) == 4 {
			tfutil.SetStringIfNull(&state.ApplicationID, parts[3])
		}
	}
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *templateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan templateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Put(ctx, r.path(&plan, true), templateBody(&plan, false), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update notification template", err)
		return
	}
	plan.ID = types.StringValue(templateID(&plan))
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state templateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.path(&state, true)); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete notification template", err)
	}
}

func (r *templateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) < 3 || len(parts) > 4 {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <channel>/<type>/<locale>[/<application_id>], got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("channel"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("type"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("locale"), parts[2])...)
	if len(parts) == 4 {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), parts[3])...)
	}
}

func (r *templateResource) read(ctx context.Context, m *templateModel, diags *diag.Diagnostics) bool {
	var t map[string]any
	if err := r.client.Get(ctx, r.path(m, true), nil, &t); err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read notification template", err)
		return false
	}
	m.Body = tfutil.GetString(t, "body")
	if m.Channel.ValueString() == "email" {
		m.ContentType = tfutil.GetString(t, "contentType")
		m.Subject = tfutil.GetString(t, "subject")
		m.Footer = tfutil.StringValueOrNull(tfutil.GetStringOrEmpty(t, "footer").ValueString())
	} else if m.ContentType.IsNull() || m.ContentType.IsUnknown() {
		m.ContentType = types.StringValue("text/html")
	}
	return true
}

// ---- wso2is_notification_template_type ----

var (
	_ resource.ResourceWithConfigure   = (*templateTypeResource)(nil)
	_ resource.ResourceWithImportState = (*templateTypeResource)(nil)
)

// NewTemplateTypeResource returns the wso2is_notification_template_type resource.
func NewTemplateTypeResource() resource.Resource { return &templateTypeResource{} }

type templateTypeResource struct{ client *client.Client }

type templateTypeModel struct {
	ID      types.String `tfsdk:"id"`
	Channel types.String `tfsdk:"channel"`
	Name    types.String `tfsdk:"name"`
}

func (r *templateTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notification_template_type"
}

func (r *templateTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates a custom notification template type. Use a custom type for notifications that your own flows or adaptive scripts send. Deleting the type also deletes its templates. To import, use the ID `<channel>/<name>`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("`<channel>/<name>`."),
			"channel": schema.StringAttribute{
				MarkdownDescription: "`email` or `sms`.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("email", "sms")},
				PlanModifiers:       requiresReplace(),
			},
			"name": schema.StringAttribute{MarkdownDescription: "Template type name.", Required: true, PlanModifiers: requiresReplace()},
		},
	}
}

func (r *templateTypeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *templateTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan templateTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.CreateTemplateType(ctx, plan.Channel.ValueString(), plan.Name.ValueString()); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create notification template type", err)
		return
	}
	plan.ID = types.StringValue(plan.Channel.ValueString() + "/" + plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *templateTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state templateTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if tfutil.MissingID(state.ID) {
		resp.State.RemoveResource(ctx)
		return
	}
	if _, err := r.client.GetTemplateType(ctx, state.Channel.ValueString(), state.Name.ValueString()); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		tfutil.AddAPIError(&resp.Diagnostics, "read notification template type", err)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *templateTypeResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *templateTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state templateTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteTemplateType(ctx, state.Channel.ValueString(), state.Name.ValueString()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete notification template type", err)
	}
}

func (r *templateTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	channel, name, ok := strings.Cut(req.ID, "/")
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <channel>/<name>, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("channel"), channel)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
