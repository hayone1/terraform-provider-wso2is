---
name: run-acceptance-tests
description: Run and debug the acceptance tests of terraform-provider-wso2is against a real WSO2 Identity Server, locally in Docker or on the Hetzner lab. Use when asked to test a change, reproduce a bug, bring the lab up or down, or when an acceptance test fails with an inconsistent result, a non-empty plan or an API error.
---

# Run and debug acceptance tests

Read `HANDOFF.md` first: it says whether the lab is running and what was last verified.

## Pick a server

- **Local Docker** (anyone): `task wso2:up` (7.3.0, admin/admin, tenant and flow deletion
  enabled), then `task testacc:local -- <go test args>`. Another version: `WSO2_VERSION=7.2.0`.
- **Hetzner lab** (maintainers, needs `.env`): `task lab:up` creates the server and deploys WSO2
  with PostgreSQL behind a Let's Encrypt certificate on `https://$WSO2_HOSTNAME`; then
  `task testacc -- <go test args>`. `task lab:status`, `task lab:logs`, `task lab:destroy`
  (the server costs money while it exists). Details: `deploy/README.md`.
- **Any server**: set `WSO2IS_BASE_URL`, `WSO2IS_USERNAME`, `WSO2IS_PASSWORD` (and
  `WSO2IS_INSECURE_SKIP_VERIFY=true` for self-signed certificates), then `task testacc -- ...`.
- User store tests add users only when `WSO2IS_TEST_JDBC_URL` points at a database the server
  can reach (the lab has one); otherwise they create an H2 store.

Run one test: `task testacc:local -- -run TestAccUserShare ./internal/service/organization/`.
Run everything sequentially (fewer server races): `task testacc -- -p 1 ./...` (about 7 minutes
on the lab). Use OpenTofu instead of Terraform: put `TF_CLI=tofu` before `--`.
Missing-object handling for every resource: `task testacc -- -run TestAccResources_missingObjectIsNotFound ./internal/provider/`.

## Read failures

- **"Provider produced inconsistent result after apply"**: `read` returned something different
  from the plan. Compare with what the server returned (`TF_LOG=TRACE` logs every request and
  response). Usual causes: a server default (use `OC*` or a fixed default), a reformatted value
  (normalize in `read`), null vs empty (keep the prior form), a derived value (plan modifier).
- **"After applying this test step, the plan was not empty"**: a perpetual diff. Look at which
  attribute changes; often a set element default, a reordered list that should be a set, or an
  unknown value from a `KeepState` modifier on an attribute that is null in state.
- **"Cannot import non-existent remote object" expected but another error**: the server answers
  something other than 404 for a missing object. Find out what (curl), and map it to "gone".
- **HTTP 400 `UE-10000`**: the body has a field the server does not know (often a newer-version
  field) or a wrong type. Capture the body with `TF_LOG=TRACE`.
- **HTTP 403**: missing permission or missing access control rule (see the flow DELETE case in
  `contributing/wso2-api-behaviour.md`).
- **Flaky results under parallel runs**: tenant-wide settings changed by another test (use
  `acctest.TestTenant`), or a server race (retry with `-p 1` to confirm, then record it).
- **OpenTofu "Invalid provider namespace"**: run through Task, which sets
  `TF_ACC_PROVIDER_NAMESPACE` and `TF_ACC_PROVIDER_HOST`.

Server logs: `docker logs wso2is-7.3.0` (local) or `task lab:logs -- --since=10m` (lab). Search
for the `traceId` from the error.

## Clean up

`task sweep` deletes everything with `tfacc` in its name on the test server (it asks first).
Do not run it while another test run uses the same server. Record what you verified in
`HANDOFF.md`.
