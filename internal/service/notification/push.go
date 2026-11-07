package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure      = (*pushSenderResource)(nil)
	_ resource.ResourceWithImportState    = (*pushSenderResource)(nil)
	_ resource.ResourceWithValidateConfig = (*pushSenderResource)(nil)
)

// NewPushSenderResource returns the wso2is_push_sender resource.
func NewPushSenderResource() resource.Resource { return &pushSenderResource{} }

type pushSenderResource struct{ client *client.Client }

type pushSenderModel struct {
	ID        types.String `tfsdk:"id"`
	Provider  types.String `tfsdk:"provider_name"`
	FCM       types.Object `tfsdk:"fcm"`
	AmazonSNS types.Object `tfsdk:"amazon_sns"`
	Default   types.Bool   `tfsdk:"default"`
}

type fcmModel struct {
	ServiceAccountJSON types.String `tfsdk:"service_account_json"`
}

type snsModel struct {
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	Region          types.String `tfsdk:"region"`
	PlatformARNs    types.Map    `tfsdk:"platform_arns"`
}

var (
	fcmTypes = map[string]attr.Type{"service_account_json": types.StringType}
	snsTypes = map[string]attr.Type{
		"access_key_id": types.StringType, "secret_access_key": types.StringType, "region": types.StringType,
		"platform_arns": types.MapType{ElemType: types.StringType},
	}
)

func (r *pushSenderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_push_sender"
}

func (r *pushSenderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a push notification provider. In the Console, this is **Notification Channels > Push Providers**. The push notification authenticator uses it. WSO2 IS 7.3 supports Firebase Cloud Messaging and Amazon SNS, with one of each per tenant. The server returns the credentials in its API responses, so the credentials appear in the Terraform state. To import, use the ID `FCM_PushPublisher` or `AmazonSNS_PushPublisher`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Sender name: `<provider_name>_PushPublisher`."),
			"provider_name": schema.StringAttribute{
				MarkdownDescription: "`FCM` or `AmazonSNS`. Changing it replaces the sender.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.OneOf("FCM", "AmazonSNS")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"fcm": schema.SingleNestedAttribute{
				MarkdownDescription: "Firebase Cloud Messaging settings (with `provider_name = \"FCM\"`).",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"service_account_json": schema.StringAttribute{
						MarkdownDescription: "Content of the Firebase service account key file (JSON), for example `file(\"service-account.json\")`.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
			"amazon_sns": schema.SingleNestedAttribute{
				MarkdownDescription: "Amazon SNS settings (with `provider_name = \"AmazonSNS\"`).",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"access_key_id":     schema.StringAttribute{MarkdownDescription: "Access key ID of an IAM user allowed to manage SNS platform endpoints and publish.", Required: true},
					"secret_access_key": schema.StringAttribute{MarkdownDescription: "Secret access key.", Required: true, Sensitive: true},
					"region":            schema.StringAttribute{MarkdownDescription: "AWS region, for example `eu-west-1`.", Required: true},
					"platform_arns": schema.MapAttribute{
						MarkdownDescription: "Platform application ARN by platform, for example `{ FCM = \"arn:aws:sns:...:app/GCM/my-app\", APNS = \"...\" }`.",
						Required:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"default": schema.BoolAttribute{
				MarkdownDescription: "Use this provider for devices that do not name one when they register. The default is `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func (r *pushSenderResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m pushSenderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	switch m.Provider.ValueString() {
	case "FCM":
		if m.FCM.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("fcm"), "Missing FCM settings", "Set fcm when provider_name is \"FCM\".")
		}
		if !m.AmazonSNS.IsNull() && !m.AmazonSNS.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("amazon_sns"), "Unexpected Amazon SNS settings", "amazon_sns only applies when provider_name is \"AmazonSNS\".")
		}
	case "AmazonSNS":
		if m.AmazonSNS.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("amazon_sns"), "Missing Amazon SNS settings", "Set amazon_sns when provider_name is \"AmazonSNS\".")
		}
		if !m.FCM.IsNull() && !m.FCM.IsUnknown() {
			resp.Diagnostics.AddAttributeError(path.Root("fcm"), "Unexpected FCM settings", "fcm only applies when provider_name is \"FCM\".")
		}
	}
}

func (r *pushSenderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func pushSenderName(provider string) string { return provider + "_PushPublisher" }

func (r *pushSenderResource) senderPath(provider string) string {
	return r.client.ServerV2Path("/notification-senders/push/" + url.PathEscape(pushSenderName(provider)))
}

func pushSenderBody(ctx context.Context, m *pushSenderModel, diags *diag.Diagnostics) map[string]any {
	provider := m.Provider.ValueString()
	props := []map[string]string{}
	if v, ok := tfutil.ObjectAs[fcmModel](ctx, m.FCM, diags); ok {
		props = append(props, map[string]string{"key": "serviceAccountContent", "value": base64.StdEncoding.EncodeToString([]byte(v.ServiceAccountJSON.ValueString()))})
	}
	if v, ok := tfutil.ObjectAs[snsModel](ctx, m.AmazonSNS, diags); ok {
		arns, _ := json.Marshal(tfutil.MapOfStrings(ctx, v.PlatformARNs, diags))
		props = append(props,
			map[string]string{"key": "accessKeyId", "value": v.AccessKeyID.ValueString()},
			map[string]string{"key": "secretAccessKey", "value": v.SecretAccessKey.ValueString()},
			map[string]string{"key": "region", "value": v.Region.ValueString()},
			map[string]string{"key": "platformArns", "value": string(arns)},
		)
	}
	return map[string]any{"name": provider, "provider": provider, "properties": props}
}

// setDefault makes the provider the default push provider, or clears the default when
// it is this provider and def is false.
func (r *pushSenderResource) setDefault(ctx context.Context, provider string, def bool, diags *diag.Diagnostics) {
	p := r.client.ServerPath("/notification-senders/configs/push")
	var cur struct {
		DefaultPushProvider string `json:"defaultPushProvider"`
	}
	if err := r.client.Get(ctx, p, nil, &cur); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(diags, "read the default push provider", err)
		return
	}
	var want any
	switch {
	case def && cur.DefaultPushProvider != provider:
		want = provider
	case !def && cur.DefaultPushProvider == provider:
		want = nil
	default:
		return
	}
	if _, err := r.client.Put(ctx, p, map[string]any{"defaultPushProvider": want}, nil); err != nil {
		tfutil.AddAPIError(diags, "set the default push provider", err)
	}
}

func (r *pushSenderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pushSenderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := pushSenderBody(ctx, &plan, &resp.Diagnostics)
	if _, err := r.client.Post(ctx, r.client.ServerV2Path("/notification-senders/push"), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create push sender", err)
		return
	}
	plan.ID = types.StringValue(pushSenderName(plan.Provider.ValueString()))
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	r.setDefault(ctx, plan.Provider.ValueString(), plan.Default.ValueBool(), &resp.Diagnostics)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pushSenderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pushSenderModel
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

func (r *pushSenderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan pushSenderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// The server needs every property in an update.
	body := pushSenderBody(ctx, &plan, &resp.Diagnostics)
	delete(body, "name")
	if _, err := r.client.Put(ctx, r.senderPath(plan.Provider.ValueString()), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update push sender", err)
		return
	}
	r.setDefault(ctx, plan.Provider.ValueString(), plan.Default.ValueBool(), &resp.Diagnostics)
	plan.ID = types.StringValue(pushSenderName(plan.Provider.ValueString()))
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pushSenderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pushSenderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.senderPath(state.Provider.ValueString())); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete push sender", err)
		return
	}
	// The default setting outlives the sender; clear it.
	r.setDefault(ctx, state.Provider.ValueString(), false, &resp.Diagnostics)
}

func (r *pushSenderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	provider := req.ID
	if n := len(req.ID) - len("_PushPublisher"); n > 0 && req.ID[n:] == "_PushPublisher" {
		provider = req.ID[:n]
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), pushSenderName(provider))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("provider_name"), provider)...)
}

func (r *pushSenderResource) read(ctx context.Context, m *pushSenderModel, diags *diag.Diagnostics) bool {
	var s struct {
		Provider   string `json:"provider"`
		Properties []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"properties"`
	}
	if err := r.client.Get(ctx, r.senderPath(m.Provider.ValueString()), nil, &s); err != nil {
		if !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "read push sender", err)
		}
		return false
	}
	props := map[string]string{}
	for _, p := range s.Properties {
		props[p.Key] = p.Value
	}
	m.ID = types.StringValue(pushSenderName(s.Provider))
	m.Provider = types.StringValue(s.Provider)
	switch s.Provider {
	case "FCM":
		sa := props["serviceAccountContent"]
		if raw, err := base64.StdEncoding.DecodeString(sa); err == nil {
			sa = string(raw)
		}
		// Keep the configured formatting when the content is the same.
		if prior, ok := tfutil.ObjectAs[fcmModel](ctx, m.FCM, diags); ok && client.JSONEqualString(prior.ServiceAccountJSON.ValueString(), sa) {
			sa = prior.ServiceAccountJSON.ValueString()
		}
		m.FCM = tfutil.ObjectFrom(ctx, fcmTypes, fcmModel{ServiceAccountJSON: types.StringValue(sa)}, diags)
		m.AmazonSNS = types.ObjectNull(snsTypes)
	case "AmazonSNS":
		arns := map[string]string{}
		_ = json.Unmarshal([]byte(props["platformArns"]), &arns)
		m.AmazonSNS = tfutil.ObjectFrom(ctx, snsTypes, snsModel{
			AccessKeyID: types.StringValue(props["accessKeyId"]), SecretAccessKey: types.StringValue(props["secretAccessKey"]),
			Region: types.StringValue(props["region"]), PlatformARNs: tfutil.MapValueOfStrings(arns, false),
		}, diags)
		m.FCM = types.ObjectNull(fcmTypes)
	}
	var cfg struct {
		DefaultPushProvider string `json:"defaultPushProvider"`
	}
	if err := r.client.Get(ctx, r.client.ServerPath("/notification-senders/configs/push"), nil, &cfg); err == nil {
		m.Default = types.BoolValue(cfg.DefaultPushProvider == s.Provider)
	}
	return true
}
