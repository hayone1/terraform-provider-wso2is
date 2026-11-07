# WSO2 Identity Server API behaviour

Behaviour of the WSO2 Identity Server 7.x management APIs that the provider has to work
around. Each entry says what the server does, how it was observed, and what the provider
does about it. Entries were verified against WSO2 IS 7.3.0 unless noted.

Contributors: when you find a new quirk, add it here with the request that shows it.

## Applications (`/api/server/v1/applications`)

| Behaviour | Provider handling |
|-----------|-------------------|
| `PATCH /applications/{id}` replaces each nested section it receives as a whole. A `claimConfiguration` with only `subject` drops the requested claims; an `authenticationSequence` with only `subjectStepId` resets the flow to `DEFAULT`. | A section is sent only when something in it changed, and then complete. |
| `GET` reports `http://wso2.org/claims/role` as the default role claim, but `PATCH` rejects that URI ("Local claim ... is not available"); 7.x uses `.../roles`. | The role claim is only sent when it is set in the configuration. |
| `PATCH` with `"script": ""` does not remove the adaptive authentication script. | Removing the script calls `PUT /applications/{id}/authenticationSequence/script` with an empty script. |
| `PATCH` with `"logoutReturnUrl": ""` stores `.*` (any URL). | `.*` is shown as unset unless the configuration sets `.*`. |
| The create (`POST`) request fails with HTTP 500 (`NullPointerException`) when an OIDC nested object is partial, e.g. `accessToken` without `userAccessTokenExpiryInSeconds`. | Applications are created with the basic protocol settings; the rest is applied with a read-modify-write `PUT` over the server defaults. |
| Several fields can only be set at creation: `isManagementApp`, `isB2BSelfServiceApp`, the OIDC client ID, a custom client secret ("Invalid ClientSecret provided for update") and the FAPI flag. | These attributes force replacement when changed. |
| Multiple inbound protocols can be configured on one application (OIDC + SAML + WS-Federation). The list in `GET` uses the types `oauth2`, `samlsso`, `passivests`, `wstrust`. | All protocols are supported side by side. |
| Endpoints below a deleted application do not answer 404. `GET /applications/{id}/authorized-apis` and `DELETE .../authorized-apis/{apiId}` answer 400 `APP-60001` ("Application does not belong to the tenant domain"); `GET /applications/{id}/share` answers 200 with `{}`; `DELETE /applications/{id}/shared-apps` answers 400 `ORG-60035` ("Invalid application"). Found 2026-11-07 by importing a missing ID. | `client.ApplicationMissing` confirms with `GET /applications/{id}`; `wso2is_application_authorized_api` and `wso2is_application_share` then treat the object as gone (removed from state on read, success on delete). |
| After an application is deleted, `GET /applications/{id}` keeps answering 200 with the old application for about 15 minutes (a cache), while the list no longer has it and the endpoints below it answer 400 `APP-60001` ("does not belong to the tenant domain"). Found 2026-11-07: deleted 21:22:23, 404 from 21:37:49. | `client.ApplicationMissing` asks the `authorized-apis` endpoint when `GET` still finds the application; `client.ApplicationGone` first checks the sub-endpoint's own answer (`APP-60001` "does not belong to the tenant domain", `ORG-60035`). Before this, deleting an authorization or a share right after its application failed for 15 minutes. |
| `DELETE /applications/{id}/authorized-apis/{apiId}` for an API resource that no longer exists answers 500 `SE-50000`. | Delete re-reads the authorizations; when the API is no longer listed, the delete counts as done. |
| The list filter (`filter=name eq Sales and Marketing`) is parsed word by word: a value that contains `and` or `or` is a syntax error (400). A quoted value (`name eq "Sales and Marketing"`) works. An escaped quote inside the value matches nothing. The same holds for identity providers, API resources and organizations. | Lookups use `client.FindByEq`, which quotes the value, falls back to the full list for values with `"` or `\`, and compares results exactly. |

## OpenID Connect inbound protocol (`/applications/{id}/inbound-protocols/oidc`)

| Behaviour | Provider handling |
|-----------|-------------------|
| `PUT` needs `clientId` ("Invalid ClientID provided for update"). | Updates read the current configuration and send it back with the changes. |
| `PUT` fails with HTTP 500 (`NullPointerException`) when `allowedOrigins` (or other lists) are missing. | Updates always send the full object; lists are never omitted. |
| `PUT` without `clientSecret` keeps the secret. | The secret is never sent in updates. |
| `bindingType` is omitted from responses when it is `None`, and `PUT` rejects `""` ("'' binding type is not allowed"). | Only changed values are sent; an absent binding type is kept as configured. |
| After an update, an unset `hybridFlow.responseType` is stored as the string `"null"`. | `"null"` is read as an empty value. |
| Enabling `renewRefreshToken` also turns on `extendRenewedRefreshTokenExpiryTime` (server default). | `extend_renewed_refresh_token_expiry_time` is planned as unknown when `renew_refresh_token` changes and it is not set. |
| Public clients (`publicClient: true`) still get a generated client secret. | The secret is exposed; it is not used by public clients. |
| With the `LOCAL` claim dialect the server derives `claimMappings` from `requestedClaims`. | `claim_mappings` is planned as unknown when `requested_claims` changes. |

## SAML inbound protocol (`/applications/{id}/inbound-protocols/saml`)

| Behaviour | Provider handling |
|-----------|-------------------|
| `nameIdFormat` is returned with `/` in place of `:` (for example `urn/oasis/names/tc/SAML/1.1/nameid-format/unspecified`). | Values are compared with `/` and `:` treated as equal and shown in the configured (or standard URN) form. |
| `GET` returns the service provider configuration flat, while `PUT` takes it under `manualConfiguration` (or `metadataURL` / `metadataFile`). | Handled in the client. |

## Client credentials grant

| Behaviour | Provider handling |
|-----------|-------------------|
| Requesting `scope=SYSTEM` returns every internal scope the application is authorized for; a token request without scopes returns a token with no scopes. | The provider requests `SYSTEM` by default and reports a clear error when the token has no scopes. |

## API resources (`/api/server/v1/api-resources`)

| Behaviour | Provider handling |
|-----------|-------------------|
| `requiresAuthorization` cannot be changed after creation (`PATCH` rejects it). | Changing it replaces the API resource. |
| `PUT /api-resources/{id}/scopes` replaces the whole scope set, including display name changes. | Scope changes are applied with one `PUT`. |
| Scope names are unique per tenant (HTTP 409 across API resources). | Documented. |
| Authorizing an application for the User Sharing API (`/o/api/server/v1/users`) also authorizes it for the User Sharing API V2 (`/o/api/server/v2/users`). A later `POST /applications/{id}/authorized-apis` for version 2 answers 409 `APP-60509` ("API resource already authorized"). Found 2026-11-07 by authorizing the organization APIs one by one on 7.3.0. | Documented; `wso2is_application_authorized_api` for version 2 needs an import (ID `<application_id>/<api_resource_id>`). `acctest.TestOrganization` ignores conflicts. |

## Users, groups and roles (SCIM 2.0)

| Behaviour | Provider handling |
|-----------|-------------------|
| `PUT /scim2/v2/Roles/{id}` drops every user and group assignment of the role. | Roles are only updated with `PATCH`. |
| Creating a role with `audience.type = organization` but no `audience.value` fails ("Given Organization id: null is invalid"); omitting the audience creates an organization role. | The audience is only sent for application roles. |
| Creating an application-audience role adds it to the application's `associatedRoles`. | `associated_roles.role_ids` is read-only for `APPLICATION` audience. |
| Adding a group member without `display` (the username) fails with `invalidSyntax`. | The provider looks up each new member's username. |
| `PUT /scim2/Groups/{id}` without `members` drops the members. | Groups are renamed with `PATCH`. |
| A SCIM `remove` on a core sub-attribute (`name.familyName`) returns 200 but changes nothing; replacing it with `""` clears it. `remove` works for extension attributes (`urn:...`). | Core attributes are cleared with an empty value, extension attributes with `remove`. |
| Several untyped entries in `emails` collapse into one; typed entries (`work`, `home`) map to separate claims. | `email` (primary) and `emails` (by type) attributes. |
| Users in the primary user store have no domain prefix in `userName`; others are `DOMAIN/name`. | `user_store` attribute. |
| `GET /scim2/v2/Roles?filter=displayName eq "Read and Write"` answers 400 "Given filter operator is not supported", quoted or not. Names with spaces alone work. | `client.FindByEq` falls back to listing every role on 400 and matches the name locally. |
| Group names must match `^[\S]{3,30}$` (default user store setting): no spaces, 3 to 30 characters. The server answers 400 with that pattern in the message. | Not validated by the provider (the pattern is configurable per user store); documented in the server behaviour guide. |

## Identity providers (`/api/server/v1/identity-providers`)

| Behaviour | Provider handling |
|-----------|-------------------|
| `PATCH` needs `ADD` for a field that is unset and `REPLACE` for one that is set; `REMOVE` clears it. `templateId` cannot be patched. | The operation is chosen from the prior value; `template_id` forces replacement. |
| Certificates are sent and returned as base64-encoded PEM; a certificate and a JWKS URI are mutually exclusive. | `certificates` takes plain PEM; `jwks_uri` conflicts with it. |
| Role mappings whose local role is not written as `Internal/<role name>` are silently dropped (HTTP 200). | `local_role` is validated and the provider fails when the server drops a mapping. |
| Outbound provisioning connector secrets are returned as `********`; federated authenticator secrets are returned as stored. | Masked values keep the configured value. |
| Federated authenticator and connector properties are only returned by the per-authenticator endpoints. | The provider reads each authenticator; only configured property keys are tracked. |
| `isEnabled` is not part of the create model. | Disabled connections are patched after creation. |
| Deleting a connection used by applications fails unless `force=true`. | `force_delete` attribute. |
| The order of authenticators within an authentication step is not preserved (the server reorders them). | `authentication_sequence.steps[*].options` is a set. |
| Custom authenticators (`POST /authenticators/custom`): `GET /configs/authenticators/{id}` does not return `authenticationType`. The server adds the tag `2FA` to `VERIFICATION` authenticators; `IDENTIFICATION` authenticators have only the tag `Custom`. Found 2026-11-07. | When the state has no `authentication_type` (after an import, or in Crossplane), Read derives it from the tags. A known value is never overwritten. |

## Claims (`/api/server/v1/claim-dialects`)

| Behaviour | Provider handling |
|-----------|-------------------|
| Claim and dialect IDs are the unpadded base64url encoding of their URIs (the local dialect is `local`). | IDs are derived; imports accept URIs. |
| The custom SCIM dialect `urn:scim:schemas:extension:custom:User` is listed by `/scim2/Schemas` but does not exist as a claim dialect until created. | `wso2is_external_claim` creates it on demand and leaves it on destroy. |
| The server adds an `ExcludedUserStores` property to every local claim. | Exposed as `excluded_user_stores`; other properties are tracked only when configured. |

## Governance connectors and validation rules

| Behaviour | Provider handling |
|-----------|-------------------|
| A `GET` right after a successful `PATCH` occasionally returns the previous property values (seen once in a full regression run; not reproducible on demand). | After an update the provider re-reads for up to 15 seconds until the written values are reported. |
| Governance connector properties of a tenant are stored in one record (the resident identity provider). Concurrent `PATCH` requests to different connectors overwrite each other's changes. | Updates of tenant-wide settings are serialized per server and tenant inside the provider. |
| `POST /identity-governance/{category}/connectors/revert` needs the `connectorId` query parameter (not mentioned in the request schema) or fails with "Property cannot be null". | The client sends it. |
| Connector and category IDs are the base64url encoding of their names. | Connectors are referenced by name. |
| Reads of governance connectors or resident-IdP-backed configs during another update of the resident identity provider can fail (`CNF-65011` "Unable to find the federated authenticator config for openidconnect in the resident IDP") or return half-written data. | Reads take the same per-tenant lock as updates. |
| `JsRegExValidator` (a regular expression) is rejected for the `username` field: `IVM-60022` "The JsRegExValidator validator does not support for username field". | `wso2is_validation_rules` reports it at plan time. |
| `GET /validation-rules/{field}` returns the rules in a different order than they were sent. | `rules` is a set. |

## User stores (`/api/server/v1/userstores`)

| Behaviour | Provider handling |
|-----------|-------------------|
| Create, update and delete are applied asynchronously: right after the request the old state is still served. The user store is deployed to the user realm by a file poller about 10–17 s later; the API returns 404 (and SCIM rejects the domain) until then, and serves the new configuration of an update only once the realm has reloaded it. | The provider polls until the change is visible (up to 2 minutes), so users can be created right after `wso2is_userstore`. |
| Confidential properties (passwords) are returned encrypted. | `secret_properties` keep the configured values; changes made outside Terraform are not detected. |
| WSO2 IS 7.3 ships a built-in `AGENT` user store for AI agent identities. | Not managed by `wso2is_userstore`. |

## Tenants (`/api/server/v1/tenants`)

| Behaviour | Provider handling |
|-----------|-------------------|
| After a tenant is deleted, a new tenant with the same domain is created fine but rejects its owner's API calls with HTTP 400 for about 15 minutes (log: "Error while retrieving role id list of user"): caches of the deleted tenant (default timeout 15 min). A fresh domain is usable at once. | `wso2is_tenant` waits up to 2 minutes for the owner to reach the APIs, then warns. |
| A failed tenant creation (`TM-65002`) leaves a deactivated tenant that blocks the domain (`TM-60009`) and is hidden from `GET /tenants`. | `wso2is_tenant` deletes it. |
| Creating a tenant with an OAuth2 client-credentials token fails with HTTP 500 `TM-65002` "Error in adding claims to the user": the provisioning listener looks up the calling application in the new tenant ("Invalid service provider name"). Basic authentication works. | Documented; `wso2is_tenant` adds a hint to the error. |
| A missing tenant is reported as HTTP 400 `TM-60014` ("Tenant cannot be found"), not 404. | Treated as not found. |
| `DELETE /tenants/{id}/metadata` fails with `TM-60016` unless `[tenant_mgt] tenant_deletion = true` is set in `deployment.toml`; tenants must be deactivated first. | Destroy deactivates, then deletes; without deletion enabled it leaves the tenant deactivated and warns. |
| A new tenant takes a few seconds before its APIs answer. | Documented; tests wait for it. |

## Notification senders and templates

| Behaviour | Provider handling |
|-----------|-------------------|
| Push senders (7.3): the sender name is always `<provider>_PushPublisher` whatever name is sent. FCM wants the service account JSON base64-encoded in `serviceAccountContent` (the spec's `fcm.serviceAccount` fails with `NSM-65018`); SNS takes `accessKeyId`, `secretAccessKey`, `region` and `platformArns` (a JSON object as a string). Updates must send every property (a partial `PUT` fails with HTTP 500). Secrets are returned in plain text. The default provider is set through `PUT /api/server/v1/notification-senders/configs/push` (v1 only) and outlives a deleted sender. | `wso2is_push_sender` encodes the service account, sends complete updates and clears the default on destroy. |
| WSO2 IS 7.3 requires `authType` on email senders and earlier versions reject it. | `wso2is_email_sender` needs 7.3. |
| The super tenant's email provider cannot be managed through the API (HTTP 405); it is read from `[output_adapter.email]` in `deployment.toml`. | `wso2is_email_sender` explains this when it gets 405. |
| SMTP senders need the properties `mail.smtp.signature` (display name) and `mail.smtp.replyTo`, and `BASIC` auth needs `userName`/`password` properties; otherwise the server answers `NSM-65022` "Invalid input" without saying what is missing. | First-class `display_name`, `reply_to`, `username`, `password` attributes. |
| The sender password is never returned. With BASIC authentication the v2 SMS sender response copies the username and password into `key` and `secret` (in plain text). | Write-only values are kept from the configuration. |
| Template type IDs are the base64url encoding of the type name. A system template exists for every type in `en_US`; creating an organization template for a locale that already has one fails with 409. | The provider takes over an existing template on 409. |

## Organizations (`/api/server/v1/organizations`)

| Behaviour | Provider handling |
|-----------|-------------------|
| Application sharing is applied in the background: on a fresh server `GET /applications/{id}/share` lists the organization only a few seconds after `POST .../share` returns. | `wso2is_application_share` waits until the shared organizations match the configuration. |
| WSO2 IS 7.2.0: creating two organizations concurrently on a server that has never had one fails once with HTTP 500 `ORG-65090` ("Error while firing the events"; log: unique key violation on the `SSO` identity provider). | Documented; a second apply succeeds. |
| Applications inside organizations choose the token issuer with `inboundProtocolConfiguration.oidc.issuer.organizationId` (7.3): the root organization's issuer by default, or the organization's own. Root applications report `issuer: null`. | `oidc.token_issuer_organization_id` (`""` in the root organization). |
| Email domain discovery attributes (`POST /organizations/discovery`) are rejected with `ORG-60080` until `/organization-configs/discovery` has `emailDomain.enable = true`. The configuration has no record until first created (`OCM-60002`); keys other than `emailDomain.enable`, `emailDomainBasedSelfSignup.enable` and `defaultParam` (`orgHandle`/`orgName`) are rejected. | The resource explains `ORG-60080`; the configuration resource creates or takes over the record and deletes it on destroy. |
| User sharing (7.3) lives under `/api/server/v2/users/share*` (the v1 paths answer 400/403). Requests return 202 and are processed in the background; roles the server cannot resolve are skipped silently. `GET /users/{id}/share?attributes=roles,sharingMode&recursive=true` reports per-organization roles, and the policy at the top level (share with all) or per organization (with children). Re-sharing an organization replaces its roles. | `wso2is_user_share` waits for the expected state and explains a timeout. |
| The `Location` header of a created organization is malformed (`/api/serverv1/organizations/{id}`). | The ID is read from the body or the last path segment. |
| A deleted organization is reported as HTTP 400 `ORG-60064` ("can only be managed from an ancestor"), not 404. | Treated as not found. |
| An organization can only be created from its parent: creating one with `parentId` set to a sub-organization from the root context fails with 403 `ORG-60021`. | `parent_id` is read-only; nested organizations are created with a provider alias scoped to the parent (`organization_id`). |
| Organization APIs (`/t/{tenant}/o/...`) reject basic authentication (403); they need an organization-switched token. `organization_switch` works with tokens from the client credentials grant, provided the application is shared with the organization and authorized for the organization APIs. | `organization_id` provider mode uses client credentials + organization switch. |
| Sharing an application with organizations sets its `claimConfiguration.subject.useMappedLocalSubject` to `true`. | `use_mapped_local_subject` is server-managed. |

## Flows (`/api/server/v1/flow`)

| Behaviour | Provider handling |
|-----------|-------------------|
| Flow types in 7.3 are `REGISTRATION`, `PASSWORD_RECOVERY` and `INVITED_USER_REGISTRATION` (the spec's `ASK_PASSWORD` is rejected). No flow is stored until one is saved (`GET` returns `{}`). | `flow_type` validation; an empty flow is treated as absent. |
| The flow is stored as sent except that numbers become floating point and empty arrays (for example `"components": []`) are dropped. | Steps are compared after the same normalization. |
| `PUT` with empty steps fails (`FM-10012`). | — |
| `DELETE /api/server/v1/flow` is always forbidden (403) in the root organization: `resource-access-control-v2.xml` has rules for GET and PUT/PATCH but none for DELETE (only `/o/` flows have one). | Destroy disables the flow and warns; the guide shows the `[[resource.access_control]]` override (scope `internal_flow_update`) that allows DELETE, verified on 7.3.0 and enabled in `deploy/`. |
| `/flow/execute` is unauthenticated (it is what the hosted sign-up and recovery pages call). | Used by the acceptance tests to sign up through a custom flow. |

## Actions (`/api/server/v1/actions`)

| Behaviour | Provider handling |
|-----------|-------------------|
| Actions are created `INACTIVE`. | The provider activates them unless `active = false`. |
| One action per type per tenant by default (`ACTION-60002`). | Documented. |
| `allowedHeaders` / `allowedParameters` are rejected for password and profile actions even when empty (`ACTION-60016`). | Only sent when set. |
| Endpoint authentication properties are write-only. | Kept from the configuration. |

## Server configurations (`/api/server/v1/configs`)

| Behaviour | Provider handling |
|-----------|-------------------|
| A non-empty inbound provisioning user store (`/configs/provisioning/inbound/scim` `provisioningUserstore` for basic-auth requests, or the calling application's own inbound provisioning user store for OAuth2 tokens) makes SCIM put **every** new user into that store, silently stripping the `DOMAIN/` prefix (`SCIMUserManager.createUser`), and fails SCIM updates of users in other stores. There is no revert endpoint. | `wso2is_inbound_provisioning_configuration` clears the store on destroy; `wso2is_user` detects a user created in the wrong store, deletes it and explains the cause. |
| `PATCH /configs` only accepts `REPLACE` on `/idleSessionTimeoutPeriod`, `/rememberMePeriod`, `/enableMaximumSessionTimeoutPeriod`, `/maximumSessionTimeoutPeriod`, `/preserveCurrentSessionAtPasswordUpdate`; home realm identifiers are changed one index at a time (`/homeRealmIdentifiers/{i}` with `ADD`/`REMOVE`/`REPLACE`). SCIM inbound provisioning has its own `PUT /configs/provisioning/inbound/scim`. | Separate resources and per-field update strategies. |
| `PATCH /configs/cors` list fields take one string per operation: `REPLACE` sets one value, `ADD`/`REMOVE` change one value, `REPLACE ""` clears. Replacing with `""` and then adding leaves an empty element. Order is not preserved. | `REPLACE` the first value, `ADD` the rest; lists are sets. |
| Impersonation and JWT key validator patches take boolean values; DCR and CORS take strings; the inbound SAML/passive STS/OAuth2 configs take a partial object. | Per-resource update style. |
| `PUT /configs/remote-logging/{log-type}` wants the log type in lower case (`audit`/`carbon`) and rejects a `logType` field in the body, although `GET` reports it in upper case. | Handled in the client. |

## Webhooks and workflows

| Behaviour | Provider handling |
|-----------|-------------------|
| A webhook's `status` reflects its channel subscriptions (`PARTIALLY_ACTIVE` when some failed); active webhooks must be deactivated before deletion. | `status` is re-read when `active`, `channels` or `endpoint` change; destroy deactivates first. |
| Creating a workflow association with `isEnabled: false` creates it enabled. | Disabled associations are patched after creation. |
| The last association of a workflow cannot be deleted (`WF-51005`); deleting the workflow removes its associations. Only one association per operation and workflow. | Destroy disables such an association and warns; it goes away with the workflow. |
| While a workflow is associated with an operation, that operation is queued for approval (for example SCIM user creation returns 202 and the user does not exist yet). | Documented on `wso2is_workflow`. |

## Verifiable credential templates (`/api/server/v1/vc-templates`, 7.3)

| Behaviour | Provider handling |
|-----------|-------------------|
| Claim `claimUri` values must be claims of the `http://wso2.org/vc/claim` dialect (for example `given_name`), not local claim URIs as the specification's example shows; anything else fails with `VCM-60009` "Invalid claim". `type` must be `LOCAL`. | `claims` maps credential claim names to VC dialect claims; the error is explained. |
| `description` is accepted on creation but not stored, and rejected on update. `displayName` is required on creation and update. Formats: `jwt_vc_json`, `dc+sd-jwt`. A credential offer is generated on creation. | No `description` attribute; `display_name` required; `offer_id` computed. |
| A missing template is HTTP 404 `VC-60001`. | Treated as not found. |

## Older servers (7.1, 7.2)

| Behaviour | Provider handling |
|-----------|-------------------|
| Fields added in 7.3 are rejected as unknown with HTTP 400 `UE-10000`, even with default values (OIDC `cibaAuthenticationRequest` and `accessToken.enableJwtScopeAsArray`, identity provider `jit.idpGroupSyncMethod`, `implicitAssociation`, `idpGroups`). | These are sent only when set, or when the server reports them. |
| 7.2 returns untyped SCIM multi-valued attributes as plain strings (`"emails": ["a@example.com"]`) where 7.3 returns objects. | Both shapes are read. |
