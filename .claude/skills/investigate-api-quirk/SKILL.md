---
name: investigate-api-quirk
description: Find out why the WSO2 Identity Server API behaves unexpectedly (ignored fields, wrong status codes, undocumented requirements, async effects) and record it. Use when a request fails or the server returns something other than what was sent and the cause is not obvious.
---

# Investigate an API quirk

1. **Capture the exchange.** `TF_LOG=TRACE TF_LOG_PATH=trace.log terraform apply` (or the
   acceptance test) logs each request and response body ("WSO2 request payloads"). Note the
   `traceId` of failing responses.
2. **Reproduce with curl** against a test server, removing everything not needed. Vary one thing
   at a time: drop a field, send it empty, send a default, change the order, quote a filter
   value, use a missing ID.
3. **Read the server log** for the trace ID (`docker logs wso2is-7.3.0 | grep <traceId>`, or
   `task lab:logs`). Follow the `Caused by:` chain; the root cause is usually several levels down.
4. **Read the docs and the source** when behaviour is still unclear:
   - Product docs: https://is.docs.wso2.com/en/latest/ (source: https://github.com/wso2/docs-is,
     search it with `gh search code '<term>' --repo wso2/docs-is`).
   - Product build, release notes and integration tests: https://github.com/wso2/product-is
     (`modules/integration/tests-backend` shows how WSO2 itself calls the APIs).
   - REST layer: https://github.com/wso2/identity-api-server (`components/<api>`), spec and
     `core/` service.
   - Business logic: https://github.com/wso2/carbon-identity-framework, and the repositories under
     https://github.com/wso2-extensions (for example identity-inbound-provisioning-scim2,
     identity-openid4vc). `gh search code '<error code>' --owner wso2-extensions` finds where an
     error code is thrown.
   - Login pages: https://github.com/wso2/identity-apps (`identity-apps-core/apps/authentication-portal`);
     Console requests: `features/admin.*` in the same repository.
5. **Check other versions** if the behaviour may be new (older servers often reject newer fields):
   `task wso2:up WSO2_VERSION=7.2.0` or `task lab:compat`.
6. **Decide the handling**: work around it in the provider (normalize, retry, wait, omit a field,
   split a request, confirm with a second request), or report it clearly (validation at plan
   time, a diagnostic with the fix). If it cannot be fixed, document it.
7. **Record it** in `contributing/wso2-api-behaviour.md` under the API's section: what the server
   does, how it was observed, what the provider does. If users must act on it, add it to
   `templates/guides/server-behaviour.md.tmpl` (use the `write-docs` skill) and run `task docs`.
   Note it under "Known problems" in `HANDOFF.md` if it is not fixed.
