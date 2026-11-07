// Package idp implements wso2is_identity_provider (connections) and its data source.
package idp

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
	_ resource.ResourceWithConfigure   = (*idpResource)(nil)
	_ resource.ResourceWithImportState = (*idpResource)(nil)
)

// NewResource returns the wso2is_identity_provider resource.
func NewResource() resource.Resource { return &idpResource{} }

type idpResource struct{ client *client.Client }

type idpModel struct {
	ID                             types.String `tfsdk:"id"`
	Name                           types.String `tfsdk:"name"`
	Description                    types.String `tfsdk:"description"`
	ImageURL                       types.String `tfsdk:"image_url"`
	TemplateID                     types.String `tfsdk:"template_id"`
	Enabled                        types.Bool   `tfsdk:"enabled"`
	IsPrimary                      types.Bool   `tfsdk:"is_primary"`
	IsFederationHub                types.Bool   `tfsdk:"is_federation_hub"`
	HomeRealmIdentifier            types.String `tfsdk:"home_realm_identifier"`
	Alias                          types.String `tfsdk:"alias"`
	IdpIssuerName                  types.String `tfsdk:"idp_issuer_name"`
	JWKSURI                        types.String `tfsdk:"jwks_uri"`
	Certificates                   types.List   `tfsdk:"certificates"`
	DefaultAuthenticator           types.String `tfsdk:"default_authenticator"`
	FederatedAuthenticators        types.Map    `tfsdk:"federated_authenticators"`
	Claims                         types.Object `tfsdk:"claims"`
	RoleMappings                   types.Set    `tfsdk:"role_mappings"`
	OutboundProvisioningRoles      types.Set    `tfsdk:"outbound_provisioning_roles"`
	Groups                         types.Set    `tfsdk:"groups"`
	JITProvisioning                types.Object `tfsdk:"jit_provisioning"`
	ImplicitAssociation            types.Object `tfsdk:"implicit_association"`
	DefaultOutboundConnector       types.String `tfsdk:"default_outbound_connector"`
	OutboundProvisioningConnectors types.Map    `tfsdk:"outbound_provisioning_connectors"`
	ForceDelete                    types.Bool   `tfsdk:"force_delete"`
}

type authenticatorModel struct {
	Enabled          types.Bool   `tfsdk:"enabled"`
	Properties       types.Map    `tfsdk:"properties"`
	SecretProperties types.Map    `tfsdk:"secret_properties"`
	Endpoint         types.Object `tfsdk:"endpoint"`
}

type endpointModel struct {
	URI                      types.String `tfsdk:"uri"`
	AuthenticationType       types.String `tfsdk:"authentication_type"`
	AuthenticationProperties types.Map    `tfsdk:"authentication_properties"`
}

type claimsModel struct {
	UserIDClaim        types.String `tfsdk:"user_id_claim"`
	RoleClaim          types.String `tfsdk:"role_claim"`
	Mappings           types.Set    `tfsdk:"mappings"`
	ProvisioningClaims types.Set    `tfsdk:"provisioning_claims"`
}

type claimMappingModel struct {
	IdpClaim   types.String `tfsdk:"idp_claim"`
	LocalClaim types.String `tfsdk:"local_claim"`
}

type provisioningClaimModel struct {
	Claim        types.String `tfsdk:"claim"`
	DefaultValue types.String `tfsdk:"default_value"`
}

type roleMappingModel struct {
	IdpRole   types.String `tfsdk:"idp_role"`
	LocalRole types.String `tfsdk:"local_role"`
}

type jitModel struct {
	Enabled                        types.Bool   `tfsdk:"enabled"`
	Scheme                         types.String `tfsdk:"scheme"`
	UserStore                      types.String `tfsdk:"user_store"`
	AssociateLocalUser             types.Bool   `tfsdk:"associate_local_user"`
	SkipJITForLookupFailure        types.Bool   `tfsdk:"skip_jit_for_lookup_failure"`
	AttributeSyncMethod            types.String `tfsdk:"attribute_sync_method"`
	IdpGroupSyncMethod             types.String `tfsdk:"idp_group_sync_method"`
	AccountLookupAttributeMappings types.List   `tfsdk:"account_lookup_attribute_mappings"`
}

type lookupMappingModel struct {
	LocalAttribute     types.String `tfsdk:"local_attribute"`
	FederatedAttribute types.String `tfsdk:"federated_attribute"`
}

type implicitAssociationModel struct {
	Enabled          types.Bool `tfsdk:"enabled"`
	LookupAttributes types.List `tfsdk:"lookup_attributes"`
}

type connectorModel struct {
	Enabled          types.Bool `tfsdk:"enabled"`
	BlockingEnabled  types.Bool `tfsdk:"blocking_enabled"`
	RulesEnabled     types.Bool `tfsdk:"rules_enabled"`
	Properties       types.Map  `tfsdk:"properties"`
	SecretProperties types.Map  `tfsdk:"secret_properties"`
}

func (r *idpResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_provider"
}

func propsAttr(desc string, sensitive bool) schema.MapAttribute {
	return schema.MapAttribute{MarkdownDescription: desc, Optional: true, Sensitive: sensitive, ElementType: types.StringType}
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manages an identity provider, which the Console calls a connection. An identity provider lets users sign in with another system. Examples are social logins (Google, GitHub, Facebook, Microsoft and Apple) and any OpenID Connect or SAML provider. The resource also manages claim and role mappings, just-in-time provisioning and outbound provisioning.\n\nAuthenticator and connector settings are properties with the names that the server uses. The `wso2is_federated_authenticator_metadata` data source lists them. The provider only manages the property keys that you set. Put secrets in `secret_properties`, so that plans hide them. To use the identity provider for login, add it to an application's `authentication_sequence`. To import, use the identity provider's ID.",
		Attributes: map[string]schema.Attribute{
			"id":          tfutil.IDAttribute("Identity provider ID."),
			"name":        schema.StringAttribute{MarkdownDescription: "Unique name, used in authentication steps.", Required: true},
			"description": tfutil.String("Description.", ""),
			"image_url":   tfutil.String("Logo URL.", ""),
			"template_id": schema.StringAttribute{
				MarkdownDescription: "The Console template, for example `google-idp`, `github-idp`, `enterprise-oidc-idp` or `enterprise-saml-idp`. The template only changes how the Console shows the connection. The server only accepts it at creation, so changing it replaces the connection.",
				Optional:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled":               tfutil.Bool("Whether the connection can be used.", true),
			"is_primary":            tfutil.Bool("Use as the primary (default) identity provider.", false),
			"is_federation_hub":     tfutil.Bool("The provider is a federation hub (it fronts other providers).", false),
			"home_realm_identifier": tfutil.String("Home realm identifier (`fidp` request parameter value) that selects this provider.", ""),
			"alias":                 tfutil.String("Alias used as the audience of tokens this provider issues (for example the token endpoint of the resident IdP).", ""),
			"idp_issuer_name":       tfutil.String("Issuer of tokens from this provider, when different from its name (used to validate JWT bearer grants).", ""),
			"jwks_uri": schema.StringAttribute{
				MarkdownDescription: "JWKS endpoint used to validate tokens signed by the provider. Conflicts with `certificates`.",
				Optional:            true,
				Validators:          []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("certificates"))},
			},
			"certificates": schema.ListAttribute{
				MarkdownDescription: "PEM certificates of the provider (for validating signed responses and tokens). Conflicts with `jwks_uri`.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"default_authenticator": schema.StringAttribute{
				MarkdownDescription: "Name of the federated authenticator used by default. The default is the only (or first) configured authenticator.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString()},
			},
			"federated_authenticators": schema.MapNestedAttribute{
				MarkdownDescription: "Federated authenticators, keyed by authenticator name: `GoogleOIDCAuthenticator`, `GithubAuthenticator`, `FacebookAuthenticator`, " +
					"`MicrosoftWindowsLiveAuthenticator`, `Office365Authenticator`, `AppleOIDCAuthenticator`, `TwitterAuthenticator`, `OpenIDConnectAuthenticator`, " +
					"`SAMLSSOAuthenticator`, `PassiveSTSAuthenticator`, `IWAKerberosAuthenticator`, `EmailOTP`, `SMSOTP`, or a custom authenticator.",
				Optional: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"enabled":           schema.BoolAttribute{MarkdownDescription: "Whether the authenticator is enabled. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
					"properties":        propsAttr("Authenticator properties, for example `ClientId`, `callbackUrl`, `Scopes`, `OAuth2AuthzEPUrl`, `OAuth2TokenEPUrl`, `UserInfoUrl` (OIDC) or `IdPEntityId`, `SSOUrl`, `SPEntityId` (SAML).", false),
					"secret_properties": propsAttr("Confidential properties, for example `ClientSecret`.", true),
					"endpoint": schema.SingleNestedAttribute{
						MarkdownDescription: "External service endpoint of a custom (user defined) federated authenticator.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"uri": schema.StringAttribute{MarkdownDescription: "HTTPS endpoint.", Required: true},
							"authentication_type": schema.StringAttribute{
								MarkdownDescription: "`NONE`, `BASIC`, `BEARER` or `API_KEY`.",
								Required:            true,
								Validators:          []validator.String{stringvalidator.OneOf("NONE", "BASIC", "BEARER", "API_KEY")},
							},
							"authentication_properties": schema.MapAttribute{
								MarkdownDescription: "Credentials: `username`/`password` (BASIC), `accessToken` (BEARER) or `header`/`value` (API_KEY). The server never returns this value.",
								Optional:            true,
								Sensitive:           true,
								ElementType:         types.StringType,
							},
						},
					},
				}},
			},
			"claims": tfutil.Object("Claim mappings from the provider's attributes to local claims.", map[string]schema.Attribute{
				"user_id_claim": tfutil.String("The identity provider's claim that identifies the user. When it is empty, the server uses the subject.", ""),
				"role_claim":    tfutil.String("Provider claim that carries roles.", ""),
				"mappings": tfutil.ObjectSet("Provider claim to local claim mappings. Without mappings the provider's claims are used as is.", map[string]schema.Attribute{
					"idp_claim":   schema.StringAttribute{MarkdownDescription: "Claim name used by the provider.", Required: true},
					"local_claim": schema.StringAttribute{MarkdownDescription: "Local claim URI.", Required: true},
				}),
				"provisioning_claims": tfutil.ObjectSet("Default values of claims for outbound provisioning.", map[string]schema.Attribute{
					"claim":         schema.StringAttribute{MarkdownDescription: "Claim URI.", Required: true},
					"default_value": schema.StringAttribute{MarkdownDescription: "Default value.", Required: true},
				}),
			}),
			"role_mappings": tfutil.ObjectSet("Provider role to local role mappings.", map[string]schema.Attribute{
				"idp_role": schema.StringAttribute{MarkdownDescription: "Role name sent by the provider.", Required: true},
				"local_role": schema.StringAttribute{
					MarkdownDescription: "Local role, as `Internal/<role name>` for organization roles (WSO2 silently ignores other formats).",
					Required:            true,
					Validators:          []validator.String{stringvalidator.RegexMatches(regexp.MustCompile(`^[^/]+/.+$`), "must be qualified with its domain, for example Internal/admin")},
				},
			}),
			"outbound_provisioning_roles": tfutil.StringSet("Only users with these roles are provisioned to this provider."),
			"groups":                      tfutil.StringSet("Groups defined at the provider that can be mapped to local roles."),
			"jit_provisioning": tfutil.Object("Just-in-time provisioning of local accounts for federated users.", map[string]schema.Attribute{
				"enabled":                     tfutil.Bool("Create local users on first login.", false),
				"scheme":                      enumString("`PROVISION_SILENTLY`, `PROMPT_CONSENT`, `PROMPT_PASSWORD_CONSENT` or `PROMPT_USERNAME_PASSWORD_CONSENT`.", "PROVISION_SILENTLY", "PROVISION_SILENTLY", "PROMPT_CONSENT", "PROMPT_PASSWORD_CONSENT", "PROMPT_USERNAME_PASSWORD_CONSENT"),
				"user_store":                  tfutil.String("User store for provisioned users.", "PRIMARY"),
				"associate_local_user":        tfutil.Bool("Link to an existing local user found through the lookup attributes.", false),
				"skip_jit_for_lookup_failure": tfutil.Bool("Skip provisioning when the local user lookup fails.", false),
				"attribute_sync_method":       enumString("`OVERRIDE_ALL`, `PRESERVE_LOCAL` or `NONE`.", "OVERRIDE_ALL", "OVERRIDE_ALL", "PRESERVE_LOCAL", "NONE"),
				"idp_group_sync_method":       enumString("`MERGE_WITH_EXISTING` or `OVERRIDE_ALL`.", "MERGE_WITH_EXISTING", "MERGE_WITH_EXISTING", "OVERRIDE_ALL"),
				"account_lookup_attribute_mappings": tfutil.ObjectList("Local and federated attributes used to find an existing local user.", map[string]schema.Attribute{
					"local_attribute":     schema.StringAttribute{MarkdownDescription: "Local claim URI.", Required: true},
					"federated_attribute": schema.StringAttribute{MarkdownDescription: "Provider claim.", Required: true},
				}),
			}),
			"implicit_association": tfutil.Object("Associate federated users with local users that have matching attributes.", map[string]schema.Attribute{
				"enabled":           tfutil.Bool("Enable implicit association.", false),
				"lookup_attributes": tfutil.StringList("Local claim URIs used to find the local user."),
			}),
			"default_outbound_connector": schema.StringAttribute{
				MarkdownDescription: "Name of the default outbound provisioning connector.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString()},
			},
			"outbound_provisioning_connectors": schema.MapNestedAttribute{
				MarkdownDescription: "Outbound provisioning connectors keyed by name: `SCIM2`, `scim`, `googleapps`, `salesforce`.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"enabled":           schema.BoolAttribute{MarkdownDescription: "Enabled. The default is `true`.", Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
					"blocking_enabled":  schema.BoolAttribute{MarkdownDescription: "Provision synchronously.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
					"rules_enabled":     schema.BoolAttribute{MarkdownDescription: "Apply provisioning rules.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
					"properties":        propsAttr("Connector properties, for example `scim2-user-ep`, `scim2-username`.", false),
					"secret_properties": propsAttr("Confidential connector properties, for example `scim2-password`.", true),
				}},
			},
			"force_delete": schema.BoolAttribute{
				MarkdownDescription: "Delete the connection even when applications still use it. The default is `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		},
	}
}

func enumString(desc, def string, values ...string) schema.StringAttribute {
	a := tfutil.String(desc, def)
	a.Validators = []validator.String{stringvalidator.OneOf(values...)}
	return a
}

var (
	idpAttrTypes       = resourceSchema().Type().(types.ObjectType).AttrTypes
	authTypes          = idpAttrTypes["federated_authenticators"].(types.MapType).ElemType.(types.ObjectType).AttrTypes
	endpointTypes      = authTypes["endpoint"].(types.ObjectType).AttrTypes
	claimsTypes        = tfutil.NestedAttrTypes(idpAttrTypes, "claims")
	claimMappingTypes  = tfutil.NestedAttrTypes(claimsTypes, "mappings")
	provClaimTypes     = tfutil.NestedAttrTypes(claimsTypes, "provisioning_claims")
	roleMappingTypes   = tfutil.NestedAttrTypes(idpAttrTypes, "role_mappings")
	jitTypes           = tfutil.NestedAttrTypes(idpAttrTypes, "jit_provisioning")
	lookupMappingTypes = tfutil.NestedAttrTypes(jitTypes, "account_lookup_attribute_mappings")
	implicitTypes      = tfutil.NestedAttrTypes(idpAttrTypes, "implicit_association")
	connectorTypes     = idpAttrTypes["outbound_provisioning_connectors"].(types.MapType).ElemType.(types.ObjectType).AttrTypes
)

func (r *idpResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *idpResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = tfutil.ConfigureResource(req, resp)
}

func encodeCert(pem string) string {
	return base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(pem) + "\n"))
}

func decodeCert(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return strings.TrimSpace(string(b)) + "\n"
}

func props(m map[string]string) []map[string]any {
	out := make([]map[string]any, 0, len(m))
	for _, k := range tfutil.SortedKeys(m) {
		out = append(out, map[string]any{"key": k, "value": m[k]})
	}
	return out
}

// authenticatorBodies builds the federated authenticator list. server holds the current
// properties per authenticator ID so unmanaged keys survive updates; prevManaged lists
// keys managed before (so removed keys are dropped).
func authenticatorBodies(ctx context.Context, m *idpModel, server map[string]map[string]string, prevManaged map[string]map[string]bool, diags *diag.Diagnostics) (string, []map[string]any) {
	var auths map[string]authenticatorModel
	if !m.FederatedAuthenticators.IsNull() && !m.FederatedAuthenticators.IsUnknown() {
		diags.Append(m.FederatedAuthenticators.ElementsAs(ctx, &auths, false)...)
	}
	names := tfutil.SortedKeys(auths)
	def := m.DefaultAuthenticator.ValueString()
	if _, ok := auths[def]; !ok {
		if len(names) > 0 {
			def = names[0]
		}
	}
	list := make([]map[string]any, 0, len(names))
	for _, name := range names {
		a := auths[name]
		id := client.AuthenticatorID(name)
		merged := map[string]string{}
		for k, v := range server[id] {
			if !prevManaged[name][k] {
				merged[k] = v
			}
		}
		for k, v := range tfutil.MapOfStrings(ctx, a.Properties, diags) {
			merged[k] = v
		}
		for k, v := range tfutil.MapOfStrings(ctx, a.SecretProperties, diags) {
			merged[k] = v
		}
		body := map[string]any{
			"authenticatorId": id,
			"isEnabled":       a.Enabled.ValueBool(),
			"isDefault":       name == def,
			"properties":      props(merged),
		}
		if e, ok := tfutil.ObjectAs[endpointModel](ctx, a.Endpoint, diags); ok {
			auth := map[string]any{"type": e.AuthenticationType.ValueString()}
			if p := tfutil.MapOfStrings(ctx, e.AuthenticationProperties, diags); len(p) > 0 {
				ap := map[string]any{}
				for k, v := range p {
					ap[k] = v
				}
				auth["properties"] = ap
			}
			body["endpoint"] = map[string]any{"uri": e.URI.ValueString(), "authentication": auth}
			body["definedBy"] = "USER"
		}
		list = append(list, body)
	}
	if def == "" {
		return "", list
	}
	return client.AuthenticatorID(def), list
}

func connectorBodies(ctx context.Context, m *idpModel, server map[string]map[string]string, prevManaged map[string]map[string]bool, diags *diag.Diagnostics) (string, []map[string]any) {
	var conns map[string]connectorModel
	if !m.OutboundProvisioningConnectors.IsNull() && !m.OutboundProvisioningConnectors.IsUnknown() {
		diags.Append(m.OutboundProvisioningConnectors.ElementsAs(ctx, &conns, false)...)
	}
	names := tfutil.SortedKeys(conns)
	def := m.DefaultOutboundConnector.ValueString()
	if _, ok := conns[def]; !ok && len(names) > 0 {
		def = names[0]
	}
	list := make([]map[string]any, 0, len(names))
	for _, name := range names {
		c := conns[name]
		id := client.AuthenticatorID(name)
		merged := map[string]string{}
		for k, v := range server[id] {
			if !prevManaged[name][k] {
				merged[k] = v
			}
		}
		for k, v := range tfutil.MapOfStrings(ctx, c.Properties, diags) {
			merged[k] = v
		}
		for k, v := range tfutil.MapOfStrings(ctx, c.SecretProperties, diags) {
			merged[k] = v
		}
		list = append(list, map[string]any{
			"connectorId":     id,
			"isEnabled":       c.Enabled.ValueBool(),
			"isDefault":       name == def,
			"blockingEnabled": c.BlockingEnabled.ValueBool(),
			"rulesEnabled":    c.RulesEnabled.ValueBool(),
			"properties":      props(merged),
		})
	}
	if def == "" {
		return "", list
	}
	return client.AuthenticatorID(def), list
}

func claimsBody(ctx context.Context, obj types.Object, diags *diag.Diagnostics) map[string]any {
	c, ok := tfutil.ObjectAs[claimsModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	body := map[string]any{"mappings": []any{}, "provisioningClaims": []any{}}
	if v := c.UserIDClaim.ValueString(); v != "" {
		body["userIdClaim"] = map[string]any{"uri": v}
	}
	if v := c.RoleClaim.ValueString(); v != "" {
		body["roleClaim"] = map[string]any{"uri": v}
	}
	if ms, ok := tfutil.ElementsAs[claimMappingModel](ctx, c.Mappings, diags); ok {
		list := []any{}
		for _, mm := range ms {
			list = append(list, map[string]any{"idpClaim": mm.IdpClaim.ValueString(), "localClaim": map[string]any{"uri": mm.LocalClaim.ValueString()}})
		}
		body["mappings"] = list
	}
	if pcs, ok := tfutil.ElementsAs[provisioningClaimModel](ctx, c.ProvisioningClaims, diags); ok {
		list := []any{}
		for _, pc := range pcs {
			list = append(list, map[string]any{"claim": map[string]any{"uri": pc.Claim.ValueString()}, "defaultValue": pc.DefaultValue.ValueString()})
		}
		body["provisioningClaims"] = list
	}
	return body
}

func rolesBody(ctx context.Context, m *idpModel, diags *diag.Diagnostics) map[string]any {
	body := map[string]any{"mappings": []any{}, "outboundProvisioningRoles": nonNil(tfutil.Strings(ctx, m.OutboundProvisioningRoles, diags))}
	if rms, ok := tfutil.ElementsAs[roleMappingModel](ctx, m.RoleMappings, diags); ok {
		list := []any{}
		for _, rm := range rms {
			list = append(list, map[string]any{"idpRole": rm.IdpRole.ValueString(), "localRole": rm.LocalRole.ValueString()})
		}
		body["mappings"] = list
	}
	return body
}

func groupsBody(ctx context.Context, m *idpModel, diags *diag.Diagnostics) []map[string]any {
	out := []map[string]any{}
	for _, g := range tfutil.Strings(ctx, m.Groups, diags) {
		out = append(out, map[string]any{"name": g})
	}
	return out
}

func jitBody(ctx context.Context, obj types.Object, diags *diag.Diagnostics) map[string]any {
	j, ok := tfutil.ObjectAs[jitModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	body := tfutil.JSON{}.Bool("isEnabled", j.Enabled).Str("scheme", j.Scheme).Str("userstore", j.UserStore).
		Bool("associateLocalUser", j.AssociateLocalUser).Bool("skipJITForLookupFailure", j.SkipJITForLookupFailure).
		Str("attributeSyncMethod", j.AttributeSyncMethod).Map()
	// Servers before 7.3 reject idpGroupSyncMethod; the default is what an omitted value means.
	if v := j.IdpGroupSyncMethod.ValueString(); v != "" && v != "MERGE_WITH_EXISTING" {
		body["idpGroupSyncMethod"] = v
	}
	if lms, ok := tfutil.ElementsAs[lookupMappingModel](ctx, j.AccountLookupAttributeMappings, diags); ok {
		list := []any{}
		for _, lm := range lms {
			list = append(list, map[string]any{"localAttribute": lm.LocalAttribute.ValueString(), "federatedAttribute": lm.FederatedAttribute.ValueString()})
		}
		body["accountLookupAttributeMappings"] = list
	}
	return body
}

func implicitBody(ctx context.Context, obj types.Object, diags *diag.Diagnostics) map[string]any {
	ia, ok := tfutil.ObjectAs[implicitAssociationModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	return map[string]any{"isEnabled": ia.Enabled.ValueBool(), "lookupAttribute": nonNil(tfutil.Strings(ctx, ia.LookupAttributes, diags))}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (r *idpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan idpModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := tfutil.JSON{}.Str("name", plan.Name).Str("description", plan.Description).Str("image", plan.ImageURL).
		Str("templateId", plan.TemplateID).Bool("isPrimary", plan.IsPrimary).Bool("isFederationHub", plan.IsFederationHub).
		Str("homeRealmIdentifier", plan.HomeRealmIdentifier).Str("alias", plan.Alias).Str("idpIssuerName", plan.IdpIssuerName).Map()
	if tfutil.Known(plan.JWKSURI) {
		body["certificate"] = map[string]any{"jwksUri": plan.JWKSURI.ValueString()}
	} else if certs := tfutil.Strings(ctx, plan.Certificates, &resp.Diagnostics); len(certs) > 0 {
		enc := make([]string, 0, len(certs))
		for _, c := range certs {
			enc = append(enc, encodeCert(c))
		}
		body["certificate"] = map[string]any{"certificates": enc}
	}
	if def, auths := authenticatorBodies(ctx, &plan, nil, nil, &resp.Diagnostics); len(auths) > 0 {
		body["federatedAuthenticators"] = map[string]any{"defaultAuthenticatorId": def, "authenticators": auths}
	}
	if c := claimsBody(ctx, plan.Claims, &resp.Diagnostics); c != nil {
		body["claims"] = c
	}
	body["roles"] = rolesBody(ctx, &plan, &resp.Diagnostics)
	if g := groupsBody(ctx, &plan, &resp.Diagnostics); len(g) > 0 {
		body["groups"] = g
	}
	prov := map[string]any{}
	if j := jitBody(ctx, plan.JITProvisioning, &resp.Diagnostics); j != nil {
		prov["jit"] = j
	}
	if def, conns := connectorBodies(ctx, &plan, nil, nil, &resp.Diagnostics); len(conns) > 0 {
		prov["outboundConnectors"] = map[string]any{"defaultConnectorId": def, "connectors": conns}
	}
	if len(prov) > 0 {
		body["provisioning"] = prov
	}
	// Servers before 7.3 reject implicitAssociation; only send it when it is used.
	if ia := implicitBody(ctx, plan.ImplicitAssociation, &resp.Diagnostics); ia != nil && (ia["isEnabled"] == true || len(ia["lookupAttribute"].([]string)) > 0) {
		body["implicitAssociation"] = ia
	}
	if resp.Diagnostics.HasError() {
		return
	}
	body = client.PruneEmpty(body, nil)
	id, err := r.client.CreateIdentityProvider(ctx, body)
	if err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "create identity provider", err)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	plan.ID = types.StringValue(id)
	// isEnabled is not part of the create model.
	if !plan.Enabled.ValueBool() {
		if err := r.client.PatchIdentityProvider(ctx, id, []client.IDPPatchOperation{{Operation: "REPLACE", Path: "/isEnabled", Value: false}}); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "disable identity provider", err)
			return
		}
	}
	r.checkRoleMappings(ctx, id, &plan, &resp.Diagnostics)
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Identity provider not found after creation", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *idpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state idpModel
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

func patchOp(prev, next, p string) *client.IDPPatchOperation {
	switch {
	case prev == next:
		return nil
	case next == "":
		return &client.IDPPatchOperation{Operation: "REMOVE", Path: p}
	case prev == "":
		return &client.IDPPatchOperation{Operation: "ADD", Path: p, Value: next}
	}
	return &client.IDPPatchOperation{Operation: "REPLACE", Path: p, Value: next}
}

func (r *idpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state idpModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	var ops []client.IDPPatchOperation
	for _, f := range []struct {
		old, new types.String
		path     string
	}{
		{state.Name, plan.Name, "/name"}, {state.Description, plan.Description, "/description"},
		{state.ImageURL, plan.ImageURL, "/image"}, {state.HomeRealmIdentifier, plan.HomeRealmIdentifier, "/homeRealmIdentifier"},
		{state.Alias, plan.Alias, "/alias"}, {state.IdpIssuerName, plan.IdpIssuerName, "/idpIssuerName"},
		{state.JWKSURI, plan.JWKSURI, "/certificate/jwksUri"},
	} {
		if op := patchOp(f.old.ValueString(), f.new.ValueString(), f.path); op != nil {
			ops = append(ops, *op)
		}
	}
	for _, f := range []struct {
		old, new types.Bool
		path     string
	}{{state.Enabled, plan.Enabled, "/isEnabled"}, {state.IsPrimary, plan.IsPrimary, "/isPrimary"}, {state.IsFederationHub, plan.IsFederationHub, "/isFederationHub"}} {
		if !f.old.Equal(f.new) {
			ops = append(ops, client.IDPPatchOperation{Operation: "REPLACE", Path: f.path, Value: f.new.ValueBool()})
		}
	}
	if !plan.Certificates.Equal(state.Certificates) {
		oldN := len(state.Certificates.Elements())
		for i := oldN - 1; i >= 0; i-- {
			ops = append(ops, client.IDPPatchOperation{Operation: "REMOVE", Path: "/certificate/certificates/" + itoa(i)})
		}
		for i, c := range tfutil.Strings(ctx, plan.Certificates, &resp.Diagnostics) {
			ops = append(ops, client.IDPPatchOperation{Operation: "ADD", Path: "/certificate/certificates/" + itoa(i), Value: encodeCert(c)})
		}
	}
	if err := r.client.PatchIdentityProvider(ctx, id, ops); err != nil {
		tfutil.AddAPIError(&resp.Diagnostics, "update identity provider", err)
		return
	}

	if !plan.FederatedAuthenticators.Equal(state.FederatedAuthenticators) || !plan.DefaultAuthenticator.Equal(state.DefaultAuthenticator) {
		server, err := r.authenticatorProperties(ctx, id)
		if err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "read federated authenticators", err)
			return
		}
		def, auths := authenticatorBodies(ctx, &plan, server, managedKeys(ctx, state.FederatedAuthenticators, &resp.Diagnostics), &resp.Diagnostics)
		if err := r.client.PutIdentityProviderSub(ctx, id, "federated-authenticators", map[string]any{"defaultAuthenticatorId": def, "authenticators": auths}); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update federated authenticators", err)
			return
		}
	}
	if !plan.Claims.Equal(state.Claims) {
		if err := r.client.PutIdentityProviderSub(ctx, id, "claims", claimsBody(ctx, plan.Claims, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update identity provider claims", err)
			return
		}
	}
	if !plan.RoleMappings.Equal(state.RoleMappings) || !plan.OutboundProvisioningRoles.Equal(state.OutboundProvisioningRoles) {
		if err := r.client.PutIdentityProviderSub(ctx, id, "roles", rolesBody(ctx, &plan, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update identity provider roles", err)
			return
		}
	}
	r.checkRoleMappings(ctx, id, &plan, &resp.Diagnostics)
	if !plan.Groups.Equal(state.Groups) {
		if err := r.client.PutIdentityProviderSub(ctx, id, "groups", groupsBody(ctx, &plan, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update identity provider groups", err)
			return
		}
	}
	if !plan.JITProvisioning.Equal(state.JITProvisioning) {
		if err := r.client.PutIdentityProviderSub(ctx, id, "provisioning/jit", jitBody(ctx, plan.JITProvisioning, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update just-in-time provisioning", err)
			return
		}
	}
	if !plan.ImplicitAssociation.Equal(state.ImplicitAssociation) {
		if err := r.client.PutIdentityProviderSub(ctx, id, "implicit-association", implicitBody(ctx, plan.ImplicitAssociation, &resp.Diagnostics)); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update implicit association", err)
			return
		}
	}
	if !plan.OutboundProvisioningConnectors.Equal(state.OutboundProvisioningConnectors) || !plan.DefaultOutboundConnector.Equal(state.DefaultOutboundConnector) {
		server, err := r.connectorProperties(ctx, id)
		if err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "read outbound connectors", err)
			return
		}
		def, conns := connectorBodies(ctx, &plan, server, managedKeys(ctx, state.OutboundProvisioningConnectors, &resp.Diagnostics), &resp.Diagnostics)
		if err := r.client.PutIdentityProviderSub(ctx, id, "provisioning/outbound-connectors", map[string]any{"defaultConnectorId": def, "connectors": conns}); err != nil {
			tfutil.AddAPIError(&resp.Diagnostics, "update outbound provisioning connectors", err)
			return
		}
	}

	plan.ID = state.ID
	if !r.read(ctx, &plan, &resp.Diagnostics) && !resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Identity provider not found after update", id)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func itoa(i int) string { return strconv.Itoa(i) }

// managedKeys returns the property keys (plain and secret) managed per authenticator
// or connector in a state map.
func managedKeys(ctx context.Context, v types.Map, diags *diag.Diagnostics) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	if v.IsNull() || v.IsUnknown() {
		return out
	}
	var items map[string]struct {
		Properties       types.Map `tfsdk:"properties"`
		SecretProperties types.Map `tfsdk:"secret_properties"`
	}
	for name, raw := range v.Elements() {
		obj, ok := raw.(types.Object)
		if !ok {
			continue
		}
		keys := map[string]bool{}
		for _, field := range []string{"properties", "secret_properties"} {
			if m, ok := obj.Attributes()[field].(types.Map); ok {
				for k := range m.Elements() {
					keys[k] = true
				}
			}
		}
		out[name] = keys
	}
	_ = items
	return out
}

func (r *idpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state idpModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteIdentityProvider(ctx, state.ID.ValueString(), state.ForceDelete.ValueBool()); err != nil && !client.IsNotFound(err) {
		tfutil.AddAPIError(&resp.Diagnostics, "delete identity provider", err)
	}
}

func (r *idpResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_delete"), false)...)
}

// authenticatorProperties returns the current properties of every federated
// authenticator, by authenticator ID.
func (r *idpResource) authenticatorProperties(ctx context.Context, id string) (map[string]map[string]string, error) {
	var list struct {
		DefaultAuthenticatorID string `json:"defaultAuthenticatorId"`
		Authenticators         []struct {
			AuthenticatorID string `json:"authenticatorId"`
		} `json:"authenticators"`
	}
	if err := r.client.GetIdentityProviderSub(ctx, id, "federated-authenticators", &list); err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, a := range list.Authenticators {
		var detail map[string]any
		if err := r.client.GetIdentityProviderSub(ctx, id, "federated-authenticators/"+a.AuthenticatorID, &detail); err != nil {
			return nil, err
		}
		out[a.AuthenticatorID] = propertyMap(detail)
	}
	return out, nil
}

func (r *idpResource) connectorProperties(ctx context.Context, id string) (map[string]map[string]string, error) {
	var list struct {
		Connectors []struct {
			ConnectorID string `json:"connectorId"`
		} `json:"connectors"`
	}
	if err := r.client.GetIdentityProviderSub(ctx, id, "provisioning/outbound-connectors", &list); err != nil {
		return nil, err
	}
	out := map[string]map[string]string{}
	for _, c := range list.Connectors {
		var detail map[string]any
		if err := r.client.GetIdentityProviderSub(ctx, id, "provisioning/outbound-connectors/"+c.ConnectorID, &detail); err != nil {
			return nil, err
		}
		out[c.ConnectorID] = propertyMap(detail)
	}
	return out, nil
}

// masked reports whether the server hid a confidential value (outbound connector secrets
// come back as asterisks; authenticator secrets are returned as stored).
func masked(v string) bool {
	return v != "" && strings.Trim(v, "*") == ""
}

func priorSecretValue(prior types.Map, k, v string) string {
	if masked(v) {
		if pv, ok := prior.Elements()[k].(types.String); ok {
			return pv.ValueString()
		}
	}
	return v
}

func propertyMap(detail map[string]any) map[string]string {
	out := map[string]string{}
	for _, p := range tfutil.GetObjects(detail, "properties") {
		k, _ := p["key"].(string)
		v, _ := p["value"].(string)
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func decodeName(id string) string {
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(id, "="))
	if err != nil {
		return id
	}
	return string(b)
}

func (r *idpResource) read(ctx context.Context, m *idpModel, diags *diag.Diagnostics) bool {
	idp, err := r.client.GetIdentityProvider(ctx, m.ID.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			return false
		}
		tfutil.AddAPIError(diags, "read identity provider", err)
		return false
	}
	importing := m.Name.IsNull()
	m.Name = tfutil.GetString(idp, "name")
	m.Description = tfutil.GetStringOrEmpty(idp, "description")
	m.ImageURL = tfutil.GetStringOrEmpty(idp, "image")
	if t := tfutil.GetString(idp, "templateId"); tfutil.Known(t) && t.ValueString() != "" {
		m.TemplateID = t
	}
	m.Enabled = tfutil.GetBool(idp, "isEnabled", true)
	m.IsPrimary = tfutil.GetBool(idp, "isPrimary", false)
	m.IsFederationHub = tfutil.GetBool(idp, "isFederationHub", false)
	m.HomeRealmIdentifier = tfutil.GetStringOrEmpty(idp, "homeRealmIdentifier")
	m.Alias = tfutil.GetStringOrEmpty(idp, "alias")
	m.IdpIssuerName = tfutil.GetStringOrEmpty(idp, "idpIssuerName")

	cert := tfutil.GetMap(idp, "certificate")
	m.JWKSURI = tfutil.StringValueOrNull(tfutil.GetStringOrEmpty(cert, "jwksUri").ValueString())
	var certs []string
	for _, c := range tfutil.GetStrings(cert, "certificates") {
		certs = append(certs, decodeCert(c))
	}
	if prior := tfutil.Strings(ctx, m.Certificates, diags); len(prior) == len(certs) {
		// Keep the configured PEM formatting when the certificates are the same.
		same := true
		for i := range prior {
			if strings.TrimSpace(prior[i]) != strings.TrimSpace(certs[i]) {
				same = false
			}
		}
		if same {
			certs = prior
		}
	}
	m.Certificates = tfutil.ListOfStrings(certs, true)

	// Federated authenticators.
	fa := tfutil.GetMap(idp, "federatedAuthenticators")
	defID, _ := fa["defaultAuthenticatorId"].(string)
	m.DefaultAuthenticator = tfutil.StringValueOrNull(decodeName(defID))
	if defID == "" {
		m.DefaultAuthenticator = types.StringValue("")
	}
	priorAuths := map[string]authenticatorModel{}
	if !m.FederatedAuthenticators.IsNull() && !m.FederatedAuthenticators.IsUnknown() {
		diags.Append(m.FederatedAuthenticators.ElementsAs(ctx, &priorAuths, false)...)
	}
	auths := map[string]attr.Value{}
	for _, a := range tfutil.GetObjects(fa, "authenticators") {
		aid, _ := a["authenticatorId"].(string)
		name, _ := a["name"].(string)
		if name == "" {
			name = decodeName(aid)
		}
		var detail map[string]any
		if err := r.client.GetIdentityProviderSub(ctx, m.ID.ValueString(), "federated-authenticators/"+aid, &detail); err != nil {
			tfutil.AddAPIError(diags, "read federated authenticator "+name, err)
			return false
		}
		prior, known := priorAuths[name]
		all := propertyMap(detail)
		plain, secret := map[string]string{}, map[string]string{}
		priorPlain := prior.Properties.Elements()
		priorSecret := prior.SecretProperties.Elements()
		for k, v := range all {
			switch {
			case priorSecret[k] != nil:
				secret[k] = priorSecretValue(prior.SecretProperties, k, v)
			case priorPlain[k] != nil || (!known && importing):
				plain[k] = v
			}
		}
		am := authenticatorModel{
			Enabled:          tfutil.GetBool(detail, "isEnabled", false),
			Properties:       tfutil.MapValueOfStrings(plain, prior.Properties.IsNull() && (!importing || len(plain) == 0)),
			SecretProperties: tfutil.MapValueOfStrings(secret, prior.SecretProperties.IsNull()),
			Endpoint:         types.ObjectNull(endpointTypes),
		}
		if ep := tfutil.GetMap(detail, "endpoint"); ep != nil {
			auth := tfutil.GetMap(ep, "authentication")
			em := endpointModel{URI: tfutil.GetString(ep, "uri"), AuthenticationType: tfutil.GetString(auth, "type"), AuthenticationProperties: types.MapNull(types.StringType)}
			if pe, ok := tfutil.ObjectAs[endpointModel](ctx, prior.Endpoint, diags); ok {
				em.AuthenticationProperties = pe.AuthenticationProperties // write-only
			}
			am.Endpoint = tfutil.ObjectFrom(ctx, endpointTypes, em, diags)
		}
		auths[name] = tfutil.ObjectFrom(ctx, authTypes, am, diags)
	}
	if len(auths) == 0 && m.FederatedAuthenticators.IsNull() {
		m.FederatedAuthenticators = types.MapNull(types.ObjectType{AttrTypes: authTypes})
	} else {
		v, d := types.MapValue(types.ObjectType{AttrTypes: authTypes}, auths)
		diags.Append(d...)
		m.FederatedAuthenticators = v
	}

	// Claims.
	cl := tfutil.GetMap(idp, "claims")
	cms := []claimMappingModel{}
	for _, cm := range tfutil.GetObjects(cl, "mappings") {
		cms = append(cms, claimMappingModel{IdpClaim: tfutil.GetString(cm, "idpClaim"), LocalClaim: tfutil.GetString(tfutil.GetMap(cm, "localClaim"), "uri")})
	}
	pcs := []provisioningClaimModel{}
	for _, pc := range tfutil.GetObjects(cl, "provisioningClaims") {
		pcs = append(pcs, provisioningClaimModel{Claim: tfutil.GetString(tfutil.GetMap(pc, "claim"), "uri"), DefaultValue: tfutil.GetStringOrEmpty(pc, "defaultValue")})
	}
	m.Claims = tfutil.ObjectFrom(ctx, claimsTypes, claimsModel{
		UserIDClaim:        tfutil.GetStringOrEmpty(tfutil.GetMap(cl, "userIdClaim"), "uri"),
		RoleClaim:          tfutil.GetStringOrEmpty(tfutil.GetMap(cl, "roleClaim"), "uri"),
		Mappings:           tfutil.SetFrom(ctx, claimMappingTypes, cms, diags),
		ProvisioningClaims: tfutil.SetFrom(ctx, provClaimTypes, pcs, diags),
	}, diags)

	// Roles and groups.
	ro := tfutil.GetMap(idp, "roles")
	rms := []roleMappingModel{}
	for _, rm := range tfutil.GetObjects(ro, "mappings") {
		rms = append(rms, roleMappingModel{IdpRole: tfutil.GetString(rm, "idpRole"), LocalRole: tfutil.GetString(rm, "localRole")})
	}
	m.RoleMappings = tfutil.SetFrom(ctx, roleMappingTypes, rms, diags)
	m.OutboundProvisioningRoles = tfutil.SetOfStrings(tfutil.GetStrings(ro, "outboundProvisioningRoles"), false)
	var groups []string
	for _, g := range tfutil.GetObjects(idp, "groups") {
		if n, _ := g["name"].(string); n != "" {
			groups = append(groups, n)
		}
	}
	sort.Strings(groups)
	m.Groups = tfutil.SetOfStrings(groups, false)

	// Provisioning.
	prov := tfutil.GetMap(idp, "provisioning")
	jit := tfutil.GetMap(prov, "jit")
	lms := []lookupMappingModel{}
	for _, lm := range tfutil.GetObjects(jit, "accountLookupAttributeMappings") {
		lms = append(lms, lookupMappingModel{LocalAttribute: tfutil.GetString(lm, "localAttribute"), FederatedAttribute: tfutil.GetString(lm, "federatedAttribute")})
	}
	m.JITProvisioning = tfutil.ObjectFrom(ctx, jitTypes, jitModel{
		Enabled:                        tfutil.GetBool(jit, "isEnabled", false),
		Scheme:                         defaultStr(tfutil.GetStringOrEmpty(jit, "scheme"), "PROVISION_SILENTLY"),
		UserStore:                      defaultStr(tfutil.GetStringOrEmpty(jit, "userstore"), "PRIMARY"),
		AssociateLocalUser:             tfutil.GetBool(jit, "associateLocalUser", false),
		SkipJITForLookupFailure:        tfutil.GetBool(jit, "skipJITForLookupFailure", false),
		AttributeSyncMethod:            defaultStr(tfutil.GetStringOrEmpty(jit, "attributeSyncMethod"), "OVERRIDE_ALL"),
		IdpGroupSyncMethod:             defaultStr(tfutil.GetStringOrEmpty(jit, "idpGroupSyncMethod"), "MERGE_WITH_EXISTING"),
		AccountLookupAttributeMappings: tfutil.ListFrom(ctx, lookupMappingTypes, lms, diags),
	}, diags)

	ia := tfutil.GetMap(idp, "implicitAssociation")
	m.ImplicitAssociation = tfutil.ObjectFrom(ctx, implicitTypes, implicitAssociationModel{
		Enabled:          tfutil.GetBool(ia, "isEnabled", false),
		LookupAttributes: tfutil.ListOfStrings(tfutil.GetStrings(ia, "lookupAttribute"), false),
	}, diags)

	oc := tfutil.GetMap(prov, "outboundConnectors")
	defConn, _ := oc["defaultConnectorId"].(string)
	m.DefaultOutboundConnector = types.StringValue(decodeName(defConn))
	priorConns := map[string]connectorModel{}
	if !m.OutboundProvisioningConnectors.IsNull() && !m.OutboundProvisioningConnectors.IsUnknown() {
		diags.Append(m.OutboundProvisioningConnectors.ElementsAs(ctx, &priorConns, false)...)
	}
	conns := map[string]attr.Value{}
	for _, c := range tfutil.GetObjects(oc, "connectors") {
		cid, _ := c["connectorId"].(string)
		name := decodeName(cid)
		var detail map[string]any
		if err := r.client.GetIdentityProviderSub(ctx, m.ID.ValueString(), "provisioning/outbound-connectors/"+cid, &detail); err != nil {
			tfutil.AddAPIError(diags, "read outbound connector "+name, err)
			return false
		}
		if n, _ := detail["name"].(string); n != "" {
			name = n
		}
		prior, known := priorConns[name]
		all := propertyMap(detail)
		plain, secret := map[string]string{}, map[string]string{}
		for k, v := range all {
			switch {
			case prior.SecretProperties.Elements()[k] != nil:
				secret[k] = priorSecretValue(prior.SecretProperties, k, v)
			case prior.Properties.Elements()[k] != nil || (!known && importing):
				plain[k] = v
			}
		}
		conns[name] = tfutil.ObjectFrom(ctx, connectorTypes, connectorModel{
			Enabled:          tfutil.GetBool(detail, "isEnabled", false),
			BlockingEnabled:  tfutil.GetBool(detail, "blockingEnabled", false),
			RulesEnabled:     tfutil.GetBool(detail, "rulesEnabled", false),
			Properties:       tfutil.MapValueOfStrings(plain, prior.Properties.IsNull() && (!importing || len(plain) == 0)),
			SecretProperties: tfutil.MapValueOfStrings(secret, prior.SecretProperties.IsNull()),
		}, diags)
	}
	if len(conns) == 0 && m.OutboundProvisioningConnectors.IsNull() {
		m.OutboundProvisioningConnectors = types.MapNull(types.ObjectType{AttrTypes: connectorTypes})
	} else {
		v, d := types.MapValue(types.ObjectType{AttrTypes: connectorTypes}, conns)
		diags.Append(d...)
		m.OutboundProvisioningConnectors = v
	}
	if m.ForceDelete.IsNull() {
		m.ForceDelete = types.BoolValue(false)
	}
	return !diags.HasError()
}

func defaultStr(v types.String, def string) types.String {
	if v.ValueString() == "" {
		return types.StringValue(def)
	}
	return v
}

// checkRoleMappings reports mappings the server dropped (it ignores local roles it cannot
// resolve instead of rejecting them).
func (r *idpResource) checkRoleMappings(ctx context.Context, id string, plan *idpModel, diags *diag.Diagnostics) {
	want, _ := tfutil.ElementsAs[roleMappingModel](ctx, plan.RoleMappings, diags)
	if len(want) == 0 {
		return
	}
	var got struct {
		Mappings []struct {
			IdpRole   string `json:"idpRole"`
			LocalRole string `json:"localRole"`
		} `json:"mappings"`
	}
	if err := r.client.GetIdentityProviderSub(ctx, id, "roles", &got); err != nil {
		tfutil.AddAPIError(diags, "read identity provider roles", err)
		return
	}
	have := map[string]bool{}
	for _, m := range got.Mappings {
		have[m.IdpRole+"\x00"+m.LocalRole] = true
	}
	for _, w := range want {
		if !have[w.IdpRole.ValueString()+"\x00"+w.LocalRole.ValueString()] {
			diags.AddAttributeError(path.Root("role_mappings"), "Role mapping not accepted",
				fmt.Sprintf("The server ignored the mapping %q -> %q. Check that the local role exists and is written as Internal/<role name>.", w.IdpRole.ValueString(), w.LocalRole.ValueString()))
		}
	}
}
