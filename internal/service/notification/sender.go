// Package notification implements notification senders (email, SMS) and templates.
package notification

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// ---- wso2is_email_sender ----

var (
	_ resource.ResourceWithConfigure   = (*emailSenderResource)(nil)
	_ resource.ResourceWithImportState = (*emailSenderResource)(nil)
)

// NewEmailSenderResource returns the wso2is_email_sender resource.
func NewEmailSenderResource() resource.Resource { return &emailSenderResource{} }

type emailSenderResource struct{ client *client.Client }

type emailSenderModel struct {
	ID               types.String `tfsdk:"id"`
	Provider         types.String `tfsdk:"provider_type"`
	ProviderURL      types.String `tfsdk:"provider_url"`
	SMTPHost         types.String `tfsdk:"smtp_host"`
	SMTPPort         types.Int64  `tfsdk:"smtp_port"`
	FromAddress      types.String `tfsdk:"from_address"`
	DisplayName      types.String `tfsdk:"display_name"`
	ReplyTo          types.String `tfsdk:"reply_to"`
	AuthType         types.String `tfsdk:"auth_type"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	Properties       types.Map    `tfsdk:"properties"`
	SecretProperties types.Map    `tfsdk:"secret_properties"`
}

func (r *emailSenderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_sender"
}

func (r *emailSenderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the tenant's email provider. In the Console, this is **Notification Channels > Email Provider**. The server uses it to send verification, account recovery, one-time password and notification emails. Each tenant has one email provider.\n\nThis resource does not work in the super tenant (`carbon.super`). The super tenant reads its email provider only from `deployment.toml` (`[output_adapter.email]`), and the API answers HTTP 405. Use this resource with a provider that manages another tenant. To import, use the ID `EmailPublisher`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Always `EmailPublisher`."),
			"provider_type": schema.StringAttribute{
				MarkdownDescription: "The kind of email provider. `SMTP` is the default. With `HTTP` (WSO2 IS 7.3 and later), the server posts each email to `provider_url`.",
				Optional:            true,
				Computed:            true,
				Default:             stringDefault("SMTP"),
				Validators:          []validator.String{stringvalidator.OneOf("SMTP", "HTTP")},
			},
			"provider_url": schema.StringAttribute{MarkdownDescription: "Endpoint of an HTTP email provider. Set the request template in `properties.body`.", Optional: true},
			"smtp_host":    schema.StringAttribute{MarkdownDescription: "SMTP server host (SMTP providers).", Optional: true},
			"smtp_port":    schema.Int64Attribute{MarkdownDescription: "SMTP server port, for example `587`.", Optional: true},
			"from_address": schema.StringAttribute{MarkdownDescription: "Sender address.", Required: true},
			"display_name": schema.StringAttribute{MarkdownDescription: "Sender name shown to recipients (`mail.smtp.signature`). SMTP providers need this value.", Optional: true},
			"reply_to":     schema.StringAttribute{MarkdownDescription: "Reply-to address (`mail.smtp.replyTo`). SMTP providers need this value.", Optional: true},
			"auth_type": schema.StringAttribute{
				MarkdownDescription: "How the server signs in to the email provider. `BASIC` is the default and uses `username` and `password`. `CLIENT_CREDENTIAL` uses the `clientId`, `clientSecret`, `tokenEndpoint` and `scopes` properties. `BEARER` uses `accessToken`. `API_KEY` is for HTTP providers. `NONE` sends no credentials.",
				Optional:            true,
				Computed:            true,
				Default:             stringDefault("BASIC"),
				Validators:          []validator.String{stringvalidator.OneOf("BASIC", "CLIENT_CREDENTIAL", "PASSWORD_CREDENTIAL", "BEARER", "API_KEY", "NONE")},
			},
			"username": schema.StringAttribute{MarkdownDescription: "Username for `BASIC` authentication.", Optional: true},
			"password": schema.StringAttribute{MarkdownDescription: "Password for `BASIC` authentication. The server never returns this value.", Optional: true, Sensitive: true},
			"properties": schema.MapAttribute{
				MarkdownDescription: "Additional properties, for example `mail.smtp.starttls.enable`, `mail.smtp.auth`, or for HTTP providers `body`, `http.client.method`, `contentType`, `http.headers`. Only the keys listed are managed.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"secret_properties": schema.MapAttribute{
				MarkdownDescription: "Confidential properties such as `clientSecret` or `accessToken`. The server never returns this value.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *emailSenderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func emailBody(ctx context.Context, m *emailSenderModel, includeName bool, diags *diag.Diagnostics) map[string]any {
	props := map[string]string{}
	for k, v := range tfutil.MapOfStrings(ctx, m.Properties, diags) {
		props[k] = v
	}
	for k, v := range tfutil.MapOfStrings(ctx, m.SecretProperties, diags) {
		props[k] = v
	}
	if v := m.DisplayName.ValueString(); v != "" {
		props["mail.smtp.signature"] = v
	}
	if v := m.ReplyTo.ValueString(); v != "" {
		props["mail.smtp.replyTo"] = v
	}
	if v := m.Username.ValueString(); v != "" {
		props["userName"] = v
	}
	if v := m.Password.ValueString(); v != "" {
		props["password"] = v
	}
	list := make([]map[string]string, 0, len(props))
	for _, k := range tfutil.SortedKeys(props) {
		list = append(list, map[string]string{"key": k, "value": props[k]})
	}
	// authType is required by 7.3 and rejected by earlier versions: this resource needs 7.3.
	b := tfutil.JSON{}.Str("provider", m.Provider).Str("providerURL", m.ProviderURL).Str("smtpServerHost", m.SMTPHost).
		Int("smtpPort", m.SMTPPort).Str("fromAddress", m.FromAddress).Str("authType", m.AuthType).Map()
	b["properties"] = list
	if includeName {
		b["name"] = client.EmailPublisher
	}
	return b
}

func senderError(diags *diag.Diagnostics, action string, err error) {
	if client.StatusCode(err) == http.StatusMethodNotAllowed {
		diags.AddError("Unable to "+action, err.Error()+"\n\nThe email provider of the super tenant (carbon.super) can only be configured in deployment.toml ([output_adapter.email]); the API is only available in other tenants.")
		return
	}
	tfutil.AddAPIError(diags, action, err)
}

func (r *emailSenderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan emailSenderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.CreateNotificationSender(ctx, "email", emailBody(ctx, &plan, true, &resp.Diagnostics)); err != nil {
		senderError(&resp.Diagnostics, "create email sender", err)
		return
	}
	plan.ID = types.StringValue(client.EmailPublisher)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailSenderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state emailSenderModel
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

func (r *emailSenderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan emailSenderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateNotificationSender(ctx, "email", client.EmailPublisher, emailBody(ctx, &plan, false, &resp.Diagnostics)); err != nil {
		senderError(&resp.Diagnostics, "update email sender", err)
		return
	}
	plan.ID = types.StringValue(client.EmailPublisher)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailSenderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.client.DeleteNotificationSender(ctx, "email", client.EmailPublisher); err != nil && !client.IsNotFound(err) && !client.HasCode(err, "NSM-60006") {
		senderError(&resp.Diagnostics, "delete email sender", err)
	}
}

func (r *emailSenderResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), client.EmailPublisher)...)
}

func (r *emailSenderResource) read(ctx context.Context, m *emailSenderModel, diags *diag.Diagnostics) bool {
	s, err := r.client.GetNotificationSender(ctx, "email", client.EmailPublisher)
	if err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "NSM-60006") {
			return false
		}
		senderError(diags, "read email sender", err)
		return false
	}
	importing := m.FromAddress.IsNull()
	m.Provider = tfutil.GetString(s, "provider")
	if m.Provider.IsNull() {
		m.Provider = types.StringValue("SMTP")
	}
	m.ProviderURL = keepNull(m.ProviderURL, tfutil.GetString(s, "providerURL"), importing)
	m.SMTPHost = keepNull(m.SMTPHost, tfutil.GetString(s, "smtpServerHost"), importing)
	if port := tfutil.GetInt64(s, "smtpPort"); !port.IsNull() && (!m.SMTPPort.IsNull() || importing) {
		m.SMTPPort = port
	}
	m.FromAddress = tfutil.GetString(s, "fromAddress")
	m.AuthType = tfutil.GetString(s, "authType")
	if m.AuthType.IsNull() {
		m.AuthType = types.StringValue("BASIC")
	}
	props := map[string]string{}
	for _, p := range tfutil.GetObjects(s, "properties") {
		k, _ := p["key"].(string)
		v, _ := p["value"].(string)
		props[k] = v
	}
	m.DisplayName = keepNull(m.DisplayName, tfutil.StringValueOrNull(props["mail.smtp.signature"]), importing)
	m.ReplyTo = keepNull(m.ReplyTo, tfutil.StringValueOrNull(props["mail.smtp.replyTo"]), importing)
	m.Username = keepNull(m.Username, tfutil.StringValueOrNull(props["userName"]), importing)
	managed := m.Properties.Elements()
	out := map[string]string{}
	for k, v := range props {
		switch k {
		case "mail.smtp.signature", "mail.smtp.replyTo", "userName", "password":
			continue
		}
		if _, ok := managed[k]; ok || (importing && m.SecretProperties.Elements()[k] == nil) {
			out[k] = v
		}
	}
	m.Properties = tfutil.MapValueOfStrings(out, m.Properties.IsNull() && (!importing || len(out) == 0))
	return true
}

// keepNull returns the server value, unless the attribute is not managed (null in state
// and not importing).
func keepNull(cur, server types.String, importing bool) types.String {
	if cur.IsNull() && !importing {
		return cur
	}
	return server
}

// ---- wso2is_sms_sender ----

var (
	_ resource.ResourceWithConfigure   = (*smsSenderResource)(nil)
	_ resource.ResourceWithImportState = (*smsSenderResource)(nil)
)

// NewSMSSenderResource returns the wso2is_sms_sender resource.
func NewSMSSenderResource() resource.Resource { return &smsSenderResource{} }

type smsSenderResource struct{ client *client.Client }

type smsSenderModel struct {
	ID                       types.String `tfsdk:"id"`
	Provider                 types.String `tfsdk:"provider_name"`
	ProviderURL              types.String `tfsdk:"provider_url"`
	Key                      types.String `tfsdk:"key"`
	Secret                   types.String `tfsdk:"secret"`
	Sender                   types.String `tfsdk:"sender"`
	ContentType              types.String `tfsdk:"content_type"`
	AuthenticationType       types.String `tfsdk:"authentication_type"`
	AuthenticationProperties types.Map    `tfsdk:"authentication_properties"`
	Properties               types.Map    `tfsdk:"properties"`
}

func (r *smsSenderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sms_sender"
}

func (r *smsSenderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the tenant's SMS provider. In the Console, this is **Notification Channels > SMS Provider**. The server uses it for SMS one-time passwords and SMS notifications. Each tenant has one SMS provider. To import, use the ID `SMSPublisher`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Always `SMSPublisher`."),
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "`Twilio`, `Vonage`, `AWSSNS` or `Custom`.",
				Required:            true,
			},
			"provider_url": schema.StringAttribute{MarkdownDescription: "Provider endpoint (required for `Custom`).", Optional: true},
			"key":          schema.StringAttribute{MarkdownDescription: "Account key or ID (Twilio account SID, Vonage key). With BASIC authentication the server reports the username here.", Optional: true, Computed: true, PlanModifiers: keepString()},
			"secret":       schema.StringAttribute{MarkdownDescription: "Account secret or token.", Optional: true, Computed: true, Sensitive: true, PlanModifiers: keepString()},
			"sender":       schema.StringAttribute{MarkdownDescription: "Sender number or ID.", Optional: true},
			"content_type": schema.StringAttribute{
				MarkdownDescription: "`JSON` or `FORM`, the format of requests to the provider.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("JSON", "FORM")},
			},
			"authentication_type": schema.StringAttribute{
				MarkdownDescription: "How the server signs in to a custom provider's endpoint. The allowed values are `BASIC`, `CLIENT_CREDENTIAL`, `BEARER`, `API_KEY` and `NONE`.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("BASIC", "CLIENT_CREDENTIAL", "BEARER", "API_KEY", "NONE")},
			},
			"authentication_properties": schema.MapAttribute{
				MarkdownDescription: "Authentication properties, for example `username`/`password` (BASIC), `accessToken` (BEARER), `header`/`value` (API_KEY), `clientId`/`clientSecret`/`tokenEndpoint`/`scopes`. The server never returns this value.",
				Optional:            true,
				Sensitive:           true,
				ElementType:         types.StringType,
			},
			"properties": schema.MapAttribute{
				MarkdownDescription: "Additional properties, for example `body` (request template with `{{mobile}}` and `{{body}}` placeholders), `http.client.method`, `http.headers`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *smsSenderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func smsBody(ctx context.Context, m *smsSenderModel, includeName bool, diags *diag.Diagnostics) map[string]any {
	b := tfutil.JSON{}.Str("provider", m.Provider).Str("providerURL", m.ProviderURL).Str("sender", m.Sender).Str("contentType", m.ContentType).Map()
	if tfutil.Known(m.Key) && m.AuthenticationType.IsNull() {
		b["key"] = m.Key.ValueString()
	}
	if tfutil.Known(m.Secret) && m.AuthenticationType.IsNull() {
		b["secret"] = m.Secret.ValueString()
	}
	if tfutil.Known(m.AuthenticationType) {
		auth := map[string]any{"type": m.AuthenticationType.ValueString()}
		if p := tfutil.MapOfStrings(ctx, m.AuthenticationProperties, diags); len(p) > 0 {
			props := map[string]any{}
			for k, v := range p {
				props[k] = v
			}
			auth["properties"] = props
		}
		b["authentication"] = auth
	}
	props := tfutil.MapOfStrings(ctx, m.Properties, diags)
	list := make([]map[string]string, 0, len(props))
	for _, k := range tfutil.SortedKeys(props) {
		list = append(list, map[string]string{"key": k, "value": props[k]})
	}
	b["properties"] = list
	if includeName {
		b["name"] = client.SMSPublisher
	}
	return b
}

func (r *smsSenderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan smsSenderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.CreateNotificationSender(ctx, "sms", smsBody(ctx, &plan, true, &resp.Diagnostics)); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create SMS sender", err)
		return
	}
	plan.ID = types.StringValue(client.SMSPublisher)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *smsSenderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state smsSenderModel
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

func (r *smsSenderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan smsSenderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.UpdateNotificationSender(ctx, "sms", client.SMSPublisher, smsBody(ctx, &plan, false, &resp.Diagnostics)); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update SMS sender", err)
		return
	}
	plan.ID = types.StringValue(client.SMSPublisher)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *smsSenderResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.client.DeleteNotificationSender(ctx, "sms", client.SMSPublisher); err != nil && !client.IsNotFound(err) && !client.HasCode(err, "NSM-60006") {
		tfutil.AddAPIError(&resp.Diagnostics, "delete SMS sender", err)
	}
}

func (r *smsSenderResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), client.SMSPublisher)...)
}

func (r *smsSenderResource) read(ctx context.Context, m *smsSenderModel, diags *diag.Diagnostics) bool {
	s, err := r.client.GetNotificationSender(ctx, "sms", client.SMSPublisher)
	if err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "NSM-60006") {
			return false
		}
		tfutil.AddAPIError(diags, "read SMS sender", err)
		return false
	}
	importing := m.Provider.IsNull()
	m.Provider = tfutil.GetString(s, "provider")
	m.ProviderURL = keepNull(m.ProviderURL, tfutil.GetString(s, "providerURL"), importing)
	m.Key = tfutil.GetString(s, "key")
	if sec := tfutil.GetString(s, "secret"); tfutil.Known(sec) && sec.ValueString() != "" {
		m.Secret = sec
	} else if !tfutil.Known(m.Secret) {
		m.Secret = types.StringNull()
	}
	m.Sender = keepNull(m.Sender, tfutil.GetString(s, "sender"), importing)
	m.ContentType = tfutil.GetString(s, "contentType")
	if auth := tfutil.GetMap(s, "authentication"); auth != nil && (!m.AuthenticationType.IsNull() || importing) {
		m.AuthenticationType = tfutil.GetString(auth, "type")
	}
	managed := m.Properties.Elements()
	out := map[string]string{}
	for _, p := range tfutil.GetObjects(s, "properties") {
		k, _ := p["key"].(string)
		v, _ := p["value"].(string)
		if _, ok := managed[k]; ok || importing {
			out[k] = v
		}
	}
	m.Properties = tfutil.MapValueOfStrings(out, m.Properties.IsNull() && (!importing || len(out) == 0))
	return true
}
