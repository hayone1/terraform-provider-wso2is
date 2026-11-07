---
name: add-resource
description: Add or extend a resource, data source or attribute in terraform-provider-wso2is. Use when asked to support a new WSO2 Identity Server feature or API, add an attribute, or fix how a resource maps to the server.
---

# Add or extend a resource

Work through these steps in order. Each one exists because skipping it caused real bugs.
Read `HANDOFF.md` first if you have not done so in this session.

## 1. Learn the feature and the API from the server, not just the spec

1. Read what the feature does in the product docs: https://is.docs.wso2.com/en/latest/
   (source: https://github.com/wso2/docs-is). Check which release added it in
   https://github.com/wso2/product-is (releases and `modules/integration` tests).
2. Find the endpoint in the OpenAPI spec (`components/<api>/src/main/resources/*.yaml` in
   https://github.com/wso2/identity-api-server; `_debug/ref` may already have a clone).
3. Against a test server (`task wso2:up`, admin/admin on https://localhost:9443, or the lab),
   exercise it with curl: create with minimal and full bodies, GET, update one field, update a
   nested object partially, remove an optional value, delete, and GET a missing ID. Note what a
   missing ID returns: some endpoints answer 400 or 200 instead of 404.
4. Write down what differs from what you sent: dropped fields, renamed or reformatted values,
   defaults the server fills in, reordered lists, error codes for "not found", async behaviour.
   Use the `investigate-api-quirk` skill when something is unclear.

## 2. Design the schema

- Names in snake_case, close to the Console's wording; IDs as `id`, import ID documented.
- Fixed server default: `tfutil.Bool/String/Int64/StringSet/...(desc, default)`; nested blocks
  with defaults: `tfutil.Object`.
- Value the server decides: `tfutil.OCString/OCBool/...` (optional + computed, keeps state).
- Value derived from other attributes: the `*StateUnlessChanged` plan modifiers.
- Lists the server reorders: sets. No defaults or computed attributes inside set elements.
- Secrets the API does not return: keep the configured value; consider a `*_wo` variant.
- Changes the API cannot apply in place: `RequiresReplace`.
- Write descriptions with the `write-docs` skill: what the resource manages, what destroy does,
  how to import, in complete, short sentences.

## 3. Implement

- Package `internal/service/<area>/`, register in `internal/provider/registry.go`.
- Updates: PATCH sections only when changed and complete; PUT as read-modify-write
  (`client.DiffJSON(expand(plan), expand(state))` merged over the GET with `client.MergeJSON`;
  snapshot with `client.CloneJSON` before merging if you compare against the original).
- Only send what the configuration sets (older servers reject unknown fields).
- `Read` removes the resource from state when the object is gone, whatever the server answers
  for a missing object (404, or 400/200 below a deleted parent: see `client.ApplicationMissing`).
  `Delete` treats "already gone" as success.
- Lookups by name use `client.FindByEq` and compare results exactly.
- Errors through `tfutil.AddAPIError`; add an actionable diagnostic when the server's message is
  unhelpful.
- Tenant-wide settings: `client.LockTenantConfig()` around reads and writes.

## 4. Test

- Acceptance test in the same package: create, update (including removing optional values),
  `ImportState` + `ImportStateVerify`, destroy (add `CheckDestroy` when destroy restores state).
- Add a case to `TestAccResources_missingObjectIsNotFound` in
  `internal/provider/notfound_test.go` (imports an ID that does not exist).
- Names from `acctest.RandomName` so `task sweep` can clean up.
- Run it: `task testacc:local -- -run TestAccX ./internal/service/<area>/` (or `task testacc`),
  and once with OpenTofu: `task testacc:local TF_CLI=tofu -- -run TestAccX ./internal/service/<area>/`.
- Add a sweeper in `internal/sweep/sweep_test.go` for new object types.

## 5. Document

- `examples/resources/wso2is_<name>/resource.tf` (realistic, self-contained) and `import.sh`.
- `task docs`, `task examples:validate`.
- Every quirk into `contributing/wso2-api-behaviour.md`; user-visible ones also into
  `templates/guides/server-behaviour.md.tmpl`.
- Mention the change in `CHANGELOG.md`, and update `HANDOFF.md` at the end of the session.
