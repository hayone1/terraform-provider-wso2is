# Handoff

This file is the memory between work sessions. Read it at the start of a session and update it
before you stop (see the `session-handoff` skill). Never put secret values here.

## Last updated

2026-11-08 (fourth session, from the Crossplane provider): **v0.1.2 is released** (tag on
`a1ec1c7`, fast-forward of the branch `fix/read-without-id` into `main`). It contains the four
fixes found by the Crossplane provider: reads without an ID report "not found"
(`tfutil.MissingID`, `internal/provider/emptyid_test.go`); the User Sharing API v2 quirk is
documented; deleted applications that the server still caches are noticed
(`client.ApplicationGone`); and member lists of a group or role that just disappeared delete
cleanly (`internal/provider/deleteparent_test.go`). CI and Acceptance passed on `main` before the
tag, and the Release workflow published 13 archives, the signed checksums and the manifest.

Earlier:

2026-11-07 (second session): v0.1.1 prepares the provider for Crossplane's Upjet. The Hetzner
lab was recreated and is running. The Crossplane provider is a separate project in
`/home/hayone/github/crossplane-provider-wso2is` (start with its `BRIEF.md`).

## State

* 52 resources and 12 data sources. Build, vet, unit tests, `task lint` (0 issues), docs
  validation and `task examples:validate` (Terraform and OpenTofu) pass.
* Full acceptance suite against the Hetzner lab (WSO2 IS 7.3.0, PostgreSQL), Terraform 1.14,
  with the Upjet read check: `task testacc -- -p 1 -count=1 -v ./...` passed on 2026-11-07:
  63 tests and 33 sub-tests in 26 packages, 0 failures, 0 skips (log:
  `_debug/lab-upjet-full.log`). The same suite passed against the local Docker server.
* OpenTofu 1.13: the branding, config, governance, notification, scripting, authenticator,
  organization and provider packages pass against Docker (2026-11-07, second session), and all
  69 examples validate.
* Fresh-copy check (only the files that would be committed, `cp example.env .env`, Docker
  server): build, unit tests, `task wso2:up` and the identity, lookup and not-found acceptance
  tests pass. The Getting started guide applies cleanly (8 objects), the next plan is empty,
  the Step 5 change plans one update, and destroy removes everything. This check found that
  `.gitignore` excluded `internal/service/idp/testdata/cert.pem` (fixed).
* `task release:snapshot` builds all 13 platform archives, the checksums and the manifest.
* Public repository: https://github.com/hayone1/terraform-provider-wso2is. CI and Acceptance
  (Terraform and OpenTofu jobs) pass on `main`. The five Dependabot action bumps are merged.
* **v0.1.2 is released** (2026-11-08, tag on `a1ec1c7`). The Crossplane provider uses it.
* **v0.1.1 is released** (2026-11-07, tag on `1c868f2`): 13 archives, signed checksums and the
  manifest; signature verified with the release key. The Go proxy and registry.terraform.io
  serve it. CI and Acceptance (Terraform and OpenTofu) passed on that commit. v0.1.0 was
  released earlier the same day and is listed on registry.terraform.io as `hayone1/wso2is`. The OpenTofu registry submission
  was pending approval on 2026-11-07 (check https://registry.opentofu.org/v1/providers/hayone1/wso2is/versions).

## Environment

* **Hetzner lab: running** since 2026-11-07 (recreated for the Upjet work, IP 2.31.91.77). It is
  a `cpx32` (4 vCPU, 8 GB, about EUR 42 a month) in `nbg1`, Ubuntu 24.04, k3s, WSO2 at
  <https://wso2is.lab.software-cloud.com>. The Crossplane project uses it for its tests (its
  `.env` has the admin credentials). `task lab:destroy` deletes it; ask the maintainer first.
  `task lab:up -- -auto-approve` recreates it in about ten minutes. Without it, use
  `task wso2:up` (Docker).
* **Secrets** live in `.env` (git-ignored): `HCLOUD_TOKEN`, the `WSO2_*` passwords, the test
  JDBC settings, and the release signing key (`GPG_FINGERPRINT`, `PASSPHRASE`,
  `GPG_PRIVATE_KEY`). The signing key is also in `.secrets/` (git-ignored files: private key, public key, passphrase) and in the maintainer's local GPG keyring.
  `_debug/env.backup-2026-11-07` is a backup of the old `.env` (old lab keys); delete it once
  the maintainer has confirmed that nothing is missing.
* **Release key**: RSA (shared with terraform-provider-contabo), fingerprint `EC973EEC56F04F2C92EE43D953D83AB409F9A781`. Public key: `task release:public-key` (copy in `.secrets/release-signing-key.public.asc`).

## Done this session

Second session on 2026-11-07, for the Crossplane provider (Upjet):

* Public package `provider/` (`provider.New`, `provider.Address`); `main.go` uses it.
  `provider/provider_test.go` checks the schemas through the plugin protocol and counts the
  resources (52) and data sources (12).
* `internal/client`: OAuth tokens are cached per server, tenant, organization, client and scopes
  for the whole process (`sharedTokenCache` in `auth.go`), and HTTP transports per TLS setting
  (`sharedTransport` in `client.go`). Upjet creates a provider server for every reconcile, so
  each reconcile used to request a token. Unit tests in `internal/client/auth_test.go`.
* Upjet read check: `acctest.Test` (`internal/acctest/upjet.go`) replaces `resource.Test` in all
  43 test cases. After each step it rebuilds every resource's prior state from the ID and the
  arguments (computed attributes cleared), calls `ReadResource` through the protocol, and fails
  on errors, "missing" answers or unrestored computed attributes.
* Read fixes found by the check (`tfutil.IDParts`, `tfutil.SetStringIfNull`,
  `tfutil.SetBoolIfNull` in `internal/tfutil/fill.go`): branding preference and text,
  notification template and secret take empty identifying attributes from the ID; governance
  connector looks up its category; custom authenticator derives `authentication_type` from the
  server's `2FA` tag; application share and governance connector fill their defaults.
* New test `TestAccOAuth2IssuerUsageScope` (in a test tenant); this resource had no test.
* Lab tasks connect over SSH to the IP from the Terraform state, because the lab's name may not
  resolve for up to an hour after a destroy (negative DNS caching). Documented in
  `deploy/README.md`.
* Upjet's scraper reads all 52 pages in `docs/resources` without changes (checked with Upjet
  v2.5.1, which needs Go 1.26.8). A separate module fetched v0.1.1 from the Go proxy, and Upjet
  configured all 52 resources with the in-process provider; its CRD type builder passed for all
  of them in both scopes (program in the Crossplane project's `_debug/upjet-smoke/`).
* The Crossplane handoff moved to `/home/hayone/github/crossplane-provider-wso2is/BRIEF.md`
  (new local project with `HANDOFF.md`, `CLAUDE.md`, skills and `.env` for the lab).

The first session of 2026-11-07 (v0.1.0) moved the lab to Hetzner, fixed the name filters, the
missing-application handling and the retry backoff, added OpenTofu to the tooling and CI,
rewrote the docs, and added `RELEASING.md`, `HANDOFF.md` and the `session-handoff` and
`write-docs` skills.

## Open items

1. Maintainer: finish the OpenTofu registry submission (provider form, and the key form if the
   key is not registered for the whole `hayone1` namespace). See RELEASING.md step 5.
2. Crossplane provider: built in `/home/hayone/github/crossplane-provider-wso2is`. Fix Terraform
   provider bugs that it finds here, with tests, and release a new version.
3. Destroy the lab (`task lab:destroy`) when the Crossplane work no longer needs it.

## Known problems

Server behaviour that the provider cannot fix is documented for users in
`templates/guides/server-behaviour.md.tmpl` and, with technical detail, in
`contributing/wso2-api-behaviour.md`. The main ones:

* Tenants cannot be created with client credentials (server error `TM-65002`).
* The root organization's flows cannot be deleted without a `deployment.toml` change.
* Some tenant-wide settings have no reset API, so destroying their resource leaves the values.
* Secrets are write-only on most APIs, so out-of-band changes to them are not detected.
* WSO2 IS 7.2 lacks the 7.3 features; 7.1 is not supported.
