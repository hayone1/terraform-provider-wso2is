# 2. Supported WSO2 Identity Server versions

Date: 2026-10-31. Status: accepted.

## Context

The provider was built against WSO2 Identity Server 7.3.0, the latest release. Many deployments
still run 7.1 or 7.2. The management APIs evolve between versions mostly by adding fields and
endpoints, and older servers reject request fields they do not know with HTTP 400
(`UE-10000 Provided request body content is not in the expected format`), even when the value is
a default.

The full acceptance suite was run against 7.1.0 and 7.2.0 (official images, embedded H2,
`task lab:compat`). Most failures came from the provider sending 7.3-only fields with their
default values, and a few from differences in responses (7.2 returns untyped SCIM `emails` as
plain strings).

## Decision

* **7.3.x is the supported version.** Every resource is tested against it, in CI and in the lab.
* **Do not send what the configuration does not set** when an older server might not know the
  field: omit fields holding their defaults (identity provider `idpGroupSyncMethod`,
  `implicitAssociation`), or drop them when the server's own GET does not return them (OIDC
  `cibaAuthenticationRequest`, `enableJwtScopeAsArray`). Read responses tolerantly (both SCIM
  shapes). These rules cost little and keep 7.2 working for the common resources.
* **Features introduced in 7.3 are not emulated** on older servers. Using them there fails with the
  server's error. Resources whose API changed incompatibly (the email provider requires `authType`
  in 7.3 and rejects it before) follow 7.3.
* **7.1 is not supported.** Too many APIs differ.

## Consequences

* The compatibility notes in the documentation list what is known to work on 7.2.
* New resources and attributes follow the "omit defaults" rule, which `CLAUDE.md` and
  `CONTRIBUTING.md` spell out.
* Supporting a version means running the suite against it (`task testacc:matrix VERSIONS=...`)
  before claiming it.
