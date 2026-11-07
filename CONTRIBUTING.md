# Contributing

Thank you for helping. This guide shows you how to set up your computer, run the tests, and
follow the conventions of the code.

The WSO2 Identity Server APIs have many quirks that only show up against a real server. For this
reason, the acceptance tests, which run against a real server, are the final word on whether a
change works.

## Quick start

These steps take you from a fresh clone to a passing acceptance test. They need about ten minutes.

1. Install the tools listed in [What you need](#what-you-need).
2. Clone the repository and create your local settings file:

   ```shell
   git clone https://github.com/hayone1/terraform-provider-wso2is.git
   cd terraform-provider-wso2is
   cp example.env .env
   ```

   You do not need to change `.env` for local testing. Task reads the file automatically.
3. Build the provider and run the unit tests:

   ```shell
   task build
   task test
   ```

4. Start WSO2 Identity Server in Docker. The first start downloads the image and takes a few
   minutes.

   ```shell
   task wso2:up
   ```

5. Run one acceptance test against the local server:

   ```shell
   task testacc:local -- -run TestAccGroupAndRoles ./internal/service/identity/
   ```

6. When you are done, stop the server:

   ```shell
   task wso2:down
   ```

## What you need

* [Go](https://go.dev/dl/), the version in `go.mod`
* [Task](https://taskfile.dev/installation/) 3.40 or later. Every workflow in this repository is a
  task. Run `task --list` to see them all.
* [Terraform](https://developer.hashicorp.com/terraform/install) 1.11 or later, or
  [OpenTofu](https://opentofu.org/docs/intro/install/) 1.11 or later
* [Docker](https://docs.docker.com/get-docker/), to run a local WSO2 Identity Server
* Optional: [golangci-lint](https://golangci-lint.run/welcome/install/), for `task lint`

## Everyday tasks

| Task | What it does |
|------|--------------|
| `task build` | Builds the provider into `bin/`. |
| `task test` | Runs the unit tests. |
| `task wso2:up` | Starts WSO2 Identity Server in Docker, with the settings that the tests need. Add `WSO2_VERSION=7.2.0` for another version. |
| `task testacc:local -- -run TestAccGroup ./internal/service/identity/` | Runs acceptance tests against the local Docker server. |
| `task testacc -- ./internal/service/identity/` | Runs acceptance tests against `WSO2IS_BASE_URL`, or against the lab server in `.env`. |
| `task testacc:local TF_CLI=tofu -- ./...` | Runs the acceptance tests with OpenTofu instead of Terraform. |
| `task testacc:matrix VERSIONS="7.2.0 7.3.0"` | Runs the acceptance tests against several server versions, one after another. |
| `task sweep` | Deletes objects that interrupted tests left behind. These objects have `tfacc` in their names. |
| `task docs` | Rebuilds `docs/` from the schemas, `templates/` and `examples/`. |
| `task examples:validate` | Runs `terraform validate` on every example against the local build. |
| `task lint`, `task fmt`, `task vet` | Runs the static checks and formats the code. |
| `task install` | Installs the provider and prints a `dev_overrides` block, so you can try it with Terraform. |

Acceptance tests create real objects on the server. Run them against a server that you can throw
away. Every object that a test creates has `tfacc` in its name, and `task sweep` removes any
leftovers.

## Repository layout

```
provider/                 public Go package that returns the provider (for Crossplane and other programs)
internal/provider/        provider configuration and the list of resources (registry.go)
internal/client/          HTTP client: sign-in, retries, errors, JSON helpers, API calls
internal/tfutil/          schema helpers, plan modifiers and conversions shared by resources
internal/service/<area>/  one package per API area, with resources, data sources and tests
internal/acctest/         helpers for acceptance tests (test tenant, test organization, ...)
internal/sweep/           sweepers that delete test leftovers
examples/                 HCL used in the docs; every example is validated in CI
templates/                doc templates and guides (tfplugindocs)
contributing/             notes on server behaviour, and design decisions (ADRs)
deploy/                   Docker settings, the Kubernetes deployment and the Hetzner lab
```

## WSO2 references

Read the server's documentation and source code when you add or change a resource. The OpenAPI
specifications are incomplete and sometimes wrong, so the source code is often the best answer.

| What | Where |
|------|-------|
| Product documentation | <https://is.docs.wso2.com/en/latest/>, source in [wso2/docs-is](https://github.com/wso2/docs-is) |
| Product build, releases and integration tests | [wso2/product-is](https://github.com/wso2/product-is) |
| Management REST APIs (OpenAPI specs and REST layer) | [wso2/identity-api-server](https://github.com/wso2/identity-api-server), in `components/<api>/` |
| Core business logic | [wso2/carbon-identity-framework](https://github.com/wso2/carbon-identity-framework) and the repositories under [wso2-extensions](https://github.com/wso2-extensions) |
| Console and login pages | [wso2/identity-apps](https://github.com/wso2/identity-apps) |

## Add or change a resource

1. **Explore the API first.** Use `curl` against a real server. Create an object, read it, update
   it partly and fully, and delete it. Write down what the server changes, drops or returns
   differently from what you sent.
2. **Write the resource** in `internal/service/<area>/`, and register it in
   `internal/provider/registry.go`. Follow the patterns below.
3. **Write an acceptance test.** The test must create the object, update it (including removing
   optional settings), import it with `ImportStateVerify`, and destroy it. Run it with
   `acctest.Test`, not `resource.Test` (see "Read from the ID" below). Use
   `acctest.RandomName` for names. Add the resource to
   `TestAccResources_missingObjectIsNotFound` in `internal/provider/notfound_test.go`.
4. **Add an example** in `examples/resources/<type>/resource.tf` and `import.sh`. Then run
   `task docs` and `task examples:validate`.
5. **Record every quirk** in [`contributing/wso2-api-behaviour.md`](contributing/wso2-api-behaviour.md),
   together with what the provider does about it. If the quirk changes how users write
   configurations, also add it to `templates/guides/server-behaviour.md.tmpl`.

### Patterns that the provider follows

* **Updates.** WSO2 `PATCH` endpoints usually replace each nested section that they receive as a
  whole. Send a section only when something in it changed, and then send the complete section.
  `PUT` endpoints need a read-modify-write: `GET` the current document, compute what changed
  between the plan and the state (`client.DiffJSON`), and merge that change into the server's
  document (`client.MergeJSON`). Settings that the provider does not manage then survive.
* **Do not send the server's defaults back.** Some endpoints reject their own `GET` output, and
  older servers reject fields that they do not know. Send only what the configuration sets.
* **Defaults.** An attribute with a fixed server default uses the `tfutil.Bool`, `tfutil.String`,
  `tfutil.Int64` (and similar) helpers. Nested blocks use `tfutil.Object`, so removing a block
  resets its defaults. A value that the server decides uses `tfutil.OC*` (optional and computed,
  keeping the prior state). A value that is derived from other attributes uses the
  `*StateUnlessChanged` plan modifiers.
* **No computed attributes or defaults inside set elements.** Terraform matches set elements by
  their value, so a computed value inside an element causes a permanent difference. Normalize the
  value in `read` instead (see `wso2is_user_share`).
* **Unordered server data is a set.** If the server reorders a list, make the attribute a set.
* **Secrets.** When the API does not return a secret, keep the value from the configuration.
  Offer a `*_wo` write-only variant where it makes sense.
* **Tenant-wide settings** that live in the resident identity provider (governance connectors and
  the `*_configuration` resources) take `client.LockTenantConfig()` for reads and writes.
* **Missing objects.** `Read` must remove the resource from the state when the object is gone.
  Some endpoints do not answer 404 for a missing object. Find out what they answer, and handle it
  (see `client.ApplicationMissing`).
* **Read from the ID.** `Read` must find the object with only the ID and the arguments in the
  prior state. Computed attributes, and attributes that have a default, can be empty. This
  happens after an import, and always in Crossplane: Upjet runs this provider inside a Crossplane
  provider, and it builds the prior state from the ID and the managed resource's spec. Take the
  missing values from the ID (`tfutil.IDParts` and `tfutil.SetStringIfNull`), or look them up.
  `acctest.Test` checks this after every test step. It fails when `Read` returns an error, reports
  the object as missing, or leaves a computed attribute empty. It logs other differences with the
  prefix `upjet read:`. Tests whose resources use several provider configurations pass
  `acctest.UpjetReadProviderConfig` or `acctest.SkipUpjetRead`.
* **Lookups by name.** Use `client.FindByEq`. It quotes the value, so names with spaces or with the
  words "and" and "or" work, and it falls back to a full list when the server cannot filter.
* **Errors** go through `tfutil.AddAPIError`, which adds hints for sign-in and TLS problems. When
  the server's message does not help, add a message that says what to change.
* **Tests that change tenant-wide behaviour** (for example the inbound provisioning user store) run
  in their own tenant (`acctest.TestTenant`), so they cannot break tests that run at the same time.
  Tests with more than one `wso2is` provider block use `acctest.ExternalProviders`.

The reasons behind the overall design are in [`contributing/adr`](contributing/adr).

### The public `provider` package

Other Go programs use `provider.New` to run the provider in their own process. The Crossplane
provider for WSO2 Identity Server does this. Keep the package's API stable, and keep the
provider safe to use in this way:

* The provider instance must not change after it is created. `Configure` puts the API client
  into the configure response, not into the provider struct. One instance can serve many
  configurations at the same time.
* State that should be shared in a process lives in package variables in `internal/client`:
  the token caches (`sharedTokenCache`), the HTTP transports (`sharedTransport`) and the
  tenant-wide locks (`LockTenantConfig`).
* `TestNew_servesValidSchemas` in `provider/provider_test.go` counts the resources and data
  sources. Update the count when you add one.

## Pull requests

* Keep each pull request to one change, with tests. CI runs the build, vet, lint, unit tests, a
  check that the docs are up to date, and example validation. The acceptance workflow runs the
  acceptance tests against a Docker server with Terraform and with OpenTofu.
* Say which WSO2 Identity Server version you tested against.
* Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, ...).
  The release notes are built from the commit messages.

## Report server quirks

If the provider fails because the server behaves in an unexpected way, use the "Server behaviour"
issue template. It asks for the request and the response that show the problem. These reports are
as valuable as code.

## Working with AI coding agents

The repository includes [Claude Code skills](.claude/skills) for the common tasks: adding a
resource, running acceptance tests and investigating API quirks. [HANDOFF.md](HANDOFF.md) records
the current state of the work. Agents read it at the start of a session and update it at the end.

## Releases

[RELEASING.md](RELEASING.md) describes how to publish a release to the Terraform Registry and the
OpenTofu Registry.
