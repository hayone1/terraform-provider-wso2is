package apiresource

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var _ datasource.DataSourceWithConfigValidators = (*apiResourceDataSource)(nil)

// NewDataSource returns the wso2is_api_resource data source.
func NewDataSource() datasource.DataSource { return &apiResourceDataSource{} }

type apiResourceDataSource struct{ client *client.Client }

type dsModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Identifier            types.String `tfsdk:"identifier"`
	Description           types.String `tfsdk:"description"`
	Type                  types.String `tfsdk:"type"`
	RequiresAuthorization types.Bool   `tfsdk:"requires_authorization"`
	Scopes                types.Set    `tfsdk:"scopes"`
	ScopeNames            types.Set    `tfsdk:"scope_names"`
	ScopeIDs              types.Map    `tfsdk:"scope_ids"`
}

func (d *apiResourceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_resource"
}

func (d *apiResourceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up an API resource by ID, identifier or name. This includes the built-in management APIs, for example the APIs with the identifiers `/api/server/v1/applications` or `/scim2/Users`.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{MarkdownDescription: "API resource ID.", Optional: true, Computed: true},
			"identifier":             schema.StringAttribute{MarkdownDescription: "Identifier, for example `https://api.example.com` or `/api/server/v1/applications`.", Optional: true, Computed: true},
			"name":                   schema.StringAttribute{MarkdownDescription: "Display name.", Optional: true, Computed: true},
			"description":            schema.StringAttribute{MarkdownDescription: "Description.", Computed: true},
			"type":                   schema.StringAttribute{MarkdownDescription: "Type: `BUSINESS`, `TENANT`, `ORGANIZATION`, `SYSTEM`, `CONSOLE_FEATURE`, ...", Computed: true},
			"requires_authorization": schema.BoolAttribute{MarkdownDescription: "Whether applications need authorization to request its scopes.", Computed: true},
			"scopes": schema.SetNestedAttribute{
				MarkdownDescription: "Scopes.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Scope name."},
					"display_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
					"description":  schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
				}},
			},
			"scope_names": schema.SetAttribute{MarkdownDescription: "All scope names, handy for authorizing every scope.", Computed: true, ElementType: types.StringType},
			"scope_ids":   schema.MapAttribute{MarkdownDescription: "Scope IDs by name.", Computed: true, ElementType: types.StringType},
		},
	}
}

func (d *apiResourceDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("id"), path.MatchRoot("identifier"), path.MatchRoot("name")),
	}
}

func (d *apiResourceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = tfutil.ConfigureDataSource(req, resp)
}

func (d *apiResourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m dsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var (
		a   *client.APIResource
		err error
	)
	switch {
	case tfutil.Known(m.ID):
		a, err = d.client.GetAPIResource(ctx, m.ID.ValueString())
	case tfutil.Known(m.Identifier):
		a, err = d.client.FindAPIResourceByIdentifier(ctx, m.Identifier.ValueString())
	default:
		var items []client.APIResourceListItem
		items, err = client.FindByEq(ctx, d.client.ListAPIResources, "name", m.Name.ValueString())
		if err == nil {
			var matches []client.APIResourceListItem
			for _, it := range items {
				if it.Name == m.Name.ValueString() {
					matches = append(matches, it)
				}
			}
			switch len(matches) {
			case 0:
				resp.Diagnostics.AddError("API resource not found", fmt.Sprintf("no API resource named %q", m.Name.ValueString()))
				return
			case 1:
				a, err = d.client.GetAPIResource(ctx, matches[0].ID)
			default:
				resp.Diagnostics.AddError("Multiple API resources found", fmt.Sprintf("%d API resources are named %q; look it up by identifier instead", len(matches), m.Name.ValueString()))
				return
			}
		}
	}
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read API resource", err)
		return
	}
	m.ID = types.StringValue(a.ID)
	m.Name = types.StringValue(a.Name)
	m.Identifier = types.StringValue(a.Identifier)
	m.Description = types.StringValue(a.Description)
	m.Type = types.StringValue(a.Type)
	m.RequiresAuthorization = tfutil.BoolValueOr(a.RequiresAuthorization, true)
	m.Scopes = flattenScopes(ctx, a.Scopes, &resp.Diagnostics)
	names := make([]string, 0, len(a.Scopes))
	ids := map[string]string{}
	for _, s := range a.Scopes {
		names = append(names, s.Name)
		ids[s.Name] = s.ID
	}
	sort.Strings(names)
	m.ScopeNames = tfutil.SetOfStrings(names, false)
	m.ScopeIDs = tfutil.MapValueOfStrings(ids, false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
