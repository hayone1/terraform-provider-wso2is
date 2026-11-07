# Changelog

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
