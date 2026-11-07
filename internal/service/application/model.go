package application

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type appModel struct {
	ID                               types.String `tfsdk:"id"`
	Name                             types.String `tfsdk:"name"`
	Description                      types.String `tfsdk:"description"`
	ImageURL                         types.String `tfsdk:"image_url"`
	AccessURL                        types.String `tfsdk:"access_url"`
	LogoutReturnURL                  types.String `tfsdk:"logout_return_url"`
	TemplateID                       types.String `tfsdk:"template_id"`
	TemplateVersion                  types.String `tfsdk:"template_version"`
	Enabled                          types.Bool   `tfsdk:"enabled"`
	IsManagementApp                  types.Bool   `tfsdk:"is_management_app"`
	IsB2BSelfServiceApp              types.Bool   `tfsdk:"is_b2b_self_service_app"`
	EnhancedOrgAuthenticationEnabled types.Bool   `tfsdk:"enhanced_org_authentication_enabled"`
	ApplicationVersion               types.String `tfsdk:"application_version"`
	ClientID                         types.String `tfsdk:"client_id"`
	Issuer                           types.String `tfsdk:"issuer"`
	Access                           types.String `tfsdk:"access"`
	AssociatedRoles                  types.Object `tfsdk:"associated_roles"`
	ClaimConfiguration               types.Object `tfsdk:"claim_configuration"`
	AuthenticationSequence           types.Object `tfsdk:"authentication_sequence"`
	AdvancedConfiguration            types.Object `tfsdk:"advanced_configuration"`
	Provisioning                     types.Object `tfsdk:"provisioning"`
	OIDC                             types.Object `tfsdk:"oidc"`
	SAML                             types.Object `tfsdk:"saml"`
	PassiveSTS                       types.Object `tfsdk:"passive_sts"`
	WSTrust                          types.Object `tfsdk:"ws_trust"`
}

type associatedRolesModel struct {
	AllowedAudience types.String `tfsdk:"allowed_audience"`
	RoleIDs         types.Set    `tfsdk:"role_ids"`
}

type claimConfigModel struct {
	Dialect         types.String `tfsdk:"dialect"`
	RequestedClaims types.Set    `tfsdk:"requested_claims"`
	ClaimMappings   types.Set    `tfsdk:"claim_mappings"`
	Subject         types.Object `tfsdk:"subject"`
	Role            types.Object `tfsdk:"role"`
}

type requestedClaimModel struct {
	URI       types.String `tfsdk:"uri"`
	Mandatory types.Bool   `tfsdk:"mandatory"`
}

type claimMappingModel struct {
	ApplicationClaim types.String `tfsdk:"application_claim"`
	LocalClaim       types.String `tfsdk:"local_claim"`
}

type subjectModel struct {
	Claim                       types.String `tfsdk:"claim"`
	IncludeUserDomain           types.Bool   `tfsdk:"include_user_domain"`
	IncludeTenantDomain         types.Bool   `tfsdk:"include_tenant_domain"`
	UseMappedLocalSubject       types.Bool   `tfsdk:"use_mapped_local_subject"`
	MappedLocalSubjectMandatory types.Bool   `tfsdk:"mapped_local_subject_mandatory"`
}

type roleClaimModel struct {
	Claim             types.String `tfsdk:"claim"`
	IncludeUserDomain types.Bool   `tfsdk:"include_user_domain"`
	Mappings          types.Set    `tfsdk:"mappings"`
}

type roleMappingModel struct {
	LocalRole       types.String `tfsdk:"local_role"`
	ApplicationRole types.String `tfsdk:"application_role"`
}

type authSeqModel struct {
	Type                      types.String `tfsdk:"type"`
	Steps                     types.List   `tfsdk:"steps"`
	Script                    types.String `tfsdk:"script"`
	SubjectStepID             types.Int64  `tfsdk:"subject_step_id"`
	AttributeStepID           types.Int64  `tfsdk:"attribute_step_id"`
	RequestPathAuthenticators types.List   `tfsdk:"request_path_authenticators"`
}

type stepModel struct {
	Options types.Set `tfsdk:"options"`
}

type optionModel struct {
	IDP           types.String `tfsdk:"idp"`
	Authenticator types.String `tfsdk:"authenticator"`
}

type advancedModel struct {
	SaaS                         types.Bool   `tfsdk:"saas"`
	DiscoverableByEndUsers       types.Bool   `tfsdk:"discoverable_by_end_users"`
	SkipLoginConsent             types.Bool   `tfsdk:"skip_login_consent"`
	SkipLogoutConsent            types.Bool   `tfsdk:"skip_logout_consent"`
	UseExternalConsentPage       types.Bool   `tfsdk:"use_external_consent_page"`
	ReturnAuthenticatedIdpList   types.Bool   `tfsdk:"return_authenticated_idp_list"`
	EnableAuthorization          types.Bool   `tfsdk:"enable_authorization"`
	EnableAPIBasedAuthentication types.Bool   `tfsdk:"enable_api_based_authentication"`
	Fragment                     types.Bool   `tfsdk:"fragment"`
	DiscoverableGroups           types.Set    `tfsdk:"discoverable_groups"`
	Certificate                  types.Object `tfsdk:"certificate"`
	Attestation                  types.Object `tfsdk:"attestation"`
	TrustedApp                   types.Object `tfsdk:"trusted_app"`
}

type discoverableGroupModel struct {
	UserStore types.String `tfsdk:"user_store"`
	GroupIDs  types.Set    `tfsdk:"group_ids"`
}

type certificateModel struct {
	Type  types.String `tfsdk:"type"`
	Value types.String `tfsdk:"value"`
}

type attestationModel struct {
	Enabled                              types.Bool   `tfsdk:"enabled"`
	AndroidPackageName                   types.String `tfsdk:"android_package_name"`
	AppleAppID                           types.String `tfsdk:"apple_app_id"`
	AndroidAttestationServiceCredentials types.String `tfsdk:"android_attestation_service_credentials"`
}

type trustedAppModel struct {
	IsFIDOTrustedApp   types.Bool   `tfsdk:"is_fido_trusted_app"`
	IsConsentGranted   types.Bool   `tfsdk:"is_consent_granted"`
	AndroidPackageName types.String `tfsdk:"android_package_name"`
	AndroidThumbprints types.Set    `tfsdk:"android_thumbprints"`
	AppleAppID         types.String `tfsdk:"apple_app_id"`
}

type provisioningModel struct {
	Inbound                   types.Object `tfsdk:"inbound"`
	OutboundIdentityProviders types.List   `tfsdk:"outbound_identity_providers"`
}

type inboundProvisioningModel struct {
	ProxyMode                   types.Bool   `tfsdk:"proxy_mode"`
	ProvisioningUserstoreDomain types.String `tfsdk:"provisioning_userstore_domain"`
}

type outboundIdpModel struct {
	IDP       types.String `tfsdk:"idp"`
	Connector types.String `tfsdk:"connector"`
	Blocking  types.Bool   `tfsdk:"blocking"`
	Rules     types.Bool   `tfsdk:"rules"`
	JIT       types.Bool   `tfsdk:"jit"`
}

type oidcModel struct {
	ClientID                           types.String `tfsdk:"client_id"`
	ClientSecret                       types.String `tfsdk:"client_secret"`
	State                              types.String `tfsdk:"state"`
	GrantTypes                         types.Set    `tfsdk:"grant_types"`
	CallbackURLs                       types.List   `tfsdk:"callback_urls"`
	AllowedOrigins                     types.Set    `tfsdk:"allowed_origins"`
	PublicClient                       types.Bool   `tfsdk:"public_client"`
	PKCE                               types.Object `tfsdk:"pkce"`
	AccessToken                        types.Object `tfsdk:"access_token"`
	RefreshToken                       types.Object `tfsdk:"refresh_token"`
	IDToken                            types.Object `tfsdk:"id_token"`
	Logout                             types.Object `tfsdk:"logout"`
	HybridFlow                         types.Object `tfsdk:"hybrid_flow"`
	SubjectToken                       types.Object `tfsdk:"subject_token"`
	ValidateRequestObjectSignature     types.Bool   `tfsdk:"validate_request_object_signature"`
	ScopeValidators                    types.Set    `tfsdk:"scope_validators"`
	ClientAuthentication               types.Object `tfsdk:"client_authentication"`
	RequestObject                      types.Object `tfsdk:"request_object"`
	RequirePushedAuthorizationRequests types.Bool   `tfsdk:"require_pushed_authorization_requests"`
	Subject                            types.Object `tfsdk:"subject"`
	IsFAPIApplication                  types.Bool   `tfsdk:"is_fapi_application"`
	CIBA                               types.Object `tfsdk:"ciba"`
	TokenIssuerOrganizationID          types.String `tfsdk:"token_issuer_organization_id"`
}

type pkceModel struct {
	Mandatory                      types.Bool `tfsdk:"mandatory"`
	SupportPlainTransformAlgorithm types.Bool `tfsdk:"support_plain_transform_algorithm"`
}

type accessTokenModel struct {
	Type                                  types.String `tfsdk:"type"`
	UserAccessTokenExpiryInSeconds        types.Int64  `tfsdk:"user_access_token_expiry_in_seconds"`
	ApplicationAccessTokenExpiryInSeconds types.Int64  `tfsdk:"application_access_token_expiry_in_seconds"`
	BindingType                           types.String `tfsdk:"binding_type"`
	RevokeTokensWhenIDPSessionTerminated  types.Bool   `tfsdk:"revoke_tokens_when_idp_session_terminated"`
	ValidateTokenBinding                  types.Bool   `tfsdk:"validate_token_binding"`
	AccessTokenAttributes                 types.Set    `tfsdk:"access_token_attributes"`
	EnableJWTScopeAsArray                 types.Bool   `tfsdk:"enable_jwt_scope_as_array"`
}

type refreshTokenModel struct {
	ExpiryInSeconds                     types.Int64 `tfsdk:"expiry_in_seconds"`
	RenewRefreshToken                   types.Bool  `tfsdk:"renew_refresh_token"`
	ExtendRenewedRefreshTokenExpiryTime types.Bool  `tfsdk:"extend_renewed_refresh_token_expiry_time"`
}

type idTokenModel struct {
	ExpiryInSeconds   types.Int64  `tfsdk:"expiry_in_seconds"`
	Audience          types.Set    `tfsdk:"audience"`
	SignedResponseAlg types.String `tfsdk:"signed_response_alg"`
	Encryption        types.Object `tfsdk:"encryption"`
}

type encryptionModel struct {
	Enabled   types.Bool   `tfsdk:"enabled"`
	Algorithm types.String `tfsdk:"algorithm"`
	Method    types.String `tfsdk:"method"`
}

type logoutModel struct {
	BackChannelLogoutURL  types.String `tfsdk:"back_channel_logout_url"`
	FrontChannelLogoutURL types.String `tfsdk:"front_channel_logout_url"`
}

type hybridFlowModel struct {
	Enabled      types.Bool   `tfsdk:"enabled"`
	ResponseType types.String `tfsdk:"response_type"`
}

type subjectTokenModel struct {
	Enabled                                types.Bool  `tfsdk:"enabled"`
	ApplicationSubjectTokenExpiryInSeconds types.Int64 `tfsdk:"application_subject_token_expiry_in_seconds"`
}

type clientAuthModel struct {
	TokenEndpointAuthMethod          types.String `tfsdk:"token_endpoint_auth_method"`
	TokenEndpointAuthSigningAlg      types.String `tfsdk:"token_endpoint_auth_signing_alg"`
	TokenEndpointAllowReusePvtKeyJWT types.Bool   `tfsdk:"token_endpoint_allow_reuse_pvt_key_jwt"`
	TLSClientAuthSubjectDN           types.String `tfsdk:"tls_client_auth_subject_dn"`
}

type requestObjectModel struct {
	SigningAlg          types.String `tfsdk:"signing_alg"`
	EncryptionAlgorithm types.String `tfsdk:"encryption_algorithm"`
	EncryptionMethod    types.String `tfsdk:"encryption_method"`
}

type oidcSubjectModel struct {
	SubjectType         types.String `tfsdk:"subject_type"`
	SectorIdentifierURI types.String `tfsdk:"sector_identifier_uri"`
}

type cibaModel struct {
	AuthRequestExpiryTime types.Int64 `tfsdk:"auth_request_expiry_time"`
	NotificationChannels  types.Set   `tfsdk:"notification_channels"`
	SkipUserValidation    types.Bool  `tfsdk:"skip_user_validation"`
	AllowFederatedUsers   types.Bool  `tfsdk:"allow_federated_users"`
}

type samlModel struct {
	MetadataURL                 types.String `tfsdk:"metadata_url"`
	Issuer                      types.String `tfsdk:"issuer"`
	ServiceProviderQualifier    types.String `tfsdk:"service_provider_qualifier"`
	AssertionConsumerURLs       types.List   `tfsdk:"assertion_consumer_urls"`
	DefaultAssertionConsumerURL types.String `tfsdk:"default_assertion_consumer_url"`
	IdpEntityIDAlias            types.String `tfsdk:"idp_entity_id_alias"`
	SingleSignOnProfile         types.Object `tfsdk:"single_sign_on_profile"`
	AttributeProfile            types.Object `tfsdk:"attribute_profile"`
	SingleLogoutProfile         types.Object `tfsdk:"single_logout_profile"`
	RequestValidation           types.Object `tfsdk:"request_validation"`
	ResponseSigning             types.Object `tfsdk:"response_signing"`
	EnableAssertionQueryProfile types.Bool   `tfsdk:"enable_assertion_query_profile"`
}

type ssoProfileModel struct {
	Bindings                                    types.Set    `tfsdk:"bindings"`
	EnableSignatureValidationForArtifactBinding types.Bool   `tfsdk:"enable_signature_validation_for_artifact_binding"`
	EnableIdpInitiatedSingleSignOn              types.Bool   `tfsdk:"enable_idp_initiated_single_sign_on"`
	AttributeConsumingServiceIndex              types.String `tfsdk:"attribute_consuming_service_index"`
	Assertion                                   types.Object `tfsdk:"assertion"`
}

type assertionModel struct {
	NameIDFormat    types.String `tfsdk:"name_id_format"`
	Audiences       types.Set    `tfsdk:"audiences"`
	Recipients      types.Set    `tfsdk:"recipients"`
	DigestAlgorithm types.String `tfsdk:"digest_algorithm"`
	Encryption      types.Object `tfsdk:"encryption"`
}

type assertionEncryptionModel struct {
	Enabled                      types.Bool   `tfsdk:"enabled"`
	AssertionEncryptionAlgorithm types.String `tfsdk:"assertion_encryption_algorithm"`
	KeyEncryptionAlgorithm       types.String `tfsdk:"key_encryption_algorithm"`
}

type attributeProfileModel struct {
	Enabled                           types.Bool   `tfsdk:"enabled"`
	AlwaysIncludeAttributesInResponse types.Bool   `tfsdk:"always_include_attributes_in_response"`
	NameFormat                        types.String `tfsdk:"name_format"`
}

type sloProfileModel struct {
	Enabled                  types.Bool   `tfsdk:"enabled"`
	LogoutRequestURL         types.String `tfsdk:"logout_request_url"`
	LogoutResponseURL        types.String `tfsdk:"logout_response_url"`
	LogoutMethod             types.String `tfsdk:"logout_method"`
	IdpInitiatedSingleLogout types.Object `tfsdk:"idp_initiated_single_logout"`
}

type idpInitiatedSLOModel struct {
	Enabled      types.Bool `tfsdk:"enabled"`
	ReturnToURLs types.Set  `tfsdk:"return_to_urls"`
}

type requestValidationModel struct {
	EnableSignatureValidation    types.Bool   `tfsdk:"enable_signature_validation"`
	SignatureValidationCertAlias types.String `tfsdk:"signature_validation_cert_alias"`
}

type responseSigningModel struct {
	Enabled          types.Bool   `tfsdk:"enabled"`
	SigningAlgorithm types.String `tfsdk:"signing_algorithm"`
}

type passiveSTSModel struct {
	Realm         types.String `tfsdk:"realm"`
	ReplyTo       types.String `tfsdk:"reply_to"`
	ReplyToLogout types.String `tfsdk:"reply_to_logout"`
}

type wsTrustModel struct {
	Audience         types.String `tfsdk:"audience"`
	CertificateAlias types.String `tfsdk:"certificate_alias"`
}
