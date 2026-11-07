package idp

import (
	"context"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var _ datasource.DataSourceWithConfigure = (*metadataDataSource)(nil)

// NewMetadataDataSource returns the wso2is_federated_authenticator_metadata data source.
func NewMetadataDataSource() datasource.DataSource { return &metadataDataSource{} }

type metadataDataSource struct{ client *client.Client }

type metadataModel struct {
	Name        types.String `tfsdk:"name"`
	Kind        types.String `tfsdk:"kind"`
	ID          types.String `tfsdk:"id"`
	DisplayName types.String `tfsdk:"display_name"`
	Properties  types.List   `tfsdk:"properties"`
}

type propertyMetaModel struct {
	Key          types.String `tfsdk:"key"`
	DisplayName  types.String `tfsdk:"display_name"`
	Description  types.String `tfsdk:"description"`
	Type         types.String `tfsdk:"type"`
	Mandatory    types.Bool   `tfsdk:"mandatory"`
	Confidential types.Bool   `tfsdk:"confidential"`
	DefaultValue types.String `tfsdk:"default_value"`
	Options      types.List   `tfsdk:"options"`
}

var propertyMetaTypes = map[string]attr.Type{
	"key": types.StringType, "display_name": types.StringType, "description": types.StringType, "type": types.StringType,
	"mandatory": types.BoolType, "confidential": types.BoolType, "default_value": types.StringType, "options": types.ListType{ElemType: types.StringType},
}

func (d *metadataDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_federated_authenticator_metadata"
}

func (d *metadataDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Describes the properties that a federated authenticator or an outbound provisioning connector accepts. Use it when you write a `wso2is_identity_provider` configuration. Put confidential properties in `secret_properties`.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{Required: true, MarkdownDescription: "Authenticator or connector name, for example `GoogleOIDCAuthenticator` or `SCIM2`."},
			"kind": schema.StringAttribute{
				Optional: true, MarkdownDescription: "`federated-authenticators` (default) or `outbound-provisioning-connectors`.",
				Validators: []validator.String{stringvalidator.OneOf("federated-authenticators", "outbound-provisioning-connectors")},
			},
			"id":           schema.StringAttribute{Computed: true, MarkdownDescription: "API ID."},
			"display_name": schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
			"properties": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Accepted properties.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"key":           schema.StringAttribute{Computed: true, MarkdownDescription: "Property key."},
					"display_name":  schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
					"description":   schema.StringAttribute{Computed: true, MarkdownDescription: "Description."},
					"type":          schema.StringAttribute{Computed: true, MarkdownDescription: "`STRING`, `BOOLEAN`, ..."},
					"mandatory":     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is required."},
					"confidential":  schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether it is a secret."},
					"default_value": schema.StringAttribute{Computed: true, MarkdownDescription: "Default value."},
					"options":       schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Allowed values."},
				}},
			},
		},
	}
}

func (d *metadataDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = tfutil.ConfigureDataSource(req, resp)
}

func (d *metadataDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m metadataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	kind := m.Kind.ValueString()
	if kind == "" {
		kind = "federated-authenticators"
	}
	id := client.AuthenticatorID(m.Name.ValueString())
	var meta map[string]any
	if err := d.client.Get(ctx, d.client.ServerPath("/identity-providers/meta/"+kind+"/"+url.PathEscape(id)), nil, &meta); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "read authenticator metadata", err)
		return
	}
	m.ID = types.StringValue(id)
	m.Kind = types.StringValue(kind)
	m.DisplayName = tfutil.GetStringOrEmpty(meta, "displayName")
	props := []propertyMetaModel{}
	for _, p := range tfutil.GetObjects(meta, "properties") {
		props = append(props, propertyMetaModel{
			Key: tfutil.GetStringOrEmpty(p, "key"), DisplayName: tfutil.GetStringOrEmpty(p, "displayName"), Description: tfutil.GetStringOrEmpty(p, "description"),
			Type: tfutil.GetStringOrEmpty(p, "type"), Mandatory: tfutil.GetBool(p, "isMandatory", false), Confidential: tfutil.GetBool(p, "isConfidential", false),
			DefaultValue: tfutil.GetStringOrEmpty(p, "defaultValue"), Options: tfutil.ListOfStrings(tfutil.GetStrings(p, "options"), false),
		})
	}
	sort.Slice(props, func(i, j int) bool { return props[i].Key.ValueString() < props[j].Key.ValueString() })
	m.Properties = tfutil.ListFrom(ctx, propertyMetaTypes, props, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
