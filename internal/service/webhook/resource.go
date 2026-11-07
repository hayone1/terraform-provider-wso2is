// Package webhook implements wso2is_webhook (event webhooks, WSO2 IS 7.2+).
package webhook

import (
	"context"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*webhookResource)(nil)
	_ resource.ResourceWithImportState = (*webhookResource)(nil)
)

// NewResource returns the wso2is_webhook resource.
func NewResource() resource.Resource { return &webhookResource{} }

type webhookResource struct{ client *client.Client }

type model struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Endpoint        types.String `tfsdk:"endpoint"`
	Secret          types.String `tfsdk:"secret"`
	EventProfile    types.String `tfsdk:"event_profile"`
	EventProfileURI types.String `tfsdk:"event_profile_uri"`
	Channels        types.Set    `tfsdk:"channels"`
	Active          types.Bool   `tfsdk:"active"`
	Status          types.String `tfsdk:"status"`
}

func (r *webhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *webhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a webhook (WSO2 IS 7.2 and later). The server sends identity events to the webhook as signed HTTP POST requests. Events include logins, sessions, changes to user accounts, registrations, tokens and credential updates. To import, use the webhook ID.",
		Attributes: map[string]schema.Attribute{
			"id":       tfutil.IDAttribute("Webhook ID."),
			"name":     schema.StringAttribute{MarkdownDescription: "Webhook name.", Required: true},
			"endpoint": schema.StringAttribute{MarkdownDescription: "HTTPS URL events are posted to.", Required: true},
			"secret": schema.StringAttribute{
				MarkdownDescription: "The secret that the server uses to sign each payload (`X-WSO2-Event-Signature`). The server never returns it, so the provider cannot notice changes made outside Terraform.",
				Optional:            true,
				Sensitive:           true,
			},
			"event_profile":     schema.StringAttribute{MarkdownDescription: "Event profile name. The default is `WSO2`.", Optional: true, Computed: true, Default: stringdefault.StaticString("WSO2")},
			"event_profile_uri": schema.StringAttribute{MarkdownDescription: "Event profile URI. The default is `https://schemas.identity.wso2.org/events`.", Optional: true, Computed: true, Default: stringdefault.StaticString("https://schemas.identity.wso2.org/events")},
			"channels": schema.SetAttribute{
				MarkdownDescription: "Channel URIs to subscribe to, for example `https://schemas.identity.wso2.org/events/login`, `.../session`, `.../user`, `.../registration`, `.../token`, `.../credential`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"active": schema.BoolAttribute{MarkdownDescription: "Whether events are sent. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
			"status": schema.StringAttribute{
				MarkdownDescription: "The webhook's status on the server: `ACTIVE`, `PARTIALLY_ACTIVE`, `INACTIVE` or `PARTIALLY_INACTIVE`. A partial status means that some channel subscriptions failed.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.StringStateUnlessChanged(path.MatchRoot("active"), path.MatchRoot("channels"), path.MatchRoot("endpoint"))},
			},
		},
	}
}

func (r *webhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func body(ctx context.Context, m *model, diags *diag.Diagnostics) map[string]any {
	status := "INACTIVE"
	if m.Active.ValueBool() {
		status = "ACTIVE"
	}
	b := map[string]any{
		"name":               m.Name.ValueString(),
		"endpoint":           m.Endpoint.ValueString(),
		"eventProfile":       map[string]string{"name": m.EventProfile.ValueString(), "uri": m.EventProfileURI.ValueString()},
		"channelsSubscribed": tfutil.Strings(ctx, m.Channels, diags),
		"status":             status,
	}
	if v := m.Secret.ValueString(); v != "" {
		b["secret"] = v
	}
	return b
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if _, err := r.client.Post(ctx, r.client.ServerPath("/webhooks"), body(ctx, &plan, &resp.Diagnostics), &out); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create webhook", err)
		return
	}
	id, _ := out["id"].(string)
	plan.ID = types.StringValue(id)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	p := r.client.ServerPath("/webhooks/" + url.PathEscape(state.ID.ValueString()))
	if _, err := r.client.Put(ctx, p, body(ctx, &plan, &resp.Diagnostics), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update webhook", err)
		return
	}
	if !plan.Active.Equal(state.Active) {
		op := "/deactivate"
		if plan.Active.ValueBool() {
			op = "/activate"
		}
		if _, err := r.client.Post(ctx, p+op, nil, nil); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "change webhook status", err)
			return
		}
	}
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	p := r.client.ServerPath("/webhooks/" + url.PathEscape(state.ID.ValueString()))
	// Active webhooks must be deactivated (unsubscribed) before deletion.
	if state.Active.ValueBool() {
		if _, err := r.client.Post(ctx, p+"/deactivate", nil, nil); err != nil && !client.IsNotFound(err) {
			tfutil.AddAPIError(&resp.Diagnostics, "deactivate webhook", err)
			return
		}
	}
	if err := r.client.Delete(ctx, p); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete webhook", err)
	}
}

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *webhookResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	var w map[string]any
	if err := r.client.Get(ctx, r.client.ServerPath("/webhooks/"+url.PathEscape(m.ID.ValueString())), nil, &w); err != nil {
		if client.IsNotFound(err) || client.HasCode(err, "WEBHOOKMGT-60003", "WEBHOOKMGT-60004") {
			return false
		}
		tfutil.AddAPIError(diags, "read webhook", err)
		return false
	}
	m.Name = tfutil.GetString(w, "name")
	m.Endpoint = tfutil.GetString(w, "endpoint")
	ep := tfutil.GetMap(w, "eventProfile")
	m.EventProfile = tfutil.GetString(ep, "name")
	m.EventProfileURI = tfutil.GetString(ep, "uri")
	var channels []string
	for _, c := range tfutil.GetObjects(w, "channelsSubscribed") {
		if uri, _ := c["channelUri"].(string); uri != "" {
			channels = append(channels, uri)
		}
	}
	sort.Strings(channels)
	m.Channels = tfutil.SetOfStrings(channels, false)
	m.Status = tfutil.GetString(w, "status")
	status := m.Status.ValueString()
	m.Active = types.BoolValue(status == "ACTIVE" || status == "PARTIALLY_ACTIVE")
	return true
}
