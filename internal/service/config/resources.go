package config

import "github.com/hashicorp/terraform-plugin-framework/resource"

var singletons = []singleton{
	{
		typeName: "server_configuration",
		desc:     "Manages the tenant-wide session and provisioning settings. In the Console, these settings are under **Login & Registration > Session** and in the resident identity provider.",
		path:     "/configs",
		style:    patchOps,
		fields: []field{
			{attr: "idle_session_timeout", kind: kindInt, key: "idleSessionTimeoutPeriod", desc: "Idle session timeout, in minutes."},
			{attr: "remember_me_period", kind: kindInt, key: "rememberMePeriod", desc: "How long *Remember me* keeps users signed in, in minutes."},
			{attr: "enable_maximum_session_timeout", kind: kindBool, key: "enableMaximumSessionTimeoutPeriod", desc: "End sessions after `maximum_session_timeout` regardless of activity."},
			{attr: "maximum_session_timeout", kind: kindInt, key: "maximumSessionTimeoutPeriod", desc: "Maximum session lifetime, in minutes."},
			{attr: "preserve_current_session_at_password_update", kind: kindBool, key: "preserveCurrentSessionAtPasswordUpdate", desc: "Keep the current session when the user changes their password (other sessions end)."},
			{attr: "home_realm_identifiers", kind: kindStringList, key: "homeRealmIdentifiers", listOps: listIndexed, desc: "Home realm identifiers of the resident identity provider."},
		},
	},
	{
		typeName: "inbound_provisioning_configuration",
		desc: "Manages inbound SCIM provisioning for requests that sign in with basic authentication (the resident service provider). " +
			"Requests that use an application's OAuth2 token follow that application's own provisioning settings instead.",
		path:  "/configs/provisioning/inbound/scim",
		style: putObject,
		fields: []field{
			{attr: "proxy_mode", kind: kindBool, key: "enableProxyMode", desc: "Forward SCIM provisioning requests to outbound connectors without storing users locally."},
			{attr: "provisioning_userstore", kind: kindString, key: "provisioningUserstore", desc: "The user store that receives every user created through `/scim2`. The default, `\"\"`, lets the `DOMAIN/` prefix of each username choose the user store. **When you set a user store, the server ignores the user store of new users, including `wso2is_user.user_store`. It also refuses SCIM updates of users in other user stores.**"},
		},
		resetBody: map[string]any{"provisioningUserstore": "", "enableProxyMode": false},
	},
	{
		typeName: "cors_configuration",
		desc:     "Manages the CORS settings of the server's endpoints. The allowed origins come from the `oidc.allowed_origins` setting of each application.",
		path:     "/configs/cors",
		style:    patchOps,
		fields: []field{
			{attr: "allow_generic_http_requests", kind: kindBool, key: "allowGenericHttpRequests", desc: "Let non-CORS requests through."},
			{attr: "allow_any_origin", kind: kindBool, key: "allowAnyOrigin", desc: "Allow every origin (not recommended)."},
			{attr: "allow_subdomains", kind: kindBool, key: "allowSubdomains", desc: "Allow subdomains of the allowed origins."},
			{attr: "supported_methods", kind: kindStringSet, listOps: listAddEach, key: "supportedMethods", desc: "HTTP methods allowed in CORS requests."},
			{attr: "support_any_header", kind: kindBool, key: "supportAnyHeader", desc: "Allow every request header."},
			{attr: "supported_headers", kind: kindStringSet, listOps: listAddEach, key: "supportedHeaders", desc: "Request headers allowed when `support_any_header` is off."},
			{attr: "exposed_headers", kind: kindStringSet, listOps: listAddEach, key: "exposedHeaders", desc: "Response headers exposed to browsers."},
			{attr: "supports_credentials", kind: kindBool, key: "supportsCredentials", desc: "Allow credentials (cookies, authorization headers)."},
			{attr: "max_age", kind: kindInt, key: "maxAge", desc: "Preflight cache lifetime in seconds (`-1` = not set)."},
		},
	},
	{
		typeName:   "impersonation_configuration",
		desc:       "Manages the user impersonation settings.",
		path:       "/configs/impersonation",
		style:      patchOpsTyped,
		revertPath: "/configs/impersonation",
		fields: []field{
			{attr: "enable_email_notification", kind: kindBool, key: "enableEmailNotification", desc: "Email users when they are impersonated."},
		},
	},
	{
		typeName: "jwt_key_validator_configuration",
		desc:     "Manages how the server checks JWTs that clients use as client assertions (`private_key_jwt`).",
		path:     "/configs/jwt-key-validator",
		style:    patchOpsTyped,
		fields: []field{
			{attr: "enable_token_reuse", kind: kindBool, key: "enableTokenReuse", desc: "Allow a client assertion JWT (same `jti`) to be used more than once."},
		},
	},
	{
		typeName: "dcr_configuration",
		desc:     "Manages OAuth2 dynamic client registration (`/api/identity/oauth2/dcr/v1.1/register`).",
		path:     "/configs/dcr",
		style:    patchOps,
		fields: []field{
			{attr: "authentication_required", kind: kindBool, key: "authenticationRequired", desc: "Require authentication to register clients."},
			{attr: "enable_fapi_enforcement", kind: kindBool, key: "enableFapiEnforcement", desc: "Enforce FAPI rules on registered clients."},
			{attr: "mandate_ssa", kind: kindBool, key: "mandateSSA", desc: "Require a software statement assertion."},
			{attr: "ssa_jwks", kind: kindString, key: "ssaJwks", desc: "JWKS URL used to validate software statement assertions."},
		},
	},
	{
		typeName:   "saml_idp_configuration",
		desc:       "Manages the server's settings as a SAML identity provider, such as its metadata and destination URLs.",
		path:       "/configs/authentication/inbound/saml2",
		style:      patchObject,
		revertPath: "/configs/authentication/inbound/saml2",
		fields: []field{
			{attr: "destination_urls", kind: kindStringList, key: "destinationURLs", desc: "SAML SSO endpoint URLs advertised to service providers."},
			{attr: "metadata_validity_period", kind: kindInt, key: "metadataValidityPeriod", desc: "Validity of the published metadata, in minutes."},
			{attr: "enable_metadata_signing", kind: kindBool, key: "enableMetadataSigning", desc: "Sign the published metadata."},
		},
	},
	{
		typeName:   "passive_sts_configuration",
		desc:       "Manages the WS-Federation (passive STS) endpoint.",
		path:       "/configs/authentication/inbound/passivests",
		style:      patchObject,
		revertPath: "/configs/authentication/inbound/passivests",
		fields: []field{
			{attr: "enable_request_signing", kind: kindBool, key: "enableRequestSigning", desc: "Sign authentication requests."},
		},
	},
	{
		typeName:   "oauth2_configuration",
		desc:       "Manages the tenant-wide OAuth2 settings.",
		path:       "/configs/authentication/inbound/oauth2",
		style:      patchObject,
		revertPath: "/configs/authentication/inbound/oauth2",
		fields: []field{
			{attr: "enable_jwt_scope_as_array", kind: kindBool, key: "enableJwtScopeAsArray", desc: "Send the `scope` claim of JWT access tokens as an array (applications can override it)."},
		},
	},
	{
		typeName: "oauth2_issuer_usage_scope",
		desc:     "Manages which organizations may use the root organization's token issuer.",
		path:     "/configs/authentication/inbound/oauth2/issuer/usage-scope",
		style:    patchObject,
		fields: []field{
			{attr: "usage_scope", kind: kindString, key: "usageScope", desc: "`ALL_EXISTING_AND_FUTURE_ORGS` or `NONE`."},
		},
	},
	{
		typeName: "admin_advisory_banner",
		desc:     "Manages the advisory banner on the Console sign-in page.",
		path:     "/admin-advisory-management/banner",
		style:    patchObject,
		fields: []field{
			{attr: "enabled", kind: kindBool, key: "enableBanner", desc: "Show the banner."},
			{attr: "content", kind: kindString, key: "bannerContent", desc: "Banner text."},
		},
	},
	{
		typeName: "fraud_detection_configuration",
		desc:     "Manages how the server sends events to a fraud detection service.",
		path:     "/configs/fraud-detection",
		style:    putObject,
		fields: []field{
			{attr: "publish_user_info", kind: kindBool, key: "publishUserInfo", desc: "Include user information in events."},
			{attr: "publish_device_metadata", kind: kindBool, key: "publishDeviceMetadata", desc: "Include device metadata in events."},
			{attr: "log_request_payload", kind: kindBool, key: "logRequestPayload", desc: "Log the payload sent to the fraud detection service."},
		},
	},
}

// Resources returns the configuration singleton resources.
func Resources() []func() resource.Resource {
	out := make([]func() resource.Resource, 0, len(singletons)+1)
	for _, s := range singletons {
		out = append(out, s.factory())
	}
	return append(out, NewRemoteLoggingResource)
}
