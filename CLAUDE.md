# terraform-provider-wso2is

Terraform provider for WSO2 Identity Server 7.3, hand-written on terraform-plugin-framework.
Read CONTRIBUTING.md for the full conventions; this file is the short version for agents.

## Every session

1. **Start:** read [HANDOFF.md](HANDOFF.md). It says what the last session did, what is open and
   how the test environment is set up. Use the `session-handoff` skill.
2. **End:** update HANDOFF.md before you stop, even when the work is unfinished.

## Commands

Use Task, not make or ad-hoc scripts (`task --list`):

- `task build`, `task test`, `task vet`, `task lint`, `task fmt`
- `task wso2:up` then `task testacc:local -- -run TestAccX ./internal/service/<pkg>/`
- `task testacc -- ...` runs against `WSO2IS_BASE_URL` (or the lab in `.env`); add `TF_CLI=tofu`
  before `--` to use OpenTofu
- `task docs` after any schema, example or template change; `task examples:validate`
- `task sweep` removes `tfacc` leftovers from the test server
- `task lab:up` / `task lab:destroy` create and delete the Hetzner lab (deploy/README.md)

## Where things are

- `internal/provider/registry.go`: every resource and data source is registered here
- `internal/service/<area>/`: resources, data sources and their acceptance tests
- `internal/client/`: HTTP client, auth, errors, `MergeJSON`/`DiffJSON`/`CloneJSON`, `FindByEq`
- `internal/tfutil/`: schema helpers (`OC*`, defaults, `tfutil.Object`), plan modifiers
- `contributing/wso2-api-behaviour.md`: every known server quirk and how it is handled
- `contributing/docs-style.md`: how to write docs and schema descriptions (9th-grade English)
- `examples/` (used by docs and smoke tests), `templates/` (doc templates and guides)
- `deploy/hetzner` (lab server, Terraform), `deploy/k8s` (WSO2 on Kubernetes), `RELEASING.md`
- `_debug/` is git-ignored scratch space for local experiments

## WSO2 references

- Docs: https://is.docs.wso2.com/en/latest/ (source: https://github.com/wso2/docs-is)
- Product source and releases: https://github.com/wso2/product-is
- REST API specs and REST layer: https://github.com/wso2/identity-api-server
- Business logic: https://github.com/wso2/carbon-identity-framework and github.com/wso2-extensions
- Console and login pages: https://github.com/wso2/identity-apps

## Rules that come from real failures

- Explore an API with curl against a real server before writing code; the OpenAPI specs are
  incomplete and sometimes wrong. Read the server source when behaviour is unclear.
- PATCH endpoints replace nested sections wholesale: send a section only if changed, and
  complete. PUT endpoints: read-modify-write with `DiffJSON` + `MergeJSON` over the GET result.
- Never send fields the configuration did not set when the server may not know them (older
  servers reject unknown fields with HTTP 400 `UE-10000`).
- No defaults or computed attributes inside set elements; normalize in `read` instead.
- Server-decided values: `tfutil.OC*`; fixed server defaults: `tfutil.Bool/String/...`;
  derived values: `*StateUnlessChanged` plan modifiers.
- Tenant-wide settings take `client.LockTenantConfig()` for reads and writes.
- A missing object must read as "not found", even when the server answers 400 or 200 for it
  (see `client.ApplicationMissing`); add new resources to `internal/provider/notfound_test.go`.
- Name lookups go through `client.FindByEq` (quoted values; names with "and"/"or" break filters).
- Tests that change tenant-wide behaviour run in `acctest.TestTenant`; tests with several
  provider blocks use `acctest.ExternalProviders`.
- A change is done when its acceptance test passes against a real server, `task lint` is
  clean, docs are regenerated and any new quirk is recorded.
