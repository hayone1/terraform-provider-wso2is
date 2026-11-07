package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	_ resource.ResourceWithConfigure   = (*authorizedAPIResource)(nil)
	_ resource.ResourceWithImportState = (*authorizedAPIResource)(nil)
)

// NewAuthorizedAPIResource returns the wso2is_application_authorized_api resource.
func NewAuthorizedAPIResource() resource.Resource { return &authorizedAPIResource{} }

type authorizedAPIResource struct{ client *client.Client }

type authorizedAPIModel struct {
	ID                        types.String `tfsdk:"id"`
	ApplicationID             types.String `tfsdk:"application_id"`
	APIResourceID             types.String `tfsdk:"api_resource_id"`
	PolicyIdentifier          types.String `tfsdk:"policy_identifier"`
	Scopes                    types.Set    `tfsdk:"scopes"`
	AuthorizationDetailsTypes types.Set    `tfsdk:"authorization_details_types"`
	Identifier                types.String `tfsdk:"identifier"`
}

func (r *authorizedAPIResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_authorized_api"
}

func (r *authorizedAPIResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Allows an application to request the scopes of an API resource. In the Console, this is the application's **API Authorization** tab. Use it for your own APIs (`wso2is_api_resource`) and for the server's management APIs. To find a management API, use the `wso2is_api_resource` data source. For example, you can allow an M2M application to call the user management API. To import, use the ID `<application_id>/<api_resource_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("`<application_id>/<api_resource_id>`."),
			"application_id": schema.StringAttribute{
				MarkdownDescription: "Application ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"api_resource_id": schema.StringAttribute{
				MarkdownDescription: "API resource ID.",
				Required:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"policy_identifier": schema.StringAttribute{
				MarkdownDescription: "The authorization policy. With `RBAC`, users get the scopes that their roles grant. With `No Policy`, every signed-in user gets the authorized scopes. The default is `RBAC`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("RBAC"),
				Validators:          []validator.String{stringvalidator.OneOf("RBAC", "No Policy")},
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scopes": schema.SetAttribute{
				MarkdownDescription: "Scope names the application may request.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"authorization_details_types": schema.SetAttribute{
				MarkdownDescription: "Authorization details types the application may request. Changing them replaces the authorization.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(types.SetValueMust(types.StringType, nil)),
				PlanModifiers:       []planmodifier.Set{setplanmodifier.RequiresReplace()},
			},
			"identifier": tfutil.ComputedString("Identifier of the API resource."),
		},
	}
}

func (r *authorizedAPIResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func (r *authorizedAPIResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan authorizedAPIModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	appID, apiID := plan.ApplicationID.ValueString(), plan.APIResourceID.ValueString()
	err := r.client.AuthorizeAPI(ctx, appID, apiID, plan.PolicyIdentifier.ValueString(),
		tfutil.Strings(ctx, plan.Scopes, &resp.Diagnostics), tfutil.Strings(ctx, plan.AuthorizationDetailsTypes, &resp.Diagnostics))
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "authorize API for application", err)
		return
	}
	plan.ID = types.StringValue(appID + "/" + apiID)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("API authorization not found after creation", plan.ID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *authorizedAPIResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state authorizedAPIModel
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

func (r *authorizedAPIResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state authorizedAPIModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	want := tfutil.Strings(ctx, plan.Scopes, &resp.Diagnostics)
	have := tfutil.Strings(ctx, state.Scopes, &resp.Diagnostics)
	added, removed := diffStrings(want, have)
	if len(added) > 0 || len(removed) > 0 {
		if err := r.client.PatchAuthorizedAPI(ctx, state.ApplicationID.ValueString(), state.APIResourceID.ValueString(), added, removed); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update authorized scopes", err)
			return
		}
	}
	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("API authorization not found after update", plan.ID.ValueString())
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *authorizedAPIResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state authorizedAPIModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.RevokeAPI(ctx, state.ApplicationID.ValueString(), state.APIResourceID.ValueString())
	if err == nil || r.client.ApplicationGone(ctx, state.ApplicationID.ValueString(), err) {
		return
	}
	// The server answers 400 when the application is gone and 500 when the API resource
	// is gone. In both cases the authorization no longer exists, which is the goal.
	var check diag.Diagnostics
	if !r.read(ctx, &state, &check) && !check.HasError() {
		return
	}
	tfutil.AddAPIError(&resp.Diagnostics, "remove API authorization", err)
}

func (r *authorizedAPIResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	appID, apiID, ok := strings.Cut(req.ID, "/")
	if !ok || appID == "" || apiID == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected <application_id>/<api_resource_id>, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), appID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_resource_id"), apiID)...)
}

func (r *authorizedAPIResource) read(ctx context.Context, m *authorizedAPIModel, diags *diag.Diagnostics) bool {
	apis, err := r.client.ListAuthorizedAPIs(ctx, m.ApplicationID.ValueString())
	if err != nil {
		if r.client.ApplicationGone(ctx, m.ApplicationID.ValueString(), err) {
			return false
		}
		tfutil.AddAPIError(diags, "read authorized APIs", err)
		return false
	}
	for _, a := range apis {
		if a.ID != m.APIResourceID.ValueString() {
			continue
		}
		scopes := make([]string, 0, len(a.AuthorizedScopes))
		for _, s := range a.AuthorizedScopes {
			scopes = append(scopes, s.Name)
		}
		sort.Strings(scopes)
		adts := make([]string, 0, len(a.AuthorizedAuthorizationDetailsTypes))
		for _, t := range a.AuthorizedAuthorizationDetailsTypes {
			adts = append(adts, t.Type)
		}
		m.Scopes = tfutil.SetOfStrings(scopes, false)
		m.AuthorizationDetailsTypes = tfutil.SetOfStrings(adts, false)
		m.PolicyIdentifier = types.StringValue(a.PolicyID)
		m.Identifier = types.StringValue(a.Identifier)
		return true
	}
	return false
}

func diffStrings(want, have []string) (added, removed []string) {
	hs := map[string]bool{}
	for _, h := range have {
		hs[h] = true
	}
	ws := map[string]bool{}
	for _, w := range want {
		ws[w] = true
		if !hs[w] {
			added = append(added, w)
		}
	}
	for _, h := range have {
		if !ws[h] {
			removed = append(removed, h)
		}
	}
	return added, removed
}
