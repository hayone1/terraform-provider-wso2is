---
page_title: "Known server behaviour and limitations"
subcategory: ""
description: |-
  WSO2 Identity Server behaviour that affects your Terraform configuration, what the provider does about it, and what the provider cannot do.
---

# Known server behaviour and limitations

The management APIs of WSO2 Identity Server 7.3.0 have some surprising behaviour. The provider
handles most of it for you, so you never notice. This page lists the behaviour that still affects
how you write your configuration.

Each section says what the server does, what the provider does about it, and what you should do.
Developers can find the complete list, with technical details, in
`contributing/wso2-api-behaviour.md` in the provider's GitHub repository.

## Supported server versions

The provider supports **WSO2 Identity Server 7.3.x**. The test suite also ran against the official
7.2.0 and 7.1.0 Docker images, with these results:

* **7.2.0 mostly works.** These areas pass their tests: actions, API resources and their
  authorization, applications, custom authenticators, branding, claims and OIDC scopes, flows,
  governance connectors and validation rules, users, groups and roles, identity providers,
  identity verification providers, organizations and email domain discovery, script libraries and
  secrets, tenants, user stores, webhooks, and all data sources.

  Features that were added in 7.3 do not work on 7.2.0. The server answers with HTTP 400
  `UE-10000` or with HTTP 404. These features are `wso2is_email_sender`, the `authentication` block
  of `wso2is_sms_sender`, `wso2is_push_sender`, `wso2is_user_share`, `wso2is_vc_template`,
  notifications in workflows, `wso2is_oauth2_configuration`,
  `wso2is_fraud_detection_configuration`, and `oidc.token_issuer_organization_id` on
  applications.

  On a new 7.2.0 server, creating two organizations at the same time can fail once, because of a
  race condition in the server. Run `terraform apply` again to finish.
* **7.1.0 is not supported.** Most areas fail on 7.1.0.

## Settings that change how other resources behave

**Inbound provisioning user store.** You can set `provisioning_userstore` in
`wso2is_inbound_provisioning_configuration`. While it is set, the server puts *every* user that
is created through SCIM into that user store. The server ignores `wso2is_user.user_store`. It also
refuses to update users in other user stores. The same thing happens when you set inbound
provisioning on the application that the provider signs in with.

The provider notices when the server creates a user in the wrong user store. It then deletes that
user and tells you why. When you destroy the configuration resource, the provider clears the
setting.

**Approval workflows.** A `wso2is_workflow_association` makes an operation, such as `ADD_USER`, wait
for approval. While it waits, Terraform cannot finish creating the object. Do not attach workflows
to operations that Terraform performs.

## Secrets

* Most APIs never send secrets back. The provider keeps the secret from your configuration. It
  cannot notice when someone changes the secret in the Console. Where a resource offers a
  write-only argument (`*_wo`, Terraform 1.11 or later), use it to keep the secret out of the state.
* A few APIs do send secrets back. These are the SMS sender with basic authentication (`key` and
  `secret`), push senders (`wso2is_push_sender`), and the secrets of identity provider
  authenticators. These secrets end up in the Terraform state like any other value. Protect your
  state file, for example with an encrypted remote backend.

## Changes that take time

The server finishes some changes in the background. The provider waits for these changes before it
reports success:

* **User stores.** A background task deploys a new user store about 10 to 20 seconds after the
  request.
* **User sharing.** `wso2is_user_share` runs in the background. If the server cannot find a role
  that you named, it skips the role without an error. The provider then times out after two
  minutes. If this happens, check the role names and audiences.
* **Tenants.** A new tenant needs a short time before its APIs answer. A tenant that reuses the
  domain of a recently deleted tenant does not work for about 15 minutes, because the server keeps
  cached data about the deleted tenant.

## Tenants

* Create tenants with **basic authentication**. The server fails when it creates a tenant with a
  client credentials token.
* The server deletes tenants only when `deployment.toml` sets `[tenant_mgt] tenant_deletion = true`.
  Without this setting, destroying `wso2is_tenant` only deactivates the tenant. A deactivated tenant
  keeps its domain.
* The super tenant's email provider is set in `deployment.toml` (`[output_adapter.email]`).
  `wso2is_email_sender` does not work in the super tenant.

## Organizations

* To manage a sub-organization, the provider must sign in with an application's client
  credentials and the `organization_switch` grant. The organization APIs reject basic
  authentication. See [Organizations](organizations).
* Sub-organizations are created with the provider of their parent organization.
* Email domain discovery (`wso2is_organization_discovery`) only works after you create
  `wso2is_organization_discovery_configuration` with `email_domain_enabled = true`.

## Flows

By default, `wso2is_flow` cannot delete a flow in the root organization. WSO2 IS 7.3.0 has no
access control rule for `DELETE /api/server/v1/flow`, so the server answers HTTP 403. When you
destroy the resource, the provider turns the flow off instead and shows a warning.

To allow deletion, add this rule to `deployment.toml` and restart the server:

```toml
[[resource.access_control]]
context = "(.*)/api/server/v1/flow(.*)"
secure = true
http_method = "DELETE"
scopes = ["internal_flow_update"]
```

## Tenant-wide settings only change what you configure

Some resources manage settings that always exist, such as the `*_configuration` resources,
`wso2is_governance_connector` and the `wso2is_branding_*` resources. These resources only change
the settings that you list, and they leave the other settings alone.

Some of these APIs have no way to reset a setting. When you destroy such a resource, the last
values stay on the server. Each resource page tells you whether this happens.

Manage each tenant-wide setting in only one configuration. Two configurations that manage the same
setting overwrite each other's changes.

## Rules for names and values

* **Group names** must be 3 to 30 characters long and cannot contain spaces. This is the default
  rule of the user store. The server rejects other names with HTTP 400.
* **Scope names** must be unique in the whole tenant, across all API resources.
* **Role mappings of identity providers** must name local roles as `Internal/<role>`. The server
  silently drops other forms. The provider checks the result and reports the problem.
* **Regular expression rules** (`wso2is_validation_rules.regex`) only work for passwords.
* **Application fields that are fixed at creation.** The server only accepts some application
  fields when the application is created: `is_management_app`, `is_b2b_self_service_app`, custom
  OIDC client IDs and secrets, and the FAPI setting. Changing one of them makes Terraform replace
  the application.
* **`wso2is_vc_template` has no `description`.** The 7.3.0 API accepts a description when you
  create a template, but it does not store it.

## Settings that only exist in deployment.toml

The server reads some settings only from its `deployment.toml` file. The provider cannot change
these settings. Examples are token persistence, the hashing of secrets, keystores, the super
tenant's email provider and access control rules.

## APIs without a resource

These management APIs do not have a resource yet:

* certificate revocation checks (`/certificate-validation`)
* tenant keystore certificates (`/keystores/certs`)
* the deprecated B2B self-service preferences API
* guest invitations
* the consent management v2 API (update level 7.3.0.12 and later)

The provider also leaves runtime operations to the Console and the APIs. Examples are user
sessions, credentials that users enrol, and workflow approvals.
