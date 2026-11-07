// Package tenant implements wso2is_tenant.
package tenant

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*tenantResource)(nil)
	_ resource.ResourceWithImportState = (*tenantResource)(nil)
)

// NewResource returns the wso2is_tenant resource.
func NewResource() resource.Resource { return &tenantResource{} }

type tenantResource struct{ client *client.Client }

type model struct {
	ID             types.String `tfsdk:"id"`
	Domain         types.String `tfsdk:"domain"`
	Name           types.String `tfsdk:"name"`
	Activated      types.Bool   `tfsdk:"activated"`
	OwnerID        types.String `tfsdk:"owner_id"`
	OwnerUsername  types.String `tfsdk:"owner_username"`
	OwnerPassword  types.String `tfsdk:"owner_password"`
	OwnerEmail     types.String `tfsdk:"owner_email"`
	OwnerFirstName types.String `tfsdk:"owner_first_name"`
	OwnerLastName  types.String `tfsdk:"owner_last_name"`
	CreatedDate    types.String `tfsdk:"created_date"`
}

func (r *tenantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (r *tenantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a tenant (a root organization) and its owner.\n\nThe provider that creates the tenant must manage the super tenant (`tenant_domain = \"carbon.super\"`). It must use basic authentication, with a user who may manage tenants. With an application's client credentials, the server fails to create the tenant (`TM-65002`, \"Error in adding claims to the user\").\n\nTo manage objects inside the tenant, add another provider block with an `alias`. Set `tenant_domain = wso2is_tenant.<name>.domain`, and sign in as the owner (`username = \"<owner>@<domain>\"`). This works in the same configuration.\n\nWhen you destroy the resource, the provider deletes the tenant if the server allows it (`[tenant_mgt] tenant_deletion = true` in `deployment.toml`). Otherwise, the provider deactivates the tenant and shows a warning. To import, use the tenant ID or domain.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Tenant ID."),
			"domain": schema.StringAttribute{
				MarkdownDescription: "Tenant domain, for example `acme.com`. Changing it replaces the tenant.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name. The default is the domain. The server only accepts this value at creation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString(), stringplanmodifier.RequiresReplaceIfConfigured()},
			},
			"activated": tfutil.Bool("Whether the tenant is active. Deactivated tenants cannot be used to sign in.", true),
			"owner_id":  tfutil.ComputedString("User ID of the tenant owner."),
			"owner_username": schema.StringAttribute{
				MarkdownDescription: "Username of the tenant owner (without the domain). Changing it replaces the tenant.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"owner_password": schema.StringAttribute{
				MarkdownDescription: "The tenant owner's password. The server never returns it, so the provider cannot notice changes made outside Terraform.",
				Required:            true,
				Sensitive:           true,
			},
			"owner_email":      schema.StringAttribute{MarkdownDescription: "Email of the tenant owner.", Required: true},
			"owner_first_name": tfutil.String("First name of the tenant owner.", ""),
			"owner_last_name":  tfutil.String("Last name of the tenant owner.", ""),
			"created_date":     tfutil.ComputedString("Creation time (ISO-8601)."),
		},
	}
}

func (r *tenantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *tenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	t := &client.Tenant{
		Domain: plan.Domain.ValueString(),
		Name:   plan.Name.ValueString(),
		Owners: []client.TenantOwner{{
			Username:           plan.OwnerUsername.ValueString(),
			Password:           plan.OwnerPassword.ValueString(),
			Email:              plan.OwnerEmail.ValueString(),
			FirstName:          plan.OwnerFirstName.ValueString(),
			LastName:           plan.OwnerLastName.ValueString(),
			ProvisioningMethod: "inline-password",
		}},
	}
	id, err := r.client.CreateTenant(ctx, t)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create tenant", err)
		if client.HasCode(err, "TM-65002") {
			resp.Diagnostics.AddWarning("Tenants need basic authentication",
				"If the provider authenticates with an application's client credentials, the server looks for that application in the new tenant while creating it, "+
					"and fails. Create tenants with a provider that uses username and password.")
			// The failure leaves a deactivated tenant behind that blocks the domain
			// (TM-60009 on the next attempt). The domain was free, so it is ours to remove.
			if left, gerr := r.client.GetTenantByDomain(ctx, plan.Domain.ValueString()); gerr == nil {
				_ = r.client.SetTenantActivated(ctx, left.ID, false)
				if derr := r.client.DeleteTenant(ctx, left.ID); derr != nil && !client.IsNotFound(derr) {
					resp.Diagnostics.AddWarning("Partially created tenant left behind",
						fmt.Sprintf("The failed request left the deactivated tenant %s (ID %s), which blocks the domain. Delete it before retrying: %s", plan.Domain.ValueString(), left.ID, derr))
				}
			}
		}
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	plan.ID = types.StringValue(id)
	r.waitUsable(ctx, &plan, &resp.Diagnostics)
	if !plan.Activated.ValueBool() {
		if err := r.client.SetTenantActivated(ctx, id, false); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "deactivate tenant", err)
			return
		}
	}
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Tenant not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state model
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

func (r *tenantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !plan.OwnerEmail.Equal(state.OwnerEmail) || !plan.OwnerFirstName.Equal(state.OwnerFirstName) || !plan.OwnerLastName.Equal(state.OwnerLastName) || !plan.OwnerPassword.Equal(state.OwnerPassword) {
		body := map[string]any{"email": plan.OwnerEmail.ValueString(), "firstname": plan.OwnerFirstName.ValueString(), "lastname": plan.OwnerLastName.ValueString()}
		if !plan.OwnerPassword.Equal(state.OwnerPassword) {
			body["password"] = plan.OwnerPassword.ValueString()
		}
		if err := r.client.UpdateTenantOwner(ctx, id, state.OwnerID.ValueString(), body); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update tenant owner", err)
			return
		}
	}
	if !plan.Activated.Equal(state.Activated) {
		if err := r.client.SetTenantActivated(ctx, id, plan.Activated.ValueBool()); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "change tenant status", err)
			return
		}
	}
	plan.ID, plan.OwnerID = state.ID, state.OwnerID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Tenant not found after update", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	// The server only deletes deactivated tenants.
	if err := r.client.SetTenantActivated(ctx, id, false); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "deactivate tenant", err)
		return
	}
	err := r.client.DeleteTenant(ctx, id)
	switch {
	case err == nil, client.IsNotFound(err):
	case client.HasCode(err, "TM-60016"):
		resp.Diagnostics.AddWarning("Tenant deactivated, not deleted",
			fmt.Sprintf("The server does not allow tenant deletion, so tenant %s was deactivated and removed from the Terraform state. Set `[tenant_mgt] tenant_deletion = true` in deployment.toml to delete tenants.", state.Domain.ValueString()))
	default:
		tfutil.AddAPIError(&resp.Diagnostics, "delete tenant", err)
	}
}

// tenantReadyTimeout bounds the wait for a new tenant to accept API calls; a fresh
// domain is usable within seconds.
const tenantReadyTimeout = 2 * time.Minute

// waitUsable waits until the owner can call the tenant's APIs, so resources inside the
// tenant can be created right after it.
func (r *tenantResource) waitUsable(ctx context.Context, m *model, diags *diag.Diagnostics) {
	tc := r.client.WithBasicAuth(m.Domain.ValueString(), m.OwnerUsername.ValueString()+"@"+m.Domain.ValueString(), m.OwnerPassword.ValueString())
	err := tfutil.WaitFor(ctx, tenantReadyTimeout, func() (bool, error) {
		var out any
		return tc.Get(ctx, tc.ServerPath("/applications"), url.Values{"limit": {"1"}}, &out) == nil, nil
	})
	if err != nil {
		diags.AddWarning("Tenant not usable yet",
			fmt.Sprintf("Tenant %s was created, but its owner could not call its APIs within %s, so resources inside it may fail for now. "+
				"This happens when the domain belonged to a tenant deleted shortly before: the server keeps caches of the deleted tenant "+
				"(\"Error while retrieving role id list of user\") until they expire, about 15 minutes later, or the server restarts.", m.Domain.ValueString(), tenantReadyTimeout))
	}
}

func (r *tenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := req.ID
	if t, err := r.client.GetTenantByDomain(ctx, req.ID); err == nil {
		id = t.ID
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

func (r *tenantResource) read(ctx context.Context, m *model, diags *diag.Diagnostics) bool {
	t, err := r.client.GetTenant(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read tenant", err)
		return false
	}
	m.Domain = types.StringValue(t.Domain)
	m.Name = types.StringValue(t.Name)
	m.CreatedDate = types.StringValue(t.CreatedDate)
	m.Activated = types.BoolValue(t.LifecycleStatus == nil || t.LifecycleStatus.Activated)
	if len(t.Owners) > 0 {
		m.OwnerID = types.StringValue(t.Owners[0].ID)
		m.OwnerUsername = types.StringValue(t.Owners[0].Username)
		if o, err := r.client.GetTenantOwner(ctx, m.ID.ValueString(), t.Owners[0].ID); err == nil {
			m.OwnerEmail = tfutil.GetString(o, "email")
			m.OwnerFirstName = tfutil.GetStringOrEmpty(o, "firstname")
			m.OwnerLastName = tfutil.GetStringOrEmpty(o, "lastname")
		}
	}
	return true
}
