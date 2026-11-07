package application

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// attribute types, derived once from the schema
var (
	appAttrTypes      = func() map[string]attr.Type { return resourceSchema().Type().(types.ObjectType).AttrTypes }()
	assocRolesTypes   = tfutil.NestedAttrTypes(appAttrTypes, "associated_roles")
	claimConfigTypes  = tfutil.NestedAttrTypes(appAttrTypes, "claim_configuration")
	reqClaimTypes     = tfutil.NestedAttrTypes(claimConfigTypes, "requested_claims")
	claimMapTypes     = tfutil.NestedAttrTypes(claimConfigTypes, "claim_mappings")
	subjectTypes      = tfutil.NestedAttrTypes(claimConfigTypes, "subject")
	roleClaimTypes    = tfutil.NestedAttrTypes(claimConfigTypes, "role")
	roleMapTypes      = tfutil.NestedAttrTypes(roleClaimTypes, "mappings")
	authSeqTypes      = tfutil.NestedAttrTypes(appAttrTypes, "authentication_sequence")
	stepTypes         = tfutil.NestedAttrTypes(authSeqTypes, "steps")
	optionTypes       = tfutil.NestedAttrTypes(stepTypes, "options")
	advancedTypes     = tfutil.NestedAttrTypes(appAttrTypes, "advanced_configuration")
	discGroupTypes    = tfutil.NestedAttrTypes(advancedTypes, "discoverable_groups")
	certTypes         = tfutil.NestedAttrTypes(advancedTypes, "certificate")
	attestationTypes  = tfutil.NestedAttrTypes(advancedTypes, "attestation")
	trustedAppTypes   = tfutil.NestedAttrTypes(advancedTypes, "trusted_app")
	provisioningTypes = tfutil.NestedAttrTypes(appAttrTypes, "provisioning")
	inboundProvTypes  = tfutil.NestedAttrTypes(provisioningTypes, "inbound")
	outboundIdpTypes  = tfutil.NestedAttrTypes(provisioningTypes, "outbound_identity_providers")
	oidcTypes         = tfutil.NestedAttrTypes(appAttrTypes, "oidc")
	samlTypes         = tfutil.NestedAttrTypes(appAttrTypes, "saml")
	passiveSTSTypes   = tfutil.NestedAttrTypes(appAttrTypes, "passive_sts")
	wsTrustTypes      = tfutil.NestedAttrTypes(appAttrTypes, "ws_trust")
)

func flattenApplication(ctx context.Context, app *client.Application, m, prior *appModel, diags *diag.Diagnostics) {
	m.ID = types.StringValue(app.ID)
	m.Name = types.StringValue(app.Name)
	m.Description = tfutil.StringValueOrNull(tfutil.Deref(app.Description))
	m.ImageURL = tfutil.StringValueOrNull(tfutil.Deref(app.ImageURL))
	m.AccessURL = tfutil.StringValueOrNull(tfutil.Deref(app.AccessURL))
	m.LogoutReturnURL = tfutil.StringValueOrNull(tfutil.Deref(app.LogoutReturnURL))
	// Clearing the logout return URL makes the server store ".*".
	if m.LogoutReturnURL.ValueString() == ".*" && (prior == nil || prior.LogoutReturnURL.ValueString() != ".*") {
		m.LogoutReturnURL = types.StringNull()
	}
	m.TemplateID = types.StringValue(tfutil.Deref(app.TemplateID))
	m.TemplateVersion = types.StringValue(tfutil.Deref(app.TemplateVersion))
	m.Enabled = tfutil.BoolValueOr(app.ApplicationEnabled, true)
	m.IsManagementApp = tfutil.BoolValueOr(app.IsManagementApp, false)
	m.IsB2BSelfServiceApp = types.BoolValue(isB2BSelfService(app))
	m.EnhancedOrgAuthenticationEnabled = tfutil.BoolValueOr(app.EnhancedOrgAuthenticationEnabled, false)
	m.ApplicationVersion = types.StringValue(app.ApplicationVersion)
	m.ClientID = types.StringValue(app.ClientID)
	m.Issuer = types.StringValue(app.Issuer)
	m.Access = types.StringValue(app.Access)

	ar := associatedRolesModel{AllowedAudience: types.StringValue("APPLICATION"), RoleIDs: tfutil.SetOfStrings(nil, false)}
	if app.AssociatedRoles != nil {
		if app.AssociatedRoles.AllowedAudience != "" {
			ar.AllowedAudience = types.StringValue(app.AssociatedRoles.AllowedAudience)
		}
		ids := make([]string, 0, len(app.AssociatedRoles.Roles))
		for _, r := range app.AssociatedRoles.Roles {
			ids = append(ids, r.ID)
		}
		ar.RoleIDs = tfutil.SetOfStrings(ids, false)
	}
	m.AssociatedRoles = tfutil.ObjectFrom(ctx, assocRolesTypes, ar, diags)

	m.ClaimConfiguration = flattenClaimConfiguration(ctx, app.ClaimConfiguration, diags)
	m.AuthenticationSequence = flattenAuthSequence(ctx, app.AuthenticationSequence, m.AuthenticationSequence, diags)
	m.AdvancedConfiguration = flattenAdvanced(ctx, app.AdvancedConfigurations, m.AdvancedConfiguration, diags)
	m.Provisioning = flattenProvisioning(ctx, app.ProvisioningConfigurations, diags)
}

func isB2BSelfService(app *client.Application) bool {
	if app.IsB2BSelfServiceApp != nil {
		return *app.IsB2BSelfServiceApp
	}
	if app.AdvancedConfigurations != nil {
		for _, p := range app.AdvancedConfigurations.AdditionalSpProperties {
			if p.Name == "isB2BSelfServiceApp" {
				return p.Value == "true"
			}
		}
	}
	return false
}

func flattenClaimConfiguration(ctx context.Context, cc *client.ClaimConfiguration, diags *diag.Diagnostics) types.Object {
	if cc == nil {
		cc = &client.ClaimConfiguration{Dialect: "LOCAL"}
	}
	out := claimConfigModel{Dialect: types.StringValue(cc.Dialect)}
	rcs := make([]requestedClaimModel, 0, len(cc.RequestedClaims))
	for _, rc := range cc.RequestedClaims {
		rcs = append(rcs, requestedClaimModel{URI: types.StringValue(rc.Claim.URI), Mandatory: types.BoolValue(rc.Mandatory)})
	}
	out.RequestedClaims = tfutil.SetFrom(ctx, reqClaimTypes, rcs, diags)
	cms := make([]claimMappingModel, 0, len(cc.ClaimMappings))
	for _, cm := range cc.ClaimMappings {
		cms = append(cms, claimMappingModel{ApplicationClaim: types.StringValue(cm.ApplicationClaim), LocalClaim: types.StringValue(cm.LocalClaim.URI)})
	}
	out.ClaimMappings = tfutil.SetFrom(ctx, claimMapTypes, cms, diags)

	sub := subjectModel{
		Claim:                       types.StringValue(""),
		IncludeUserDomain:           types.BoolValue(false),
		IncludeTenantDomain:         types.BoolValue(false),
		UseMappedLocalSubject:       types.BoolValue(false),
		MappedLocalSubjectMandatory: types.BoolValue(false),
	}
	if s := cc.Subject; s != nil {
		if s.Claim != nil {
			sub.Claim = types.StringValue(s.Claim.URI)
		}
		sub.IncludeUserDomain = tfutil.BoolValueOr(s.IncludeUserDomain, false)
		sub.IncludeTenantDomain = tfutil.BoolValueOr(s.IncludeTenantDomain, false)
		sub.UseMappedLocalSubject = tfutil.BoolValueOr(s.UseMappedLocalSubject, false)
		sub.MappedLocalSubjectMandatory = tfutil.BoolValueOr(s.MappedLocalSubjectMandatory, false)
	}
	out.Subject = tfutil.ObjectFrom(ctx, subjectTypes, sub, diags)

	role := roleClaimModel{Claim: types.StringValue(""), IncludeUserDomain: types.BoolValue(false)}
	rms := []roleMappingModel{}
	if r := cc.Role; r != nil {
		if r.Claim != nil {
			role.Claim = types.StringValue(r.Claim.URI)
		}
		role.IncludeUserDomain = tfutil.BoolValueOr(r.IncludeUserDomain, false)
		for _, rm := range r.Mappings {
			rms = append(rms, roleMappingModel{LocalRole: types.StringValue(rm.LocalRole), ApplicationRole: types.StringValue(rm.ApplicationRole)})
		}
	}
	role.Mappings = tfutil.SetFrom(ctx, roleMapTypes, rms, diags)
	out.Role = tfutil.ObjectFrom(ctx, roleClaimTypes, role, diags)
	return tfutil.ObjectFrom(ctx, claimConfigTypes, out, diags)
}

func flattenAuthSequence(ctx context.Context, as *client.AuthenticationSequence, prior types.Object, diags *diag.Diagnostics) types.Object {
	if as == nil {
		as = &client.AuthenticationSequence{Type: "DEFAULT"}
	}
	out := authSeqModel{
		Type:            types.StringValue(as.Type),
		Script:          types.StringValue(tfutil.Deref(as.Script)),
		SubjectStepID:   tfutil.Int64Value(as.SubjectStepID),
		AttributeStepID: tfutil.Int64Value(as.AttributeStepID),
	}
	if out.SubjectStepID.IsNull() {
		out.SubjectStepID = types.Int64Value(1)
	}
	if out.AttributeStepID.IsNull() {
		out.AttributeStepID = types.Int64Value(1)
	}
	// The server normalizes line endings and trailing whitespace of scripts; keep the
	// configured text when it is equivalent.
	if p, ok := tfutil.ObjectAs[authSeqModel](ctx, prior, diags); ok && tfutil.Known(p.Script) && scriptsEqual(p.Script.ValueString(), out.Script.ValueString()) {
		out.Script = p.Script
	}
	steps := append([]client.AuthenticationStep(nil), as.Steps...)
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].ID < steps[j].ID })
	stepModels := make([]stepModel, 0, len(steps))
	for _, st := range steps {
		opts := make([]optionModel, 0, len(st.Options))
		for _, o := range st.Options {
			opts = append(opts, optionModel{IDP: types.StringValue(o.IDP), Authenticator: types.StringValue(o.Authenticator)})
		}
		stepModels = append(stepModels, stepModel{Options: tfutil.SetFrom(ctx, optionTypes, opts, diags)})
	}
	out.Steps = tfutil.ListFrom(ctx, stepTypes, stepModels, diags)
	out.RequestPathAuthenticators = tfutil.ListOfStrings(as.RequestPathAuthenticators, false)
	return tfutil.ObjectFrom(ctx, authSeqTypes, out, diags)
}

func flattenAdvanced(ctx context.Context, a *client.AdvancedAppConfiguration, prior types.Object, diags *diag.Diagnostics) types.Object {
	if a == nil {
		a = &client.AdvancedAppConfiguration{}
	}
	out := advancedModel{
		SaaS:                         tfutil.BoolValueOr(a.SaaS, false),
		DiscoverableByEndUsers:       tfutil.BoolValueOr(a.DiscoverableByEndUsers, false),
		SkipLoginConsent:             tfutil.BoolValueOr(a.SkipLoginConsent, false),
		SkipLogoutConsent:            tfutil.BoolValueOr(a.SkipLogoutConsent, false),
		UseExternalConsentPage:       tfutil.BoolValueOr(a.UseExternalConsentPage, false),
		ReturnAuthenticatedIdpList:   tfutil.BoolValueOr(a.ReturnAuthenticatedIdpList, false),
		EnableAuthorization:          tfutil.BoolValueOr(a.EnableAuthorization, false),
		EnableAPIBasedAuthentication: tfutil.BoolValueOr(a.EnableAPIBasedAuthentication, false),
		Fragment:                     tfutil.BoolValueOr(a.Fragment, false),
	}
	groups := make([]discoverableGroupModel, 0, len(a.DiscoverableGroups))
	for _, g := range a.DiscoverableGroups {
		ids := make([]string, 0, len(g.Groups))
		for _, gr := range g.Groups {
			ids = append(ids, gr.ID)
		}
		groups = append(groups, discoverableGroupModel{UserStore: types.StringValue(g.UserStore), GroupIDs: tfutil.SetOfStrings(ids, false)})
	}
	out.DiscoverableGroups = tfutil.SetFrom(ctx, discGroupTypes, groups, diags)

	cert := certificateModel{Type: types.StringNull(), Value: types.StringNull()}
	if a.Certificate != nil && (a.Certificate.Type != "" || a.Certificate.Value != "") {
		cert = certificateModel{Type: types.StringValue(a.Certificate.Type), Value: types.StringValue(a.Certificate.Value)}
	}
	out.Certificate = tfutil.ObjectFrom(ctx, certTypes, cert, diags)

	att := attestationModel{Enabled: types.BoolValue(false), AndroidPackageName: types.StringValue(""), AppleAppID: types.StringValue(""), AndroidAttestationServiceCredentials: types.StringNull()}
	if md := a.AttestationMetaData; md != nil {
		att.Enabled = tfutil.BoolValueOr(md.EnableClientAttestation, false)
		att.AndroidPackageName = types.StringValue(tfutil.Deref(md.AndroidPackageName))
		att.AppleAppID = types.StringValue(tfutil.Deref(md.AppleAppID))
	}
	var priorAdv *advancedModel
	if p, ok := tfutil.ObjectAs[advancedModel](ctx, prior, diags); ok {
		priorAdv = p
		if pa, ok := tfutil.ObjectAs[attestationModel](ctx, p.Attestation, diags); ok && !pa.AndroidAttestationServiceCredentials.IsUnknown() {
			// Write-only on the server.
			att.AndroidAttestationServiceCredentials = pa.AndroidAttestationServiceCredentials
		}
	}
	_ = priorAdv
	out.Attestation = tfutil.ObjectFrom(ctx, attestationTypes, att, diags)

	ta := trustedAppModel{IsFIDOTrustedApp: types.BoolValue(false), IsConsentGranted: types.BoolValue(false), AndroidPackageName: types.StringValue(""), AndroidThumbprints: tfutil.SetOfStrings(nil, false), AppleAppID: types.StringValue("")}
	if t := a.TrustedAppConfiguration; t != nil {
		ta = trustedAppModel{
			IsFIDOTrustedApp:   tfutil.BoolValueOr(t.IsFIDOTrustedApp, false),
			IsConsentGranted:   tfutil.BoolValueOr(t.IsConsentGranted, false),
			AndroidPackageName: types.StringValue(tfutil.Deref(t.AndroidPackageName)),
			AndroidThumbprints: tfutil.SetOfStrings(t.AndroidThumbprints, false),
			AppleAppID:         types.StringValue(tfutil.Deref(t.AppleAppID)),
		}
	}
	out.TrustedApp = tfutil.ObjectFrom(ctx, trustedAppTypes, ta, diags)
	return tfutil.ObjectFrom(ctx, advancedTypes, out, diags)
}

func flattenProvisioning(ctx context.Context, p *client.AppProvisioning, diags *diag.Diagnostics) types.Object {
	if p == nil {
		p = &client.AppProvisioning{}
	}
	in := inboundProvisioningModel{ProxyMode: types.BoolValue(false), ProvisioningUserstoreDomain: types.StringValue("")}
	if p.InboundProvisioning != nil {
		in.ProxyMode = tfutil.BoolValueOr(p.InboundProvisioning.ProxyMode, false)
		in.ProvisioningUserstoreDomain = types.StringValue(tfutil.Deref(p.InboundProvisioning.ProvisioningUserstoreDomain))
	}
	idps := make([]outboundIdpModel, 0, len(p.OutboundProvisioningIdps))
	for _, i := range p.OutboundProvisioningIdps {
		idps = append(idps, outboundIdpModel{
			IDP: types.StringValue(i.IDP), Connector: types.StringValue(i.Connector),
			Blocking: types.BoolValue(i.Blocking), Rules: types.BoolValue(i.Rules), JIT: types.BoolValue(i.JIT),
		})
	}
	out := provisioningModel{
		Inbound:                   tfutil.ObjectFrom(ctx, inboundProvTypes, in, diags),
		OutboundIdentityProviders: tfutil.ListFrom(ctx, outboundIdpTypes, idps, diags),
	}
	return tfutil.ObjectFrom(ctx, provisioningTypes, out, diags)
}

// flattenOIDC converts the server's OIDC configuration. prior supplies values the
// server does not return (or returns in another form).
func flattenOIDC(ctx context.Context, o map[string]any, prior *oidcModel, diags *diag.Diagnostics) types.Object {
	str := tfutil.GetStringOrEmpty
	out := oidcModel{
		ClientID:                       str(o, "clientId"),
		ClientSecret:                   str(o, "clientSecret"),
		State:                          str(o, "state"),
		GrantTypes:                     tfutil.SetOfStrings(tfutil.GetStrings(o, "grantTypes"), false),
		CallbackURLs:                   tfutil.ListOfStrings(tfutil.GetStrings(o, "callbackURLs"), false),
		AllowedOrigins:                 tfutil.SetOfStrings(tfutil.GetStrings(o, "allowedOrigins"), false),
		PublicClient:                   tfutil.GetBool(o, "publicClient", false),
		ValidateRequestObjectSignature: tfutil.GetBool(o, "validateRequestObjectSignature", false),
		ScopeValidators:                tfutil.SetOfStrings(tfutil.GetStrings(o, "scopeValidators"), false),
		IsFAPIApplication:              tfutil.GetBool(o, "isFAPIApplication", false),
		TokenIssuerOrganizationID:      tfutil.GetStringOrEmpty(tfutil.GetMap(o, "issuer"), "organizationId"),
	}
	if prior != nil && out.ClientSecret.ValueString() == "" && tfutil.Known(prior.ClientSecret) && !out.PublicClient.ValueBool() {
		// Servers with hashed secrets ([oauth] hash_tokens_and_secrets) do not return it.
		out.ClientSecret = prior.ClientSecret
	}

	pk := tfutil.GetMap(o, "pkce")
	out.PKCE = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "pkce"), pkceModel{
		Mandatory:                      tfutil.GetBool(pk, "mandatory", false),
		SupportPlainTransformAlgorithm: tfutil.GetBool(pk, "supportPlainTransformAlgorithm", false),
	}, diags)

	at := tfutil.GetMap(o, "accessToken")
	out.AccessToken = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "access_token"), accessTokenModel{
		Type:                                  str(at, "type"),
		UserAccessTokenExpiryInSeconds:        zeroIfNull(tfutil.GetInt64(at, "userAccessTokenExpiryInSeconds")),
		ApplicationAccessTokenExpiryInSeconds: zeroIfNull(tfutil.GetInt64(at, "applicationAccessTokenExpiryInSeconds")),
		BindingType:                           str(at, "bindingType"),
		RevokeTokensWhenIDPSessionTerminated:  tfutil.GetBool(at, "revokeTokensWhenIDPSessionTerminated", false),
		ValidateTokenBinding:                  tfutil.GetBool(at, "validateTokenBinding", false),
		AccessTokenAttributes:                 tfutil.SetOfStrings(tfutil.GetStrings(at, "accessTokenAttributes"), false),
		EnableJWTScopeAsArray:                 tfutil.GetBool(at, "enableJwtScopeAsArray", false),
	}, diags)
	if prior != nil {
		// bindingType is omitted by the server when "None"; keep the configured spelling.
		if p, ok := tfutil.ObjectAs[accessTokenModel](ctx, prior.AccessToken, diags); ok && tfutil.Known(p.BindingType) {
			if cur, ok := tfutil.ObjectAs[accessTokenModel](ctx, out.AccessToken, diags); ok && cur.BindingType.ValueString() == "" && (p.BindingType.ValueString() == "None" || p.BindingType.ValueString() == "") {
				cur.BindingType = p.BindingType
				out.AccessToken = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "access_token"), cur, diags)
			}
		}
	}

	rt := tfutil.GetMap(o, "refreshToken")
	out.RefreshToken = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "refresh_token"), refreshTokenModel{
		ExpiryInSeconds:                     zeroIfNull(tfutil.GetInt64(rt, "expiryInSeconds")),
		RenewRefreshToken:                   tfutil.GetBool(rt, "renewRefreshToken", false),
		ExtendRenewedRefreshTokenExpiryTime: tfutil.GetBool(rt, "extendRenewedRefreshTokenExpiryTime", false),
	}, diags)

	idt := tfutil.GetMap(o, "idToken")
	enc := tfutil.GetMap(idt, "encryption")
	idTypes := tfutil.NestedAttrTypes(oidcTypes, "id_token")
	out.IDToken = tfutil.ObjectFrom(ctx, idTypes, idTokenModel{
		ExpiryInSeconds:   zeroIfNull(tfutil.GetInt64(idt, "expiryInSeconds")),
		Audience:          tfutil.SetOfStrings(tfutil.GetStrings(idt, "audience"), false),
		SignedResponseAlg: str(idt, "idTokenSignedResponseAlg"),
		Encryption: tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(idTypes, "encryption"), encryptionModel{
			Enabled:   tfutil.GetBool(enc, "enabled", false),
			Algorithm: str(enc, "algorithm"),
			Method:    str(enc, "method"),
		}, diags),
	}, diags)

	lo := tfutil.GetMap(o, "logout")
	out.Logout = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "logout"), logoutModel{
		BackChannelLogoutURL:  str(lo, "backChannelLogoutUrl"),
		FrontChannelLogoutURL: str(lo, "frontChannelLogoutUrl"),
	}, diags)

	hf := tfutil.GetMap(o, "hybridFlow")
	out.HybridFlow = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "hybrid_flow"), hybridFlowModel{
		Enabled: tfutil.GetBool(hf, "enable", false),
		// After an update the server stores an absent response type as the string "null".
		ResponseType: nullStringAsEmpty(str(hf, "responseType")),
	}, diags)

	st := tfutil.GetMap(o, "subjectToken")
	out.SubjectToken = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "subject_token"), subjectTokenModel{
		Enabled:                                tfutil.GetBool(st, "enable", false),
		ApplicationSubjectTokenExpiryInSeconds: zeroIfNull(tfutil.GetInt64(st, "applicationSubjectTokenExpiryInSeconds")),
	}, diags)

	ca := tfutil.GetMap(o, "clientAuthentication")
	out.ClientAuthentication = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "client_authentication"), clientAuthModel{
		TokenEndpointAuthMethod:          str(ca, "tokenEndpointAuthMethod"),
		TokenEndpointAuthSigningAlg:      str(ca, "tokenEndpointAuthSigningAlg"),
		TokenEndpointAllowReusePvtKeyJWT: tfutil.GetBool(ca, "tokenEndpointAllowReusePvtKeyJwt", false),
		TLSClientAuthSubjectDN:           str(ca, "tlsClientAuthSubjectDn"),
	}, diags)

	ro := tfutil.GetMap(o, "requestObject")
	roe := tfutil.GetMap(ro, "encryption")
	out.RequestObject = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "request_object"), requestObjectModel{
		SigningAlg:          str(ro, "requestObjectSigningAlg"),
		EncryptionAlgorithm: str(roe, "algorithm"),
		EncryptionMethod:    str(roe, "method"),
	}, diags)

	out.RequirePushedAuthorizationRequests = tfutil.GetBool(tfutil.GetMap(o, "pushAuthorizationRequest"), "requirePushAuthorizationRequest", false)

	su := tfutil.GetMap(o, "subject")
	out.Subject = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "subject"), oidcSubjectModel{
		SubjectType:         str(su, "subjectType"),
		SectorIdentifierURI: str(su, "sectorIdentifierUri"),
	}, diags)

	ci := tfutil.GetMap(o, "cibaAuthenticationRequest")
	out.CIBA = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(oidcTypes, "ciba"), cibaModel{
		AuthRequestExpiryTime: zeroIfNull(tfutil.GetInt64(ci, "authReqExpiryTime")),
		NotificationChannels:  tfutil.SetOfStrings(tfutil.GetStrings(ci, "notificationChannels"), false),
		SkipUserValidation:    tfutil.GetBool(ci, "skipUserValidation", false),
		AllowFederatedUsers:   tfutil.GetBool(ci, "allowFederatedUsers", false),
	}, diags)

	return tfutil.ObjectFrom(ctx, oidcTypes, out, diags)
}

func zeroIfNull(v types.Int64) types.Int64 {
	if v.IsNull() {
		return types.Int64Value(0)
	}
	return v
}

func flattenSAML(ctx context.Context, s map[string]any, prior *samlModel, diags *diag.Diagnostics) types.Object {
	str := tfutil.GetStringOrEmpty
	out := samlModel{
		MetadataURL:                 types.StringNull(),
		Issuer:                      str(s, "issuer"),
		ServiceProviderQualifier:    str(s, "serviceProviderQualifier"),
		AssertionConsumerURLs:       tfutil.ListOfStrings(tfutil.GetStrings(s, "assertionConsumerUrls"), false),
		DefaultAssertionConsumerURL: str(s, "defaultAssertionConsumerUrl"),
		IdpEntityIDAlias:            str(s, "idpEntityIdAlias"),
		EnableAssertionQueryProfile: tfutil.GetBool(s, "enableAssertionQueryProfile", false),
	}
	if prior != nil {
		out.MetadataURL = prior.MetadataURL
	}

	sso := tfutil.GetMap(s, "singleSignOnProfile")
	as := tfutil.GetMap(sso, "assertion")
	ae := tfutil.GetMap(as, "encryption")
	ssoTypes := tfutil.NestedAttrTypes(samlTypes, "single_sign_on_profile")
	assertTypes := tfutil.NestedAttrTypes(ssoTypes, "assertion")
	nameID := str(as, "nameIdFormat")
	if prior != nil {
		if pv := priorNameIDFormat(ctx, prior, diags); tfutil.Known(pv) && normalizeNameIDFormat(pv.ValueString()) == normalizeNameIDFormat(nameID.ValueString()) {
			nameID = pv
		}
	}
	if !nameID.IsNull() {
		// Show the standard URN form when the configuration does not say otherwise.
		if prior == nil || !tfutil.Known(priorNameIDFormat(ctx, prior, diags)) {
			nameID = types.StringValue(normalizeNameIDFormat(nameID.ValueString()))
		}
	}
	out.SingleSignOnProfile = tfutil.ObjectFrom(ctx, ssoTypes, ssoProfileModel{
		Bindings: tfutil.SetOfStrings(tfutil.GetStrings(sso, "bindings"), false),
		EnableSignatureValidationForArtifactBinding: tfutil.GetBool(sso, "enableSignatureValidationForArtifactBinding", false),
		EnableIdpInitiatedSingleSignOn:              tfutil.GetBool(sso, "enableIdpInitiatedSingleSignOn", false),
		AttributeConsumingServiceIndex:              str(sso, "attributeConsumingServiceIndex"),
		Assertion: tfutil.ObjectFrom(ctx, assertTypes, assertionModel{
			NameIDFormat:    nameID,
			Audiences:       tfutil.SetOfStrings(tfutil.GetStrings(as, "audiences"), false),
			Recipients:      tfutil.SetOfStrings(tfutil.GetStrings(as, "recipients"), false),
			DigestAlgorithm: str(as, "digestAlgorithm"),
			Encryption: tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(assertTypes, "encryption"), assertionEncryptionModel{
				Enabled:                      tfutil.GetBool(ae, "enabled", false),
				AssertionEncryptionAlgorithm: str(ae, "assertionEncryptionAlgorithm"),
				KeyEncryptionAlgorithm:       str(ae, "keyEncryptionAlgorithm"),
			}, diags),
		}, diags),
	}, diags)

	ap := tfutil.GetMap(s, "attributeProfile")
	out.AttributeProfile = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(samlTypes, "attribute_profile"), attributeProfileModel{
		Enabled:                           tfutil.GetBool(ap, "enabled", false),
		AlwaysIncludeAttributesInResponse: tfutil.GetBool(ap, "alwaysIncludeAttributesInResponse", false),
		NameFormat:                        str(ap, "nameFormat"),
	}, diags)

	slo := tfutil.GetMap(s, "singleLogoutProfile")
	idpSLO := tfutil.GetMap(slo, "idpInitiatedSingleLogout")
	sloTypes := tfutil.NestedAttrTypes(samlTypes, "single_logout_profile")
	out.SingleLogoutProfile = tfutil.ObjectFrom(ctx, sloTypes, sloProfileModel{
		Enabled:           tfutil.GetBool(slo, "enabled", false),
		LogoutRequestURL:  str(slo, "logoutRequestUrl"),
		LogoutResponseURL: str(slo, "logoutResponseUrl"),
		LogoutMethod:      str(slo, "logoutMethod"),
		IdpInitiatedSingleLogout: tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(sloTypes, "idp_initiated_single_logout"), idpInitiatedSLOModel{
			Enabled:      tfutil.GetBool(idpSLO, "enabled", false),
			ReturnToURLs: tfutil.SetOfStrings(tfutil.GetStrings(idpSLO, "returnToUrls"), false),
		}, diags),
	}, diags)

	rv := tfutil.GetMap(s, "requestValidation")
	out.RequestValidation = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(samlTypes, "request_validation"), requestValidationModel{
		EnableSignatureValidation:    tfutil.GetBool(rv, "enableSignatureValidation", false),
		SignatureValidationCertAlias: str(rv, "signatureValidationCertAlias"),
	}, diags)

	rs := tfutil.GetMap(s, "responseSigning")
	out.ResponseSigning = tfutil.ObjectFrom(ctx, tfutil.NestedAttrTypes(samlTypes, "response_signing"), responseSigningModel{
		Enabled:          tfutil.GetBool(rs, "enabled", false),
		SigningAlgorithm: str(rs, "signingAlgorithm"),
	}, diags)

	return tfutil.ObjectFrom(ctx, samlTypes, out, diags)
}

func priorNameIDFormat(ctx context.Context, prior *samlModel, diags *diag.Diagnostics) types.String {
	sso, ok := tfutil.ObjectAs[ssoProfileModel](ctx, prior.SingleSignOnProfile, diags)
	if !ok {
		return types.StringUnknown()
	}
	as, ok := tfutil.ObjectAs[assertionModel](ctx, sso.Assertion, diags)
	if !ok {
		return types.StringUnknown()
	}
	return as.NameIDFormat
}

func flattenPassiveSTS(ctx context.Context, p *client.PassiveSTSConfig, diags *diag.Diagnostics) types.Object {
	return tfutil.ObjectFrom(ctx, passiveSTSTypes, passiveSTSModel{
		Realm:         types.StringValue(p.Realm),
		ReplyTo:       types.StringValue(p.ReplyTo),
		ReplyToLogout: types.StringValue(p.ReplyToLogout),
	}, diags)
}

func flattenWSTrust(ctx context.Context, w *client.WSTrustConfig, diags *diag.Diagnostics) types.Object {
	return tfutil.ObjectFrom(ctx, wsTrustTypes, wsTrustModel{
		Audience:         types.StringValue(w.Audience),
		CertificateAlias: types.StringValue(w.CertificateAlias),
	}, diags)
}

func scriptsEqual(a, b string) bool {
	return normalizeScript(a) == normalizeScript(b)
}

func nullStringAsEmpty(v types.String) types.String {
	if v.ValueString() == "null" {
		return types.StringValue("")
	}
	return v
}
