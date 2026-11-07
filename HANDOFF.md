# Handoff

This file is the memory between work sessions. Read it at the start of a session and update it
before you stop (see the `session-handoff` skill). Never put secret values here.

## Last updated

2026-11-07: release preparation. New Hetzner lab, full test runs, bug fixes, docs rewrite,
release signing key and publishing guide.

## State

* 52 resources and 12 data sources. Build, vet, unit tests, `task lint` (0 issues), docs
  validation and `task examples:validate` (Terraform and OpenTofu) pass.
* Full acceptance suite against the Hetzner lab (WSO2 IS 7.3.0, PostgreSQL), Terraform 1.14:
  `task testacc -- -p 1 ./...` passed on 2026-11-07: 55 tests and 33 sub-tests in 24 packages,
  0 failures (log: `_debug/lab-regression-final.log`).
* OpenTofu 1.13: the identity, API resource, lookup, organization and provider (not-found)
  packages pass, and all 69 examples validate.
* Fresh-copy check (only the files that would be committed, `cp example.env .env`, Docker
  server): build, unit tests, `task wso2:up` and the identity, lookup and not-found acceptance
  tests pass. The Getting started guide applies cleanly (8 objects), the next plan is empty,
  the Step 5 change plans one update, and destroy removes everything. This check found that
  `.gitignore` excluded `internal/service/idp/testdata/cert.pem` (fixed).
* `task release:snapshot` builds all 13 platform archives, the checksums and the manifest.
* Pushed to https://github.com/hayone1/terraform-provider-wso2is (public). Release secrets are
  uploaded. The maintainer only does the registry steps in [RELEASING.md](RELEASING.md).

## Environment

* **Hetzner lab**: one `cpx32` server (4 vCPU, 8 GB) named `wso2is-lab` in `nbg1`, Ubuntu 24.04,
  k3s v1.37.2. Created by `deploy/hetzner` (Terraform state is local in that folder and
  git-ignored). The `cx43` type was sold out in every location on 2026-11-07.
  * WSO2: <https://wso2is.lab.software-cloud.com> (Let's Encrypt). Console at `/console`.
  * DNS: `lab.software-cloud.com` and `*.lab.software-cloud.com` point at the server (zone
    `software-cloud.com` in the same Hetzner project).
  * It costs about EUR 42 a month while it exists. `task lab:destroy` deletes it;
    `task lab:up` recreates everything in about ten minutes.
* **Secrets** live in `.env` (git-ignored): `HCLOUD_TOKEN`, the `WSO2_*` passwords, the test
  JDBC settings, and the release signing key (`GPG_FINGERPRINT`, `PASSPHRASE`,
  `GPG_PRIVATE_KEY`). The signing key is also in `.secrets/` (git-ignored files: private key, public key, passphrase) and in the maintainer's local GPG keyring.
  `_debug/env.backup-2026-11-07` is a backup of the old `.env` (old lab keys); delete it once
  the maintainer has confirmed that nothing is missing.
* **Release key**: RSA (shared with terraform-provider-contabo), fingerprint `EC973EEC56F04F2C92EE43D953D83AB409F9A781`. Public key: `task release:public-key` (copy in `.secrets/release-signing-key.public.asc`).

## Done this session

* Lab moved from the old IBM server and upstream cluster to Hetzner: `deploy/hetzner`
  (server, firewall, DNS), `deploy/k8s/base/certificates.yaml` (internal CA with cert-manager),
  `deploy/k8s/platform` (Let's Encrypt issuers), `deploy/k8s/overlays/lab` (Traefik ingress),
  `taskfiles/lab.yml` rewritten. The AppArmor workaround is gone (Ubuntu 24.04).
* Bug fixes (all with tests):
  * Name lookups failed for names with "and"/"or": `client.FindByEq`
    (`internal/client/filter.go`), test `TestAccLookupDataSources_namesWithFilterKeywords`.
  * Deleted applications were not detected by `wso2is_application_authorized_api` and
    `wso2is_application_share`: `client.ApplicationMissing`; new test
    `TestAccResources_missingObjectIsNotFound` (`internal/provider/notfound_test.go`) imports a
    missing ID for all 29 object resources.
  * Retry backoff overflow with a large `max_retries`.
* OpenTofu support in the tooling: `TF_CLI=tofu` for `task testacc*`, `task examples:validate`
  and `task lab:infra`; the acceptance workflow runs Terraform and OpenTofu. The test provider is
  registered as `hayone1/wso2is` in `internal/acctest` (OpenTofu needs a real namespace).
* Docs rewritten in plain, tutorial-style English: all guides, the provider index, README,
  CONTRIBUTING, deploy/README, every resource and data source description, and the hard-to-read
  attribute descriptions. Style rules: `contributing/docs-style.md`.
* New: `RELEASING.md`, `HANDOFF.md`, skills `session-handoff` and `write-docs`; the other skills,
  CLAUDE.md, `example.env` and `.env` updated. Release tasks: `task release:secrets`,
  `task release:public-key`, `task release:snapshot`.

## Open items

1. Maintainer: publish (RELEASING.md): commit, create the public GitHub repository, run
   `task release:secrets`, register the key with both registries, tag `v0.1.0`.
2. After the first push: check that the CI and Acceptance workflows pass on GitHub (they have
   only run locally so far).
3. Optional: decide whether to keep the lab running between releases.

## Known problems

Server behaviour that the provider cannot fix is documented for users in
`templates/guides/server-behaviour.md.tmpl` and, with technical detail, in
`contributing/wso2-api-behaviour.md`. The main ones:

* Tenants cannot be created with client credentials (server error `TM-65002`).
* The root organization's flows cannot be deleted without a `deployment.toml` change.
* Some tenant-wide settings have no reset API, so destroying their resource leaves the values.
* Secrets are write-only on most APIs, so out-of-band changes to them are not detected.
* WSO2 IS 7.2 lacks the 7.3 features; 7.1 is not supported.
