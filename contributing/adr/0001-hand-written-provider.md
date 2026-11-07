# 1. Hand-written provider instead of code generation

Date: 2026-10-31. Status: accepted.

## Context

WSO2 publishes OpenAPI specifications for its management APIs
([wso2/identity-api-server](https://github.com/wso2/identity-api-server)), so generating the
provider was the obvious first option. Two generators were considered:

* HashiCorp's [terraform-plugin-codegen-openapi](https://github.com/hashicorp/terraform-plugin-codegen-openapi)
  with the framework code generator: it turns OpenAPI schemas into framework schemas and model
  types, but leaves CRUD logic to hand-written code, and is a technical preview.
* [OpenAPI Generator](https://openapi-generator.tech/)'s Terraform provider generator: it
  generates resources end to end from request/response schemas.

Exploring the APIs against a running 7.3.0 server showed that the specs describe the wire format
but not how a Terraform resource has to behave:

* **Updates are not symmetric with reads.** Application `PATCH` replaces each nested section it
  receives, OIDC and SAML need a full read-modify-write `PUT`, identity providers split into
  sub-resources with JSON-patch operations whose `ADD`/`REPLACE` choice depends on the prior value,
  server configurations use four different patch styles.
* **The specs are incomplete or wrong in places that break generated code**: flow types listed but
  rejected, VC template claims documented as local claim URIs while the server wants a different
  dialect, the custom SCIM dialect advertised but absent, required properties not marked required
  (email providers), read-only fields that some endpoints reject when sent back.
* **Server behaviour needs handling that has no place in a spec**: asynchronous user stores and
  user sharing, masked or omitted secrets, base64url IDs derived from names, "not found" reported
  as HTTP 400 with product-specific codes, tenant-wide settings that must be serialized.
* **Generated schemas would mirror API shapes** (arrays of `{key, value}` properties, wrapper
  objects, IDs that are encodings of names) instead of what a Terraform user expects (maps, plain
  names, sets for unordered data, write-only secrets, sensible import IDs).

## Decision

Write the provider by hand on terraform-plugin-framework. Use the OpenAPI specs as a reference and
as a checklist for coverage, and verify every resource against a real server with acceptance
tests.

## Consequences

* More code than a generated provider. Shared helpers keep resources small and consistent:
  `internal/client` (auth, retries, error mapping, `MergeJSON`/`DiffJSON`), `internal/tfutil`
  (schema helpers, defaults, plan modifiers) and the acceptance test helpers.
* Server quirks are documented in one place (`contributing/wso2-api-behaviour.md`) and each has a
  test that would catch a regression.
* New WSO2 releases need a manual gap analysis: compare the new specs and release notes with the
  provider's resources, then explore new endpoints with curl before implementing them.
