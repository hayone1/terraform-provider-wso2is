# Changelog

## Unreleased

BUG FIXES:

* All resources: reading an object with an empty or missing ID now reports the object as missing,
  without calling the server. Terraform never does this, but Crossplane's Upjet reads an object
  with an empty ID before it creates it. Some resources answered with an object (for example
  applications, users and roles), so Crossplane treated the new object as existing and never
  created it. Other resources failed with an error from the server.

## 0.1.1 (2026-11-07)

This release prepares the provider for use inside other programs, such as a Crossplane provider
that Upjet generates. Terraform users get the bug fixes below.

FEATURES:

* New Go package `github.com/hayone1/terraform-provider-wso2is/provider`. Its `New` function
  returns the provider, so other Go programs can run it in their own process.

BUG FIXES:

* `wso2is_branding_preference`, `wso2is_branding_text`, `wso2is_notification_template` and
  `wso2is_secret`: reading failed, or reported the object as missing, when the state had only the
  ID. The attributes that have a default (`type`, `locale` and others) are now taken from the ID
  when they are empty.
* `wso2is_governance_connector`: reading failed when the state had no `category_id`. The provider
  now looks the category up.
* `wso2is_custom_authenticator`: `authentication_type` is now read from the server after an
  import. The server marks `VERIFICATION` authenticators with the tag `2FA`.

IMPROVEMENTS:

* Access tokens are shared by all provider instances in one process that use the same server,
  tenant, organization, client and scopes. HTTP connections are shared by all instances with the
  same TLS settings. A program that creates a new provider instance for each operation (Crossplane
  does) no longer requests a new token and opens new connections every time.
* Tests: every acceptance test now also reads each resource the way Upjet does: from the ID and
  the arguments only. A new test covers `wso2is_oauth2_issuer_usage_scope`.

## 0.1.0 (2026-11-07)

First release, developed and tested against WSO2 Identity Server 7.3.0.

FEATURES:

* Provider: basic authentication, OAuth2 client credentials (with the `SYSTEM` scope), static
  access tokens; tenants (`tenant_domain`) and sub-organizations (`organization_id`, through the
  `organization_switch` grant); custom CA certificates; retries for transient errors; provider
  configuration from values known only at apply time (for example an organization created in the
  same configuration).
* Applications: OIDC, SAML, WS-Federation and WS-Trust, claims, authentication sequences with
  adaptive scripts, provisioning, roles; authorized APIs; sharing with organizations; token issuer
  selection for organization applications.
* Identity: users (write-only passwords, secondary user stores), groups, group members, roles,
  role assignments, secondary user stores (tested with JDBC), user sharing with organizations.
* Attributes: local claims, claim dialects, external claims (including the custom SCIM schema),
  OIDC scopes.
* Connections: identity providers (federated authenticators, JIT provisioning, outbound
  provisioning, claim and role mappings), custom authenticators, identity verification providers.
* B2B: organizations, email domain discovery.
* Flows and extensions: flows (tested with custom registration), actions, webhooks, approval workflows,
  adaptive script libraries and secrets, verifiable credential templates.
* Notifications and branding: email, SMS and push providers, notification templates and types,
  branding preferences and text.
* Settings: governance connectors, validation rules, server, CORS, DCR, OAuth2, SAML, passive
  STS, impersonation, JWT key validation, fraud detection, inbound provisioning, remote logging,
  admin advisory banner; tenants.
* Data sources for applications, users, groups, roles, claims, identity providers, organizations,
  API resources, flows, governance connectors, authenticators and federated authenticator
  metadata.

BUG FIXES (found while preparing the release):

* Data sources: looking up an application, identity provider, role, API resource or organization
  by a name that contains the words "and" or "or" (for example "Sales and Marketing") failed with
  a filter syntax error. Lookups now quote the name, and fall back to a full listing when the
  server cannot filter (the roles API).
* `wso2is_application_authorized_api` and `wso2is_application_share`: when the application was
  deleted outside Terraform, plans failed (HTTP 400 `APP-60001`) or kept the resource. Both
  resources now notice the missing application and plan to create it again. Destroying an API
  authorization whose API resource was already deleted no longer fails with HTTP 500.
* Provider: a very large `max_retries` value made the retry delay overflow.

NOTES:

* Tested with Terraform and OpenTofu against WSO2 Identity Server 7.3.0 (PostgreSQL, Kubernetes)
  and the official Docker image.
