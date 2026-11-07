# Terraform Provider for WSO2 Identity Server

[![CI](https://github.com/hayone1/terraform-provider-wso2is/actions/workflows/ci.yml/badge.svg)](https://github.com/hayone1/terraform-provider-wso2is/actions/workflows/ci.yml)
[![Acceptance](https://github.com/hayone1/terraform-provider-wso2is/actions/workflows/acceptance.yml/badge.svg)](https://github.com/hayone1/terraform-provider-wso2is/actions/workflows/acceptance.yml)

This provider lets you manage [WSO2 Identity Server](https://wso2.com/identity-server/) with
[Terraform](https://www.terraform.io/) or [OpenTofu](https://opentofu.org/). You describe your
applications, users, roles and settings in configuration files. Terraform then uses the server's
management REST APIs to make the server match those files.

```terraform
terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# The provider reads the URL and credentials from WSO2IS_BASE_URL, WSO2IS_CLIENT_ID and
# WSO2IS_CLIENT_SECRET (or WSO2IS_USERNAME and WSO2IS_PASSWORD).
provider "wso2is" {}

resource "wso2is_application" "portal" {
  name = "Customer Portal"
  oidc = {
    grant_types   = ["authorization_code", "refresh_token"]
    callback_urls = ["https://portal.example.com/callback"]
  }
}
```

## What you can manage

| Area | Resources |
|------|-----------|
| Applications | `wso2is_application` (OIDC, SAML, WS-Federation, WS-Trust, adaptive scripts, MFA), `wso2is_application_authorized_api`, `wso2is_application_share` |
| APIs | `wso2is_api_resource` with scopes |
| Users and access | `wso2is_user`, `wso2is_group`, `wso2is_group_members`, `wso2is_role`, `wso2is_role_assignment`, `wso2is_userstore` |
| Attributes | `wso2is_claim`, `wso2is_claim_dialect`, `wso2is_external_claim`, `wso2is_oidc_scope` |
| Connections | `wso2is_identity_provider`, `wso2is_custom_authenticator`, `wso2is_identity_verification_provider` |
| Organizations (B2B) | `wso2is_organization`, `wso2is_organization_discovery`, `wso2is_organization_discovery_configuration`, `wso2is_user_share` |
| Flows and extensions | `wso2is_flow` (custom sign-up), `wso2is_action`, `wso2is_webhook`, `wso2is_workflow`, `wso2is_workflow_association`, `wso2is_script_library`, `wso2is_secret`, `wso2is_vc_template` |
| Notifications and branding | `wso2is_email_sender`, `wso2is_sms_sender`, `wso2is_push_sender`, `wso2is_notification_template`, `wso2is_notification_template_type`, `wso2is_branding_preference`, `wso2is_branding_text` |
| Tenant-wide settings | `wso2is_governance_connector`, `wso2is_validation_rules`, and the `*_configuration` resources for sessions, CORS, DCR, OAuth2, SAML, passive STS, impersonation, remote logging and more |
| Tenants | `wso2is_tenant` |

Data sources look up existing applications, users, groups, roles, claims, identity providers,
organizations, API resources, flows, governance connectors and authenticators.

You can import every resource. To bring an existing server under Terraform, write `import` blocks
and run `terraform plan -generate-config-out=generated.tf`. Terraform then writes the
configuration for you.

## Requirements

* **WSO2 Identity Server 7.3.x.** The provider is built and tested against 7.3.0. Most resources
  also work with 7.2.0, but features that were added in 7.3 do not. Version 7.1 is not supported.
  The [compatibility notes](docs/guides/server-behaviour.md#supported-server-versions) have the
  details.
* **Terraform 1.0 or later**, or **OpenTofu 1.6 or later**. Write-only arguments, such as
  `password_wo`, need Terraform 1.11 or OpenTofu 1.11 or later.

## Documentation

* [Provider configuration](docs/index.md): sign-in methods, tenants, organizations and TLS.
* Guides:
  * [Getting started](docs/guides/getting-started.md): a ten-minute tutorial with a local server.
  * [An application for Terraform](docs/guides/authentication.md): stop using the admin password.
  * [Organizations (B2B)](docs/guides/organizations.md): serve business customers.
  * [Custom sign-up flow](docs/guides/sign-up-flow.md): add your own fields to the sign-up page.
  * [Tenants](docs/guides/tenants.md): create tenants and manage objects inside them.
  * [Known server behaviour and limitations](docs/guides/server-behaviour.md): read this before
    you start.
* Reference pages for every [resource](docs/resources) and [data source](docs/data-sources).

## WSO2 Identity Server references

* Product documentation: <https://is.docs.wso2.com/en/latest/>
  (source: [wso2/docs-is](https://github.com/wso2/docs-is))
* Product source and releases: [wso2/product-is](https://github.com/wso2/product-is)
* Management REST API definitions: [wso2/identity-api-server](https://github.com/wso2/identity-api-server)

## Run WSO2 Identity Server

For a quick local server, run `docker run -p 9443:9443 wso2/wso2is:7.3.0`. Then follow the
[Getting started](docs/guides/getting-started.md) guide.

For a setup that is closer to production, the [`deploy/`](deploy/) folder has Kubernetes manifests
with PostgreSQL and TLS. It also lists the server settings that the provider benefits from, such as
allowing tenant and flow deletion.

## Use the provider from Go

Other Go programs can run the provider in their own process. The package
`github.com/hayone1/terraform-provider-wso2is/provider` has a `New` function that returns the
provider. A Crossplane provider for WSO2 Identity Server, generated with
[Upjet](https://github.com/crossplane/upjet), uses it. Terraform and OpenTofu do not need it.

## Contributing

Bug reports, server quirks that you ran into, and pull requests are welcome.
[CONTRIBUTING.md](CONTRIBUTING.md) explains how to set up your computer, run the tests against a
real server, and follow the conventions of the code. The repository also includes
[Claude Code skills](.claude/skills) that guide an AI coding agent through the same steps.

## License

This project uses the [Apache License 2.0](LICENSE). Copies and derived works must keep the
[NOTICE](NOTICE) file.

WSO2 and WSO2 Identity Server are trademarks of WSO2 LLC. This is an independent community project.
It is not affiliated with or endorsed by WSO2.
