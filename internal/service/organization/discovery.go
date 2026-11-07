package organization

import (
	"context"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

// ---- wso2is_organization_discovery_configuration ----

const discoveryConfigID = "organization_discovery_configuration"

var (
	_ resource.ResourceWithConfigure   = (*discoveryConfigResource)(nil)
	_ resource.ResourceWithImportState = (*discoveryConfigResource)(nil)
)

// NewDiscoveryConfigResource returns the wso2is_organization_discovery_configuration resource.
func NewDiscoveryConfigResource() resource.Resource { return &discoveryConfigResource{} }

type discoveryConfigResource struct{ client *client.Client }

type discoveryConfigModel struct {
	ID                    types.String `tfsdk:"id"`
	EmailDomainEnabled    types.Bool   `tfsdk:"email_domain_enabled"`
	EmailDomainSelfSignUp types.Bool   `tfsdk:"email_domain_based_self_sign_up_enabled"`
	DefaultParam          types.String `tfsdk:"default_param"`
}

func (r *discoveryConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_discovery_configuration"
}

func (r *discoveryConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Turns on organization discovery for the root organization. With discovery on, the login page sends users to their organization's login, based on the domain of their email address. Set each organization's domains with `wso2is_organization_discovery`. Each tenant has one configuration. When you destroy the resource, the provider removes the configuration, which turns discovery off. To import, use the ID `organization_discovery_configuration`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Always `" + discoveryConfigID + "`."),
			"email_domain_enabled": schema.BoolAttribute{
				MarkdownDescription: "Discover the organization from the user's email domain.",
				Required:            true,
			},
			"email_domain_based_self_sign_up_enabled": schema.BoolAttribute{
				MarkdownDescription: "Let users self-register in the organization that owns their email domain. The default is `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"default_param": schema.StringAttribute{
				MarkdownDescription: "What users type when they choose their organization on the login page, either `orgHandle` or `orgName`. When it is not set, the server uses its default.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf("orgHandle", "orgName")},
			},
		},
	}
}

func (r *discoveryConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *discoveryConfigResource) path() string {
	return r.client.ServerPath("/organization-configs/discovery")
}

func discoveryConfigBody(m *discoveryConfigModel) map[string]any {
	props := []map[string]string{
		{"key": "emailDomain.enable", "value": strconv.FormatBool(m.EmailDomainEnabled.ValueBool())},
		{"key": "emailDomainBasedSelfSignup.enable", "value": strconv.FormatBool(m.EmailDomainSelfSignUp.ValueBool())},
	}
	if tfutil.Known(m.DefaultParam) {
		props = append(props, map[string]string{"key": "defaultParam", "value": m.DefaultParam.ValueString()})
	}
	return map[string]any{"properties": props}
}

func (r *discoveryConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan discoveryConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// POST only works while no configuration exists; take over an existing one with PUT.
	var cur map[string]any
	err := r.client.Get(ctx, r.path(), nil, &cur)
	switch {
	case client.IsNotFound(err):
		_, err = r.client.Post(ctx, r.path(), discoveryConfigBody(&plan), nil)
	case err == nil:
		_, err = r.client.Put(ctx, r.path(), discoveryConfigBody(&plan), nil)
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "configure organization discovery", err)
		return
	}
	plan.ID = types.StringValue(discoveryConfigID)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *discoveryConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state discoveryConfigModel
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

func (r *discoveryConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan discoveryConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.Put(ctx, r.path(), discoveryConfigBody(&plan), nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update organization discovery", err)
		return
	}
	plan.ID = types.StringValue(discoveryConfigID)
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *discoveryConfigResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.client.Delete(ctx, r.path()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "remove organization discovery configuration", err)
	}
}

func (r *discoveryConfigResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), discoveryConfigID)...)
}

func (r *discoveryConfigResource) read(ctx context.Context, m *discoveryConfigModel, diags *diag.Diagnostics) bool {
	var out struct {
		Properties []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"properties"`
	}
	if err := r.client.Get(ctx, r.path(), nil, &out); err != nil {
		// OCM-60002: no configuration.
		if !client.IsNotFound(err) {
			tfutil.AddAPIError(diags, "read organization discovery configuration", err)
		}
		return false
	}
	m.ID = types.StringValue(discoveryConfigID)
	m.EmailDomainEnabled, m.EmailDomainSelfSignUp, m.DefaultParam = types.BoolValue(false), types.BoolValue(false), types.StringNull()
	for _, p := range out.Properties {
		switch p.Key {
		case "emailDomain.enable":
			m.EmailDomainEnabled = types.BoolValue(p.Value == "true")
		case "emailDomainBasedSelfSignup.enable":
			m.EmailDomainSelfSignUp = types.BoolValue(p.Value == "true")
		case "defaultParam":
			m.DefaultParam = types.StringValue(p.Value)
		}
	}
	return true
}

// ---- wso2is_organization_discovery ----

var (
	_ resource.ResourceWithConfigure   = (*discoveryResource)(nil)
	_ resource.ResourceWithImportState = (*discoveryResource)(nil)
)

// NewDiscoveryResource returns the wso2is_organization_discovery resource.
func NewDiscoveryResource() resource.Resource { return &discoveryResource{} }

type discoveryResource struct{ client *client.Client }

type discoveryModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	EmailDomains   types.Set    `tfsdk:"email_domains"`
}

func (r *discoveryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_discovery"
}

func (r *discoveryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets the email domains of a sub-organization's users, for email domain discovery. The login page then sends users with these domains to the organization. Turn on email domain discovery first with `wso2is_organization_discovery_configuration`, or the server answers `ORG-60080`. Add `depends_on` for that resource, so that Terraform also removes the domains before it turns discovery off. Manage this resource from the root organization. To import, use the organization ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Same as `organization_id`."),
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "Sub-organization ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"email_domains": schema.SetAttribute{
				MarkdownDescription: "Email domains of the organization's users, for example `partner.example`. A domain can belong to one organization only.",
				Required:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
			},
		},
	}
}

func (r *discoveryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *discoveryResource) orgPath(id string) string {
	return r.client.ServerPath("/organizations/" + url.PathEscape(id) + "/discovery")
}

func emailDomainAttributes(ctx context.Context, m *discoveryModel, diags *diag.Diagnostics) []map[string]any {
	return []map[string]any{{"type": "emailDomain", "values": tfutil.Strings(ctx, m.EmailDomains, diags)}}
}

func (r *discoveryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan discoveryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"organizationId": plan.OrganizationID.ValueString(), "attributes": emailDomainAttributes(ctx, &plan, &resp.Diagnostics)}
	if _, err := r.client.Post(ctx, r.client.ServerPath("/organizations/discovery"), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "add organization discovery attributes", err)
		if client.HasCode(err, "ORG-60080") {
			resp.Diagnostics.AddAttributeError(path.Root("organization_id"), "Organization discovery is disabled",
				"Enable email domain discovery with wso2is_organization_discovery_configuration (email_domain_enabled = true), and add it to this resource's depends_on.")
		}
		return
	}
	plan.ID = plan.OrganizationID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Discovery attributes not found after creation", plan.OrganizationID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *discoveryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state discoveryModel
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

func (r *discoveryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan discoveryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"attributes": emailDomainAttributes(ctx, &plan, &resp.Diagnostics)}
	if _, err := r.client.Put(ctx, r.orgPath(plan.OrganizationID.ValueString()), body, nil); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update organization discovery attributes", err)
		return
	}
	plan.ID = plan.OrganizationID
	r.read(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *discoveryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state discoveryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Delete(ctx, r.orgPath(state.OrganizationID.ValueString())); err != nil && !client.IsNotFound(err) && !client.HasCode(err, "ORG-60064", "ORG-60015") {
		tfutil.AddAPIError(&resp.Diagnostics, "delete organization discovery attributes", err)
	}
}

func (r *discoveryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_id"), req.ID)...)
}

func (r *discoveryResource) read(ctx context.Context, m *discoveryModel, diags *diag.Diagnostics) bool {
	var out struct {
		Attributes []struct {
			Type   string   `json:"type"`
			Values []string `json:"values"`
		} `json:"attributes"`
	}
	if err := r.client.Get(ctx, r.orgPath(m.OrganizationID.ValueString()), nil, &out); err != nil {
		if !client.IsNotFound(err) && !client.HasCode(err, "ORG-60064", "ORG-60015", "ORG-60006") {
			tfutil.AddAPIError(diags, "read organization discovery attributes", err)
		}
		return false
	}
	var domains []string
	for _, a := range out.Attributes {
		if a.Type == "emailDomain" {
			domains = append(domains, a.Values...)
		}
	}
	if len(domains) == 0 {
		return false
	}
	m.ID = m.OrganizationID
	m.EmailDomains = tfutil.SetOfStrings(domains, false)
	return true
}
