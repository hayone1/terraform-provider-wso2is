package application

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hayone1/terraform-provider-wso2is/internal/tfutil"
)

var (
	oc      = tfutil.OCString
	ocBool  = tfutil.OCBool
	ocInt   = tfutil.OCInt64
	optBool = tfutil.Bool
	optStr  = tfutil.String
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		MarkdownDescription: "Manages an application, which the server also calls a service provider. The resource covers the application's sign-in protocols (OpenID Connect and OAuth2, SAML 2.0, WS-Federation and WS-Trust), its login flow, the user attributes that it requests, its roles and its advanced settings.\n\nIf you leave out a setting, the server uses its default value, and the provider shows that value in the state. If you later remove a setting from the configuration, the setting keeps its last value. To change it, set the new value explicitly.\n\nManage the API scopes that the application may request with `wso2is_application_authorized_api`. Share the application with sub-organizations with `wso2is_application_share`. To import an application, use its ID.",
		Attributes: map[string]schema.Attribute{
			"id": tfutil.IDAttribute("Application ID."),
			"name": schema.StringAttribute{
				MarkdownDescription: "Unique application name.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(3)},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description shown in the Console.",
				Optional:            true,
			},
			"image_url": schema.StringAttribute{
				MarkdownDescription: "Logo URL.",
				Optional:            true,
			},
			"access_url": schema.StringAttribute{
				MarkdownDescription: "URL users open to reach the application (shown in My Account for discoverable applications).",
				Optional:            true,
			},
			"logout_return_url": schema.StringAttribute{
				MarkdownDescription: "The URL that users go to after they log out, when the application does not send one. When you clear this value, the server stores `.*`, which allows any URL. The provider shows `.*` as unset.",
				Optional:            true,
			},
			"template_id":      oc("The Console template that the application was created from, for example `custom-application`, `m2m-application`, `single-page-application`, `oidc-web-application` or `react-application`. The template only changes how the Console shows the application."),
			"template_version": oc("Version of the Console template."),
			"enabled":          optBool("Whether users can sign in to the application.", true),
			"is_management_app": schema.BoolAttribute{
				MarkdownDescription: "Whether the application can call the management (system) APIs. The server only accepts this value when it creates the application, so changing it replaces the application.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool(), boolplanmodifier.RequiresReplace()},
			},
			"is_b2b_self_service_app": schema.BoolAttribute{
				MarkdownDescription: "Whether the application is a B2B self-service application. The server only accepts this value when it creates the application, so changing it replaces the application.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool(), boolplanmodifier.RequiresReplace()},
			},
			"enhanced_org_authentication_enabled": ocBool("Whether enhanced organization authentication is enabled (organization login uses the organization's own login settings)."),
			"application_version":                 tfutil.ComputedString("Version of the application model on the server."),
			"client_id": schema.StringAttribute{
				MarkdownDescription: "OAuth2 client ID, when OpenID Connect is configured.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{derivedFromProtocol("oidc", "client_id")},
			},
			"issuer": schema.StringAttribute{
				MarkdownDescription: "SAML issuer, when SAML is configured.",
				Computed:            true,
				PlanModifiers:       []planmodifier.String{derivedFromProtocol("saml", "issuer")},
			},
			"access": tfutil.ComputedString("What the provider's user may do with this application. The value is `READ` for read-only system applications, and `WRITE` otherwise."),

			"associated_roles": tfutil.Object("Which roles the application's users can be assigned and receive in tokens.", map[string]schema.Attribute{
				"allowed_audience": schema.StringAttribute{
					MarkdownDescription: "Which roles the application uses. With `APPLICATION`, the application only uses roles that were created for it. With `ORGANIZATION`, it uses the organization's roles. New applications use `APPLICATION` by default.",
					Optional:            true,
					Computed:            true,
					PlanModifiers:       []planmodifier.String{tfutil.KeepStateString()},
					Validators:          []validator.String{stringvalidator.OneOf("APPLICATION", "ORGANIZATION")},
				},
				"role_ids": tfutil.OCStringSet("The IDs of the roles that the application uses. Set this list when `allowed_audience = \"ORGANIZATION\"`. When `allowed_audience = \"APPLICATION\"`, the list is read-only and shows the roles that `wso2is_role` created for the application."),
			}),

			"claim_configuration":     claimConfigurationSchema(),
			"authentication_sequence": authenticationSequenceSchema(),
			"advanced_configuration":  advancedConfigurationSchema(),
			"provisioning":            provisioningSchema(),

			"oidc":        oidcSchema(),
			"saml":        samlSchema(),
			"passive_sts": passiveSTSSchema(),
			"ws_trust":    wsTrustSchema(),
		},
	}
}

func claimConfigurationSchema() schema.SingleNestedAttribute {
	return tfutil.Object("User attributes (claims) the application requests and how the subject and roles are sent.", map[string]schema.Attribute{
		"dialect": schema.StringAttribute{
			MarkdownDescription: "`LOCAL` (use local claim URIs) or `CUSTOM` (map application claim names with `claim_mappings`).",
			Optional:            true,
			Computed:            true,
			PlanModifiers:       []planmodifier.String{tfutil.KeepStateString()},
			Validators:          []validator.String{stringvalidator.OneOf("LOCAL", "CUSTOM")},
		},
		"requested_claims": tfutil.ObjectSet("The claims that the server shares with the application. With the `LOCAL` dialect, use local claim URIs (`http://wso2.org/claims/...`). With the `CUSTOM` dialect, use the application's claim names from `claim_mappings`.", map[string]schema.Attribute{
			"uri": schema.StringAttribute{
				MarkdownDescription: "Claim URI.",
				Required:            true,
			},
			"mandatory": schema.BoolAttribute{
				MarkdownDescription: "Whether the user must provide a value before signing in. The default is `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
		}),
		"claim_mappings": schema.SetNestedAttribute{
			MarkdownDescription: "Maps the application's claim names to local claims. The `CUSTOM` dialect needs these mappings. With the `LOCAL` dialect, the server creates them from `requested_claims`.",
			Optional:            true,
			Computed:            true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"application_claim": schema.StringAttribute{MarkdownDescription: "Claim name the application receives.", Required: true},
				"local_claim":       schema.StringAttribute{MarkdownDescription: "Local claim URI.", Required: true},
			}},
			PlanModifiers: []planmodifier.Set{tfutil.SetStateUnlessChanged(
				path.MatchRelative().AtParent().AtName("requested_claims"),
				path.MatchRelative().AtParent().AtName("dialect"),
			)},
		},
		"subject": tfutil.Object("How the subject identifier is built.", map[string]schema.Attribute{
			"claim":                          oc("Local claim used as the subject. The default is `http://wso2.org/claims/userid`."),
			"include_user_domain":            optBool("Prefix the subject with the user store domain.", false),
			"include_tenant_domain":          optBool("Suffix the subject with the tenant domain.", false),
			"use_mapped_local_subject":       ocBool("Use the local user mapped to a federated user as the subject. The server turns it on when the application is shared with organizations."),
			"mapped_local_subject_mandatory": optBool("Fail federated logins that have no mapped local user.", false),
		}),
		"role": tfutil.Object("How roles are sent to the application.", map[string]schema.Attribute{
			"claim":               oc("Claim carrying roles. The default is `http://wso2.org/claims/role`."),
			"include_user_domain": optBool("Prefix role names with the user store domain.", true),
			"mappings": tfutil.ObjectSet("Local role to application role name mappings.", map[string]schema.Attribute{
				"local_role":       schema.StringAttribute{MarkdownDescription: "Local role name.", Required: true},
				"application_role": schema.StringAttribute{MarkdownDescription: "Role name sent to the application.", Required: true},
			}),
		}),
	})
}

func authenticationSequenceSchema() schema.SingleNestedAttribute {
	return tfutil.OCObject("The login flow. It holds the authentication steps and an optional adaptive (conditional) authentication script. If you remove this block, the current flow stays as it is. To go back to the default flow, set `type = \"DEFAULT\"`.", map[string]schema.Attribute{
		"type": schema.StringAttribute{
			MarkdownDescription: "`DEFAULT` uses the tenant's default flow (username and password). `USER_DEFINED` uses `steps` and `script`. Set automatically to `USER_DEFINED` when `steps` or `script` are configured.",
			Optional:            true,
			Computed:            true,
			PlanModifiers:       []planmodifier.String{authSequenceType()},
			Validators:          []validator.String{stringvalidator.OneOf("DEFAULT", "USER_DEFINED")},
		},
		"steps": tfutil.OCObjectList("Authentication steps, in order. Each step lets the user choose one of its options.", map[string]schema.Attribute{
			"options": schema.SetNestedAttribute{
				MarkdownDescription: "Authenticators offered in this step (the login page shows them in the server's order).",
				Required:            true,
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"idp": schema.StringAttribute{
						MarkdownDescription: "`LOCAL` for local authenticators, otherwise the identity provider (connection) name.",
						Required:            true,
					},
					"authenticator": schema.StringAttribute{
						MarkdownDescription: "Authenticator name, for example `BasicAuthenticator`, `totp`, `email-otp-authenticator`, `sms-otp-authenticator`, `FIDOAuthenticator`, `MagicLinkAuthenticator`, or a federated authenticator such as `GoogleOIDCAuthenticator` or `OpenIDConnectAuthenticator`.",
						Required:            true,
					},
				}},
			},
		}),
		"script":                      optStr("Adaptive authentication script (JavaScript).", ""),
		"subject_step_id":             ocInt("Step (1-based) whose user identity is used as the subject."),
		"attribute_step_id":           ocInt("Step (1-based) whose user attributes are used."),
		"request_path_authenticators": tfutil.OCStringList("Request path authenticators, for example `BasicAuthRequestPathAuthenticator` or `OAuthRequestPathAuthenticator`."),
	})
}

func advancedConfigurationSchema() schema.SingleNestedAttribute {
	return tfutil.Object("Advanced application settings.", map[string]schema.Attribute{
		"saas":                            optBool("Whether users of every tenant can sign in (super tenant only).", false),
		"discoverable_by_end_users":       optBool("Whether the application is listed in My Account.", false),
		"skip_login_consent":              optBool("Skip the attribute consent page at login.", false),
		"skip_logout_consent":             optBool("Skip the logout confirmation page.", false),
		"use_external_consent_page":       optBool("Use an external consent page.", false),
		"return_authenticated_idp_list":   optBool("Return the authenticated identity providers to the application.", false),
		"enable_authorization":            optBool("Engage authorization policies during authentication.", false),
		"enable_api_based_authentication": optBool("Allow app-native (API based) authentication.", false),
		"fragment": schema.BoolAttribute{
			MarkdownDescription: "Whether this is a fragment (shared) application created by organization sharing.",
			Computed:            true,
			PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool()},
		},
		"discoverable_groups": tfutil.ObjectSet("Groups whose members can discover the application in My Account. Empty means everyone.", map[string]schema.Attribute{
			"user_store": schema.StringAttribute{MarkdownDescription: "User store domain of the groups, for example `PRIMARY`.", Required: true},
			"group_ids":  schema.SetAttribute{MarkdownDescription: "Group IDs.", Required: true, ElementType: types.StringType},
		}),
		"certificate": tfutil.Object("Application certificate used to validate signed requests and encrypt tokens.", map[string]schema.Attribute{
			"type": schema.StringAttribute{
				MarkdownDescription: "`PEM` or `JWKS`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString()},
				Validators:          []validator.String{stringvalidator.OneOf("PEM", "JWKS")},
			},
			"value": oc("PEM certificate, or the JWKS endpoint URL."),
		}),
		"attestation": tfutil.Object("Client attestation for app-native authentication.", map[string]schema.Attribute{
			"enabled":              optBool("Require client attestation.", false),
			"android_package_name": optStr("Android package name.", ""),
			"apple_app_id":         optStr("Apple app ID.", ""),
			"android_attestation_service_credentials": schema.StringAttribute{
				MarkdownDescription: "The Google Play Integrity service account credentials, as JSON. The server never returns this value.",
				Optional:            true,
				Sensitive:           true,
			},
		}),
		"trusted_app": tfutil.Object("Trusted (FIDO) application settings for passkeys in native apps.", map[string]schema.Attribute{
			"is_fido_trusted_app":  optBool("Whether the app is a FIDO trusted app.", false),
			"is_consent_granted":   optBool("Whether consent to publish the app details is granted.", false),
			"android_package_name": optStr("Android package name.", ""),
			"android_thumbprints":  tfutil.StringSet("Android signing certificate thumbprints."),
			"apple_app_id":         optStr("Apple app ID.", ""),
		}),
	})
}

func provisioningSchema() schema.SingleNestedAttribute {
	return tfutil.Object("Inbound (SCIM) and outbound provisioning.", map[string]schema.Attribute{
		"inbound": tfutil.Object("Users and groups provisioned through SCIM by this application.", map[string]schema.Attribute{
			"proxy_mode":                    optBool("Forward provisioning requests to outbound connectors without storing users locally.", false),
			"provisioning_userstore_domain": oc("User store that receives provisioned users (when not in proxy mode)."),
		}),
		"outbound_identity_providers": tfutil.ObjectList("Identity providers users are provisioned to.", map[string]schema.Attribute{
			"idp":       schema.StringAttribute{MarkdownDescription: "Identity provider name.", Required: true},
			"connector": schema.StringAttribute{MarkdownDescription: "Outbound provisioning connector name, for example `scim2`, `googleapps`, `salesforce`.", Required: true},
			"blocking":  schema.BoolAttribute{MarkdownDescription: "Wait for provisioning to finish.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"rules":     schema.BoolAttribute{MarkdownDescription: "Apply provisioning rules.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
			"jit":       schema.BoolAttribute{MarkdownDescription: "Provision just-in-time provisioned users too.", Optional: true, Computed: true, Default: booldefault.StaticBool(false)},
		}),
	})
}

func oidcSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "OpenID Connect / OAuth2 configuration. Remove the attribute to remove the protocol from the application.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				MarkdownDescription: "The OAuth2 client ID. The server generates one when you do not set it. You can only choose the client ID when the application is created, so changing it replaces the application.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString(), stringplanmodifier.RequiresReplaceIf(changedFromKnown, "", "")},
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Client secret. Generated when not set (empty for public clients). WSO2 only accepts a custom secret at creation, so changing it replaces the application.",
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				PlanModifiers:       []planmodifier.String{tfutil.KeepStateString(), stringplanmodifier.RequiresReplaceIf(changedFromKnown, "", "")},
			},
			"state": tfutil.ComputedString("`ACTIVE` or `REVOKED`."),
			"grant_types": schema.SetAttribute{
				MarkdownDescription: "Allowed grant types: `authorization_code`, `implicit`, `password`, `client_credentials`, `refresh_token`, `urn:ietf:params:oauth:grant-type:device_code`, `urn:ietf:params:oauth:grant-type:token-exchange`, `urn:ietf:params:oauth:grant-type:jwt-bearer`, `urn:ietf:params:oauth:grant-type:saml2-bearer`, `urn:openid:params:grant-type:ciba`, `organization_switch`, `iwa:ntlm`, `account_switch`.",
				Required:            true,
				ElementType:         types.StringType,
				Validators:          []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"callback_urls":   tfutil.StringList("Authorized redirect URIs. A single value may be a regular expression prefixed with `regexp=`, for example `regexp=(https://a.example.com/cb|https://b.example.com/cb)`."),
			"allowed_origins": tfutil.StringSet("Origins allowed to call the OAuth2 endpoints from browsers (CORS)."),
			"public_client":   optBool("Allow the client to authenticate without a secret (single page and mobile apps). WSO2 still generates a client secret, which public clients do not use.", false),
			"pkce": tfutil.Object("PKCE settings.", map[string]schema.Attribute{
				"mandatory":                         optBool("Require PKCE.", false),
				"support_plain_transform_algorithm": optBool("Accept the `plain` code challenge method.", false),
			}),
			"access_token": tfutil.Object("Access token settings.", map[string]schema.Attribute{
				"type":                                oc("Token format: `Default` (opaque) or `JWT`."),
				"user_access_token_expiry_in_seconds": ocInt("Lifetime of tokens issued to users."),
				"application_access_token_expiry_in_seconds": ocInt("Lifetime of tokens issued to the application (client credentials)."),
				"binding_type": oc("Token binding: `None`, `cookie`, `sso-session`, `certificate`, `DPoP`, `client-request`."),
				"revoke_tokens_when_idp_session_terminated": optBool("Revoke tokens when the user's login session ends.", false),
				"validate_token_binding":                    optBool("Validate the token binding on API calls.", false),
				"access_token_attributes":                   tfutil.StringSet("User attributes (claim names) included in JWT access tokens."),
				"enable_jwt_scope_as_array":                 optBool("Send the `scope` claim of JWT access tokens as an array.", false),
			}),
			"refresh_token": tfutil.Object("Refresh token settings.", map[string]schema.Attribute{
				"expiry_in_seconds":   ocInt("Refresh token lifetime."),
				"renew_refresh_token": ocBool("Issue a new refresh token on every refresh."),
				"extend_renewed_refresh_token_expiry_time": schema.BoolAttribute{
					MarkdownDescription: "Give renewed refresh tokens a full new lifetime. When not set, the server derives it from `renew_refresh_token`.",
					Optional:            true,
					Computed:            true,
					PlanModifiers:       []planmodifier.Bool{tfutil.BoolStateUnlessChanged(path.MatchRelative().AtParent().AtName("renew_refresh_token"))},
				},
			}),
			"id_token": tfutil.Object("ID token settings.", map[string]schema.Attribute{
				"expiry_in_seconds":   ocInt("ID token lifetime."),
				"audience":            tfutil.StringSet("Additional audiences."),
				"signed_response_alg": oc("Signing algorithm, for example `RS256` or `PS256`."),
				"encryption": tfutil.Object("ID token encryption.", map[string]schema.Attribute{
					"enabled":   optBool("Encrypt ID tokens (needs an application certificate).", false),
					"algorithm": optStr("Key encryption algorithm, for example `RSA-OAEP`.", ""),
					"method":    optStr("Content encryption method, for example `A128GCM`.", ""),
				}),
			}),
			"logout": tfutil.Object("OIDC logout endpoints of the application.", map[string]schema.Attribute{
				"back_channel_logout_url":  optStr("Back-channel logout URL.", ""),
				"front_channel_logout_url": optStr("Front-channel logout URL.", ""),
			}),
			"hybrid_flow": tfutil.Object("Hybrid flow settings.", map[string]schema.Attribute{
				"enabled":       optBool("Allow the hybrid flow.", false),
				"response_type": optStr("Comma separated allowed response types, for example `code id_token`.", ""),
			}),
			"subject_token": tfutil.Object("Subject token issuance for token exchange (impersonation).", map[string]schema.Attribute{
				"enabled": optBool("Issue subject tokens.", false),
				"application_subject_token_expiry_in_seconds": ocInt("Subject token lifetime."),
			}),
			"validate_request_object_signature": optBool("Require signed request objects.", false),
			"scope_validators":                  tfutil.StringSet("Scope validators, for example `Role based scope validator`."),
			"client_authentication": tfutil.Object("Token endpoint client authentication.", map[string]schema.Attribute{
				"token_endpoint_auth_method":             optStr("`client_secret_basic`, `client_secret_post`, `private_key_jwt`, `tls_client_auth`. Empty means any method.", ""),
				"token_endpoint_auth_signing_alg":        optStr("Signing algorithm for `private_key_jwt`.", ""),
				"token_endpoint_allow_reuse_pvt_key_jwt": optBool("Allow reusing a private key JWT (only with `private_key_jwt`).", false),
				"tls_client_auth_subject_dn":             optStr("Expected certificate subject DN for `tls_client_auth`.", ""),
			}),
			"request_object": tfutil.Object("Request object settings.", map[string]schema.Attribute{
				"signing_alg":          optStr("Expected request object signing algorithm.", ""),
				"encryption_algorithm": optStr("Request object key encryption algorithm.", ""),
				"encryption_method":    optStr("Request object content encryption method.", ""),
			}),
			"require_pushed_authorization_requests": optBool("Only accept authorization requests sent through PAR.", false),
			"subject": tfutil.Object("Subject identifier type.", map[string]schema.Attribute{
				"subject_type":          optStr("`public` or `pairwise`.", "public"),
				"sector_identifier_uri": optStr("Sector identifier URI for pairwise subjects.", ""),
			}),
			"is_fapi_application": schema.BoolAttribute{
				MarkdownDescription: "Make the application FAPI conformant. The server only accepts this value at creation.",
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{tfutil.KeepStateBool(), boolplanmodifier.RequiresReplace()},
			},
			"token_issuer_organization_id": tfutil.OCString("The organization whose issuer signs the tokens. This setting only applies to applications inside an organization, that is, with a provider that sets `organization_id`. By default, the root organization signs the tokens. To make the application's own organization the issuer, set its organization ID. In the root organization, the value is `\"\"`."),
			"ciba": tfutil.Object("Client Initiated Backchannel Authentication settings.", map[string]schema.Attribute{
				"auth_request_expiry_time": ocInt("Authentication request expiry in seconds."),
				"notification_channels":    tfutil.StringSet("Allowed notification channels, for example `email`, `sms`, `external`."),
				"skip_user_validation":     optBool("Skip validating that the authenticated user matches `login_hint`.", false),
				"allow_federated_users":    optBool("Allow notifications to users not in the local user store.", false),
			}),
		},
	}
}

func samlSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "SAML 2.0 web SSO configuration. Either configure it manually (`issuer` and `assertion_consumer_urls`) or point to the service provider metadata with `metadata_url`. Remove the attribute to remove the protocol.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"metadata_url": schema.StringAttribute{
				MarkdownDescription: "Service provider metadata URL. The server reads issuer, ACS URLs and certificates from it when the protocol is configured.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AtLeastOneOf(path.MatchRelative().AtParent().AtName("issuer")),
				},
			},
			"issuer":                         oc("SAML issuer (entity ID) of the service provider."),
			"service_provider_qualifier":     oc("Qualifier to allow several service providers with the same issuer."),
			"assertion_consumer_urls":        tfutil.OCStringList("Assertion consumer service URLs."),
			"default_assertion_consumer_url": oc("Default ACS URL. The default is the first of `assertion_consumer_urls`."),
			"idp_entity_id_alias":            oc("IdP entity ID used for this service provider. The default is the resident IdP entity ID."),
			"single_sign_on_profile": tfutil.Object("SSO profile.", map[string]schema.Attribute{
				"bindings": tfutil.OCStringSet("Allowed bindings: `HTTP_POST`, `HTTP_REDIRECT`, `ARTIFACT`."),
				"enable_signature_validation_for_artifact_binding": optBool("Validate signatures of artifact resolve requests.", false),
				"enable_idp_initiated_single_sign_on":              optBool("Allow IdP initiated SSO.", false),
				"attribute_consuming_service_index":                tfutil.ComputedString("Attribute consuming service index assigned by the server."),
				"assertion": tfutil.Object("Assertion settings.", map[string]schema.Attribute{
					"name_id_format":   oc("NameID format, for example `urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress`."),
					"audiences":        tfutil.StringSet("Additional audiences."),
					"recipients":       tfutil.StringSet("Additional recipients."),
					"digest_algorithm": oc("Digest algorithm URI."),
					"encryption": tfutil.Object("Assertion encryption.", map[string]schema.Attribute{
						"enabled":                        optBool("Encrypt assertions (needs the service provider certificate).", false),
						"assertion_encryption_algorithm": oc("Assertion encryption algorithm URI."),
						"key_encryption_algorithm":       oc("Key encryption algorithm URI."),
					}),
				}),
			}),
			"attribute_profile": tfutil.Object("Attribute statement settings.", map[string]schema.Attribute{
				"enabled":                               optBool("Include an attribute statement.", false),
				"always_include_attributes_in_response": optBool("Include attributes even when not requested.", false),
				"name_format":                           oc("Attribute name format URI."),
			}),
			"single_logout_profile": tfutil.Object("Single logout settings.", map[string]schema.Attribute{
				"enabled":             optBool("Enable single logout.", false),
				"logout_request_url":  optStr("URL that accepts logout requests.", ""),
				"logout_response_url": optStr("URL that accepts logout responses.", ""),
				"logout_method":       oc("`BACKCHANNEL`, `FRONTCHANNEL_HTTP_REDIRECT` or `FRONTCHANNEL_HTTP_POST`."),
				"idp_initiated_single_logout": tfutil.Object("IdP initiated single logout.", map[string]schema.Attribute{
					"enabled":        optBool("Allow IdP initiated logout.", false),
					"return_to_urls": tfutil.StringSet("Allowed return URLs."),
				}),
			}),
			"request_validation": tfutil.Object("Authentication request validation.", map[string]schema.Attribute{
				"enable_signature_validation":     ocBool("Require signed authentication requests."),
				"signature_validation_cert_alias": oc("Alias of the certificate used to validate request signatures."),
			}),
			"response_signing": tfutil.Object("Response signing.", map[string]schema.Attribute{
				"enabled":           ocBool("Sign SAML responses."),
				"signing_algorithm": oc("Signing algorithm URI."),
			}),
			"enable_assertion_query_profile": optBool("Enable the assertion query/request profile.", false),
		},
	}
}

func passiveSTSSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "WS-Federation (passive STS) configuration. Remove the attribute to remove the protocol.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"realm":           schema.StringAttribute{MarkdownDescription: "Realm (wtrealm) of the relying party.", Required: true},
			"reply_to":        schema.StringAttribute{MarkdownDescription: "URL tokens are posted to.", Required: true},
			"reply_to_logout": oc("URL users return to after logout."),
		},
	}
}

func wsTrustSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: "WS-Trust configuration (needs the WS-Trust connector installed on the server). Remove the attribute to remove the protocol.",
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"audience":          schema.StringAttribute{MarkdownDescription: "Audience (endpoint address) of the trusted service.", Required: true},
			"certificate_alias": schema.StringAttribute{MarkdownDescription: "Alias of the service's certificate in the tenant keystore.", Required: true},
		},
	}
}

var _ = int64validator.AtLeast

// authSequenceType plans `type`: an explicit value wins; otherwise USER_DEFINED when steps
// or a script are configured, else the prior value.
func authSequenceType() planmodifier.String {
	return tfutil.StringFunc{
		Desc: "USER_DEFINED when steps or a script are configured, otherwise the prior value.",
		Fn: func(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
			if !req.ConfigValue.IsNull() {
				return
			}
			parent := req.Path.ParentPath()
			steps := tfutil.ConfigIsNull(ctx, req.Config, parent.AtName("steps"), &resp.Diagnostics)
			script := tfutil.ConfigString(ctx, req.Config, parent.AtName("script"), &resp.Diagnostics)
			if !steps || (!script.IsNull() && !script.IsUnknown() && script.ValueString() != "") {
				resp.PlanValue = types.StringValue("USER_DEFINED")
				return
			}
			if !req.StateValue.IsNull() {
				resp.PlanValue = req.StateValue
			}
		},
	}
}

// derivedFromProtocol plans the top-level client_id/issuer, which mirror a protocol
// attribute: unknown when the protocol is added, removed or the mirrored value changes.
func derivedFromProtocol(protocol, field string) planmodifier.String {
	return tfutil.StringFunc{
		Desc: "Mirrors " + protocol + "." + field + ".",
		Fn: func(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
			if req.State.Raw.IsNull() || !req.PlanValue.IsUnknown() {
				return
			}
			var planObj, stateObj types.Object
			resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(protocol), &planObj)...)
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(protocol), &stateObj)...)
			if planObj.IsNull() != stateObj.IsNull() {
				return
			}
			if !planObj.IsNull() {
				var pv, sv types.String
				resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(protocol).AtName(field), &pv)...)
				resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(protocol).AtName(field), &sv)...)
				if tfutil.Known(pv) && !pv.Equal(sv) {
					return
				}
			}
			resp.PlanValue = req.StateValue
		},
	}
}
