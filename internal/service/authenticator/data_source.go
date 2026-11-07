package authenticator

import (
	"context"
	"regexp"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

func mustRegexp(s string) *regexp.Regexp { return regexp.MustCompile(s) }

var _ datasource.DataSourceWithConfigure = (*listDataSource)(nil)

// NewListDataSource returns the wso2is_authenticators data source.
func NewListDataSource() datasource.DataSource { return &listDataSource{} }

type listDataSource struct{ client *client.Client }

type listModel struct {
	Type           types.String `tfsdk:"type"`
	Authenticators types.List   `tfsdk:"authenticators"`
}

type authModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Type        types.String `tfsdk:"type"`
	DefinedBy   types.String `tfsdk:"defined_by"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Tags        types.Set    `tfsdk:"tags"`
}

var authTypes = map[string]attr.Type{
	"id": types.StringType, "name": types.StringType, "display_name": types.StringType, "type": types.StringType,
	"defined_by": types.StringType, "enabled": types.BoolType, "tags": types.SetType{ElemType: types.StringType},
}

func (d *listDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_authenticators"
}

func (d *listDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the authenticators that login flows can use. The list includes local authenticators (use `idp = \"LOCAL\"` in `authentication_sequence`) and the federated authenticators of identity providers.",
		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by type: `LOCAL`, `REQUEST_PATH` or `FEDERATED`."},
			"authenticators": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Authenticators, sorted by name.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":           schema.StringAttribute{Computed: true, MarkdownDescription: "ID."},
					"name":         schema.StringAttribute{Computed: true, MarkdownDescription: "Name used in `authentication_sequence` (for federated ones, the identity provider name)."},
					"display_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
					"type":         schema.StringAttribute{Computed: true, MarkdownDescription: "`LOCAL`, `REQUEST_PATH` or `FEDERATED`."},
					"defined_by":   schema.StringAttribute{Computed: true, MarkdownDescription: "`SYSTEM` or `USER` (custom)."},
					"enabled":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is enabled."},
					"tags":         schema.SetAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Tags, for example `MFA`, `Passwordless`, `Social-Login`."},
				}},
			},
		},
	}
}

func (d *listDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = tfutil.ConfigureDataSource(req, resp)
}

func (d *listDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m listModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var list []map[string]any
	if err := d.client.Get(ctx, d.client.ServerPath("/authenticators"), nil, &list); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "list authenticators", err)
		return
	}
	out := []authModel{}
	for _, a := range list {
		typ := tfutil.GetStringOrEmpty(a, "type").ValueString()
		if m.Type.ValueString() != "" && typ != m.Type.ValueString() {
			continue
		}
		out = append(out, authModel{
			ID: tfutil.GetStringOrEmpty(a, "id"), Name: tfutil.GetStringOrEmpty(a, "name"), DisplayName: tfutil.GetStringOrEmpty(a, "displayName"),
			Type: types.StringValue(typ), DefinedBy: tfutil.GetStringOrEmpty(a, "definedBy"), Enabled: tfutil.GetBool(a, "isEnabled", false),
			Tags: tfutil.SetOfStrings(tfutil.GetStrings(a, "tags"), false),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name.ValueString() < out[j].Name.ValueString() })
	m.Authenticators = tfutil.ListFrom(ctx, authTypes, out, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
