package application

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/client"
	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

// expandApplication builds the core application body (everything except inbound
// protocols) from the plan. Unknown values (left to the server) are not sent.
func expandApplication(ctx context.Context, m *appModel, diags *diag.Diagnostics) *client.Application {
	app := &client.Application{
		Name:                             m.Name.ValueString(),
		Description:                      emptyIfNull(m.Description),
		ImageURL:                         emptyIfNull(m.ImageURL),
		AccessURL:                        emptyIfNull(m.AccessURL),
		LogoutReturnURL:                  emptyIfNull(m.LogoutReturnURL),
		TemplateID:                       tfutil.StringPtr(m.TemplateID),
		TemplateVersion:                  tfutil.StringPtr(m.TemplateVersion),
		ApplicationEnabled:               tfutil.BoolPtr(m.Enabled),
		IsManagementApp:                  tfutil.BoolPtr(m.IsManagementApp),
		IsB2BSelfServiceApp:              tfutil.BoolPtr(m.IsB2BSelfServiceApp),
		EnhancedOrgAuthenticationEnabled: tfutil.BoolPtr(m.EnhancedOrgAuthenticationEnabled),
	}

	if ar, ok := tfutil.ObjectAs[associatedRolesModel](ctx, m.AssociatedRoles, diags); ok {
		if tfutil.Known(ar.AllowedAudience) || tfutil.Known(ar.RoleIDs) {
			cfg := &client.AssociatedRolesConfig{AllowedAudience: ar.AllowedAudience.ValueString(), Roles: []client.RoleRef{}}
			if !tfutil.Known(ar.AllowedAudience) {
				cfg.AllowedAudience = "APPLICATION"
			}
			for _, id := range tfutil.Strings(ctx, ar.RoleIDs, diags) {
				cfg.Roles = append(cfg.Roles, client.RoleRef{ID: id})
			}
			app.AssociatedRoles = cfg
		}
	}

	app.ClaimConfiguration = expandClaimConfiguration(ctx, m.ClaimConfiguration, diags)
	app.AuthenticationSequence = expandAuthSequence(ctx, m.AuthenticationSequence, diags)
	app.AdvancedConfigurations = expandAdvanced(ctx, m.AdvancedConfiguration, diags)
	app.ProvisioningConfigurations = expandProvisioning(ctx, m.Provisioning, diags)
	return app
}

func emptyIfNull(v types.String) *string {
	if v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func expandClaimConfiguration(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.ClaimConfiguration {
	cc, ok := tfutil.ObjectAs[claimConfigModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	out := &client.ClaimConfiguration{Dialect: cc.Dialect.ValueString()}
	if rcs, ok := tfutil.ElementsAs[requestedClaimModel](ctx, cc.RequestedClaims, diags); ok {
		out.RequestedClaims = []client.AppRequestedClaim{}
		for _, rc := range rcs {
			out.RequestedClaims = append(out.RequestedClaims, client.AppRequestedClaim{
				Claim: client.ClaimRef{URI: rc.URI.ValueString()}, Mandatory: rc.Mandatory.ValueBool(),
			})
		}
	}
	// With the LOCAL dialect the server derives the mappings from the requested claims;
	// only send mappings for CUSTOM (or when the user set them explicitly).
	if cms, ok := tfutil.ElementsAs[claimMappingModel](ctx, cc.ClaimMappings, diags); ok && (out.Dialect == "CUSTOM" || !tfutil.Known(cc.Dialect)) {
		out.ClaimMappings = []client.AppClaimMapping{}
		for _, cm := range cms {
			out.ClaimMappings = append(out.ClaimMappings, client.AppClaimMapping{
				ApplicationClaim: cm.ApplicationClaim.ValueString(), LocalClaim: client.ClaimRef{URI: cm.LocalClaim.ValueString()},
			})
		}
	}
	if s, ok := tfutil.ObjectAs[subjectModel](ctx, cc.Subject, diags); ok {
		sub := &client.AppSubjectConfig{
			IncludeUserDomain:           tfutil.BoolPtr(s.IncludeUserDomain),
			IncludeTenantDomain:         tfutil.BoolPtr(s.IncludeTenantDomain),
			UseMappedLocalSubject:       tfutil.BoolPtr(s.UseMappedLocalSubject),
			MappedLocalSubjectMandatory: tfutil.BoolPtr(s.MappedLocalSubjectMandatory),
		}
		if tfutil.Known(s.Claim) {
			sub.Claim = &client.ClaimRef{URI: s.Claim.ValueString()}
		}
		out.Subject = sub
	}
	if r, ok := tfutil.ObjectAs[roleClaimModel](ctx, cc.Role, diags); ok {
		role := &client.AppRoleClaimConfig{IncludeUserDomain: tfutil.BoolPtr(r.IncludeUserDomain)}
		if tfutil.Known(r.Claim) {
			role.Claim = &client.ClaimRef{URI: r.Claim.ValueString()}
		}
		if rms, ok := tfutil.ElementsAs[roleMappingModel](ctx, r.Mappings, diags); ok {
			role.Mappings = []client.AppRoleMapping{}
			for _, rm := range rms {
				role.Mappings = append(role.Mappings, client.AppRoleMapping{LocalRole: rm.LocalRole.ValueString(), ApplicationRole: rm.ApplicationRole.ValueString()})
			}
		}
		out.Role = role
	}
	if out.Dialect == "" {
		out.Dialect = "LOCAL"
		if len(out.ClaimMappings) > 0 {
			out.Dialect = "CUSTOM"
		}
	}
	return out
}

func expandAuthSequence(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.AuthenticationSequence {
	as, ok := tfutil.ObjectAs[authSeqModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	out := &client.AuthenticationSequence{
		Type:            as.Type.ValueString(),
		Script:          tfutil.StringPtrNonEmpty(as.Script),
		SubjectStepID:   tfutil.Int64Ptr(as.SubjectStepID),
		AttributeStepID: tfutil.Int64Ptr(as.AttributeStepID),
	}
	if steps, ok := tfutil.ElementsAs[stepModel](ctx, as.Steps, diags); ok {
		out.Steps = []client.AuthenticationStep{}
		for i, st := range steps {
			step := client.AuthenticationStep{ID: int64(i + 1), Options: []client.AuthenticatorRef{}}
			opts, _ := tfutil.ElementsAs[optionModel](ctx, st.Options, diags)
			for _, o := range opts {
				step.Options = append(step.Options, client.AuthenticatorRef{IDP: o.IDP.ValueString(), Authenticator: o.Authenticator.ValueString()})
			}
			out.Steps = append(out.Steps, step)
		}
	}
	if tfutil.Known(as.RequestPathAuthenticators) {
		out.RequestPathAuthenticators = tfutil.Strings(ctx, as.RequestPathAuthenticators, diags)
		if out.RequestPathAuthenticators == nil {
			out.RequestPathAuthenticators = []string{}
		}
	}
	if out.Type == "" {
		if len(out.Steps) > 0 || (out.Script != nil && *out.Script != "") {
			out.Type = "USER_DEFINED"
		} else {
			out.Type = "DEFAULT"
		}
	}
	return out
}

func expandAdvanced(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.AdvancedAppConfiguration {
	a, ok := tfutil.ObjectAs[advancedModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	out := &client.AdvancedAppConfiguration{
		SaaS:                         tfutil.BoolPtr(a.SaaS),
		DiscoverableByEndUsers:       tfutil.BoolPtr(a.DiscoverableByEndUsers),
		SkipLoginConsent:             tfutil.BoolPtr(a.SkipLoginConsent),
		SkipLogoutConsent:            tfutil.BoolPtr(a.SkipLogoutConsent),
		UseExternalConsentPage:       tfutil.BoolPtr(a.UseExternalConsentPage),
		ReturnAuthenticatedIdpList:   tfutil.BoolPtr(a.ReturnAuthenticatedIdpList),
		EnableAuthorization:          tfutil.BoolPtr(a.EnableAuthorization),
		EnableAPIBasedAuthentication: tfutil.BoolPtr(a.EnableAPIBasedAuthentication),
	}
	if groups, ok := tfutil.ElementsAs[discoverableGroupModel](ctx, a.DiscoverableGroups, diags); ok {
		out.DiscoverableGroups = []client.DiscoverableGroup{}
		for _, g := range groups {
			dg := client.DiscoverableGroup{UserStore: g.UserStore.ValueString(), Groups: []client.GroupBasicInfo{}}
			for _, id := range tfutil.Strings(ctx, g.GroupIDs, diags) {
				dg.Groups = append(dg.Groups, client.GroupBasicInfo{ID: id})
			}
			out.DiscoverableGroups = append(out.DiscoverableGroups, dg)
		}
	}
	if c, ok := tfutil.ObjectAs[certificateModel](ctx, a.Certificate, diags); ok && (tfutil.Known(c.Type) || tfutil.Known(c.Value)) {
		out.Certificate = &client.AppCertificate{Type: c.Type.ValueString(), Value: c.Value.ValueString()}
	}
	if at, ok := tfutil.ObjectAs[attestationModel](ctx, a.Attestation, diags); ok {
		md := &client.AttestationMetaData{
			EnableClientAttestation: tfutil.BoolPtr(at.Enabled),
			AndroidPackageName:      tfutil.StringPtr(at.AndroidPackageName),
			AppleAppID:              tfutil.StringPtr(at.AppleAppID),
		}
		if tfutil.Known(at.AndroidAttestationServiceCredentials) && at.AndroidAttestationServiceCredentials.ValueString() != "" {
			md.AndroidAttestationServiceCredentials = []byte(at.AndroidAttestationServiceCredentials.ValueString())
		}
		out.AttestationMetaData = md
	}
	if t, ok := tfutil.ObjectAs[trustedAppModel](ctx, a.TrustedApp, diags); ok {
		ta := &client.TrustedAppConfiguration{
			IsFIDOTrustedApp:   tfutil.BoolPtr(t.IsFIDOTrustedApp),
			IsConsentGranted:   tfutil.BoolPtr(t.IsConsentGranted),
			AndroidPackageName: tfutil.StringPtr(t.AndroidPackageName),
			AppleAppID:         tfutil.StringPtr(t.AppleAppID),
		}
		if tfutil.Known(t.AndroidThumbprints) {
			ta.AndroidThumbprints = tfutil.Strings(ctx, t.AndroidThumbprints, diags)
		}
		out.TrustedAppConfiguration = ta
	}
	return out
}

func expandProvisioning(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *client.AppProvisioning {
	p, ok := tfutil.ObjectAs[provisioningModel](ctx, obj, diags)
	if !ok {
		return nil
	}
	out := &client.AppProvisioning{}
	if in, ok := tfutil.ObjectAs[inboundProvisioningModel](ctx, p.Inbound, diags); ok {
		out.InboundProvisioning = &client.InboundSCIMProvisioning{
			ProxyMode:                   tfutil.BoolPtr(in.ProxyMode),
			ProvisioningUserstoreDomain: tfutil.StringPtr(in.ProvisioningUserstoreDomain),
		}
	}
	if idps, ok := tfutil.ElementsAs[outboundIdpModel](ctx, p.OutboundIdentityProviders, diags); ok {
		out.OutboundProvisioningIdps = []client.OutboundProvisioningIdp{}
		for _, i := range idps {
			out.OutboundProvisioningIdps = append(out.OutboundProvisioningIdps, client.OutboundProvisioningIdp{
				IDP: i.IDP.ValueString(), Connector: i.Connector.ValueString(),
				Blocking: i.Blocking.ValueBool(), Rules: i.Rules.ValueBool(), JIT: i.JIT.ValueBool(),
			})
		}
	}
	return out
}

// expandOIDC builds the OIDC inbound configuration. On update it is merged over the
// current server configuration, so unmanaged fields are kept.
func expandOIDC(ctx context.Context, o *oidcModel, diags *diag.Diagnostics) tfutil.JSON {
	j := tfutil.JSON{}
	j.Str("clientId", o.ClientID)
	if tfutil.Known(o.ClientSecret) && o.ClientSecret.ValueString() != "" {
		j["clientSecret"] = o.ClientSecret.ValueString()
	}
	j.Strs(ctx, "grantTypes", o.GrantTypes, diags)
	j.Strs(ctx, "callbackURLs", o.CallbackURLs, diags)
	j.Strs(ctx, "allowedOrigins", o.AllowedOrigins, diags)
	j.Bool("publicClient", o.PublicClient)
	j.Bool("validateRequestObjectSignature", o.ValidateRequestObjectSignature)
	j.Strs(ctx, "scopeValidators", o.ScopeValidators, diags)
	j.Bool("isFAPIApplication", o.IsFAPIApplication)
	if v := o.TokenIssuerOrganizationID.ValueString(); v != "" {
		j.Obj("issuer", tfutil.JSON{"organizationId": v})
	}

	if v, ok := tfutil.ObjectAs[pkceModel](ctx, o.PKCE, diags); ok {
		j.Obj("pkce", tfutil.JSON{}.Bool("mandatory", v.Mandatory).Bool("supportPlainTransformAlgorithm", v.SupportPlainTransformAlgorithm))
	}
	if v, ok := tfutil.ObjectAs[accessTokenModel](ctx, o.AccessToken, diags); ok {
		j.Obj("accessToken", tfutil.JSON{}.
			Str("type", v.Type).
			Int("userAccessTokenExpiryInSeconds", v.UserAccessTokenExpiryInSeconds).
			Int("applicationAccessTokenExpiryInSeconds", v.ApplicationAccessTokenExpiryInSeconds).
			Str("bindingType", v.BindingType).
			Bool("revokeTokensWhenIDPSessionTerminated", v.RevokeTokensWhenIDPSessionTerminated).
			Bool("validateTokenBinding", v.ValidateTokenBinding).
			Strs(ctx, "accessTokenAttributes", v.AccessTokenAttributes, diags).
			Bool("enableJwtScopeAsArray", v.EnableJWTScopeAsArray))
	}
	if v, ok := tfutil.ObjectAs[refreshTokenModel](ctx, o.RefreshToken, diags); ok {
		j.Obj("refreshToken", tfutil.JSON{}.
			Int("expiryInSeconds", v.ExpiryInSeconds).
			Bool("renewRefreshToken", v.RenewRefreshToken).
			Bool("extendRenewedRefreshTokenExpiryTime", v.ExtendRenewedRefreshTokenExpiryTime))
	}
	if v, ok := tfutil.ObjectAs[idTokenModel](ctx, o.IDToken, diags); ok {
		idt := tfutil.JSON{}.
			Int("expiryInSeconds", v.ExpiryInSeconds).
			Strs(ctx, "audience", v.Audience, diags).
			Str("idTokenSignedResponseAlg", v.SignedResponseAlg)
		if e, ok := tfutil.ObjectAs[encryptionModel](ctx, v.Encryption, diags); ok {
			idt.Obj("encryption", tfutil.JSON{}.Bool("enabled", e.Enabled).Str("algorithm", e.Algorithm).Str("method", e.Method))
		}
		j.Obj("idToken", idt)
	}
	if v, ok := tfutil.ObjectAs[logoutModel](ctx, o.Logout, diags); ok {
		j.Obj("logout", tfutil.JSON{}.Str("backChannelLogoutUrl", v.BackChannelLogoutURL).Str("frontChannelLogoutUrl", v.FrontChannelLogoutURL))
	}
	if v, ok := tfutil.ObjectAs[hybridFlowModel](ctx, o.HybridFlow, diags); ok {
		j.Obj("hybridFlow", tfutil.JSON{}.Bool("enable", v.Enabled).Str("responseType", v.ResponseType))
	}
	if v, ok := tfutil.ObjectAs[subjectTokenModel](ctx, o.SubjectToken, diags); ok {
		j.Obj("subjectToken", tfutil.JSON{}.Bool("enable", v.Enabled).Int("applicationSubjectTokenExpiryInSeconds", v.ApplicationSubjectTokenExpiryInSeconds))
	}
	if v, ok := tfutil.ObjectAs[clientAuthModel](ctx, o.ClientAuthentication, diags); ok {
		j.Obj("clientAuthentication", tfutil.JSON{}.
			Str("tokenEndpointAuthMethod", v.TokenEndpointAuthMethod).
			Str("tokenEndpointAuthSigningAlg", v.TokenEndpointAuthSigningAlg).
			Bool("tokenEndpointAllowReusePvtKeyJwt", v.TokenEndpointAllowReusePvtKeyJWT).
			Str("tlsClientAuthSubjectDn", v.TLSClientAuthSubjectDN))
	}
	if v, ok := tfutil.ObjectAs[requestObjectModel](ctx, o.RequestObject, diags); ok {
		ro := tfutil.JSON{}.Str("requestObjectSigningAlg", v.SigningAlg)
		ro.Obj("encryption", tfutil.JSON{}.Str("algorithm", v.EncryptionAlgorithm).Str("method", v.EncryptionMethod))
		j.Obj("requestObject", ro)
	}
	if tfutil.Known(o.RequirePushedAuthorizationRequests) {
		j["pushAuthorizationRequest"] = map[string]any{"requirePushAuthorizationRequest": o.RequirePushedAuthorizationRequests.ValueBool()}
	}
	if v, ok := tfutil.ObjectAs[oidcSubjectModel](ctx, o.Subject, diags); ok {
		j.Obj("subject", tfutil.JSON{}.Str("subjectType", v.SubjectType).Str("sectorIdentifierUri", v.SectorIdentifierURI))
	}
	if v, ok := tfutil.ObjectAs[cibaModel](ctx, o.CIBA, diags); ok {
		j.Obj("cibaAuthenticationRequest", tfutil.JSON{}.
			Int("authReqExpiryTime", v.AuthRequestExpiryTime).
			Strs(ctx, "notificationChannels", v.NotificationChannels, diags).
			Bool("skipUserValidation", v.SkipUserValidation).
			Bool("allowFederatedUsers", v.AllowFederatedUsers))
	}
	// The server dereferences these lists without null checks (HTTP 500 when absent).
	for _, k := range []string{"callbackURLs", "allowedOrigins", "scopeValidators"} {
		if _, ok := j[k]; !ok {
			j[k] = []string{}
		}
	}
	return j
}

// expandSAML builds the SAML inbound configuration.
func expandSAML(ctx context.Context, s *samlModel, diags *diag.Diagnostics) *client.SAMLInbound {
	if tfutil.Known(s.MetadataURL) && s.MetadataURL.ValueString() != "" && !tfutil.Known(s.Issuer) {
		return &client.SAMLInbound{MetadataURL: s.MetadataURL.ValueString()}
	}
	j := tfutil.JSON{}
	j.Str("issuer", s.Issuer)
	j.Str("serviceProviderQualifier", s.ServiceProviderQualifier)
	j.Strs(ctx, "assertionConsumerUrls", s.AssertionConsumerURLs, diags)
	j.Str("defaultAssertionConsumerUrl", s.DefaultAssertionConsumerURL)
	j.Str("idpEntityIdAlias", s.IdpEntityIDAlias)
	j.Bool("enableAssertionQueryProfile", s.EnableAssertionQueryProfile)
	if v, ok := tfutil.ObjectAs[ssoProfileModel](ctx, s.SingleSignOnProfile, diags); ok {
		sso := tfutil.JSON{}.
			Strs(ctx, "bindings", v.Bindings, diags).
			Bool("enableSignatureValidationForArtifactBinding", v.EnableSignatureValidationForArtifactBinding).
			Bool("enableIdpInitiatedSingleSignOn", v.EnableIdpInitiatedSingleSignOn)
		if a, ok := tfutil.ObjectAs[assertionModel](ctx, v.Assertion, diags); ok {
			as := tfutil.JSON{}.
				Str("nameIdFormat", a.NameIDFormat).
				Strs(ctx, "audiences", a.Audiences, diags).
				Strs(ctx, "recipients", a.Recipients, diags).
				Str("digestAlgorithm", a.DigestAlgorithm)
			if e, ok := tfutil.ObjectAs[assertionEncryptionModel](ctx, a.Encryption, diags); ok {
				as.Obj("encryption", tfutil.JSON{}.
					Bool("enabled", e.Enabled).
					Str("assertionEncryptionAlgorithm", e.AssertionEncryptionAlgorithm).
					Str("keyEncryptionAlgorithm", e.KeyEncryptionAlgorithm))
			}
			sso.Obj("assertion", as)
		}
		j.Obj("singleSignOnProfile", sso)
	}
	if v, ok := tfutil.ObjectAs[attributeProfileModel](ctx, s.AttributeProfile, diags); ok {
		j.Obj("attributeProfile", tfutil.JSON{}.
			Bool("enabled", v.Enabled).
			Bool("alwaysIncludeAttributesInResponse", v.AlwaysIncludeAttributesInResponse).
			Str("nameFormat", v.NameFormat))
	}
	if v, ok := tfutil.ObjectAs[sloProfileModel](ctx, s.SingleLogoutProfile, diags); ok {
		slo := tfutil.JSON{}.
			Bool("enabled", v.Enabled).
			Str("logoutRequestUrl", v.LogoutRequestURL).
			Str("logoutResponseUrl", v.LogoutResponseURL).
			Str("logoutMethod", v.LogoutMethod)
		if i, ok := tfutil.ObjectAs[idpInitiatedSLOModel](ctx, v.IdpInitiatedSingleLogout, diags); ok {
			slo.Obj("idpInitiatedSingleLogout", tfutil.JSON{}.Bool("enabled", i.Enabled).Strs(ctx, "returnToUrls", i.ReturnToURLs, diags))
		}
		j.Obj("singleLogoutProfile", slo)
	}
	if v, ok := tfutil.ObjectAs[requestValidationModel](ctx, s.RequestValidation, diags); ok {
		j.Obj("requestValidation", tfutil.JSON{}.
			Bool("enableSignatureValidation", v.EnableSignatureValidation).
			Str("signatureValidationCertAlias", v.SignatureValidationCertAlias))
	}
	if v, ok := tfutil.ObjectAs[responseSigningModel](ctx, s.ResponseSigning, diags); ok {
		j.Obj("responseSigning", tfutil.JSON{}.Bool("enabled", v.Enabled).Str("signingAlgorithm", v.SigningAlgorithm))
	}
	return &client.SAMLInbound{ManualConfiguration: j.Map()}
}

// normalizeNameIDFormat compares NameID formats the way the server stores them (it
// replaces ':' with '/').
func normalizeNameIDFormat(s string) string {
	return strings.ReplaceAll(s, "/", ":")
}
