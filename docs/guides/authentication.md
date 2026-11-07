---
page_title: "Authentication: an application for Terraform"
subcategory: ""
description: |-
  Create an M2M application for Terraform, so Terraform no longer needs an administrator password, and choose which management APIs it can call.
---

# Authentication: an application for Terraform

The provider can sign in with an administrator's username and password. That is the easiest way
to get started, but it is not a good fit for automation:

* The administrator account can do everything on the server.
* Changing the password breaks every pipeline that uses it.
* The password belongs to a person, not to a pipeline.

A better choice is an **M2M (machine-to-machine) application** that uses the OAuth2 **client
credentials** grant. The application can only call the management APIs that you allow. You can
also replace its secret without changing any user account.

In this guide, you create that application with Terraform. You do this once, with administrator
credentials. After that, your other configurations use the application's client ID and secret.

## Step 1: Write the bootstrap configuration

Save this file as `main.tf` in a new folder. It creates the application and allows it to call the
management APIs that the provider uses.

```terraform
# Creates an M2M application for Terraform. Apply this configuration once with
# administrator credentials:
#   WSO2IS_BASE_URL=https://id.example.com WSO2IS_USERNAME=admin WSO2IS_PASSWORD=... terraform apply
# Then use the two outputs as WSO2IS_CLIENT_ID and WSO2IS_CLIENT_SECRET in your other
# configurations.
terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# The URL and the credentials come from the environment variables above.
provider "wso2is" {}

locals {
  # The management APIs that the provider's resources call. Remove the APIs of the
  # resources that you do not manage.
  management_apis = toset([
    "/api/server/v1/actions",
    "/api/server/v1/api-resources",
    "/api/server/v1/applications",
    "/api/server/v1/applications/share",
    "/api/server/v1/authenticators",
    "/api/server/v1/authenticators/custom",
    "/api/server/v1/branding-preference",
    "/api/server/v1/claim-dialects",
    "/api/server/v1/configs",
    "/api/server/v1/flow",
    "/api/server/v1/identity-governance",
    "/api/server/v1/identity-providers",
    "/api/server/v1/idv-providers",
    "/api/server/v1/notification-senders",
    "/api/server/v(.*)/notification/(?:email|sms)/template-types",
    "/api/server/v1/oidc/scopes",
    "/api/server/v1/organization-configs/discovery",
    "/api/server/v1/organizations",
    "/api/server/v1/organizations/discovery",
    "/api/server/v1/script-libraries",
    "/api/server/v1/secrets",
    "/api/server/v1/userstore",
    "/api/server/v1/validation-rules",
    "/api/server/v1/vc-templates",
    "/api/server/v1/webhooks",
    "/api/server/v1/webhooks/metadata",
    "/api/server/v1/workflow-associations",
    "/api/server/v1/workflows",
    "/scim2/Groups",
    "/scim2/Roles",
    "/api/server/v2/users", # sharing users with organizations
    "/scim2/Users",
  ])
  # These APIs exist only in the super tenant (carbon.super). They manage tenants and the
  # admin advisory banner.
  system_apis = toset([
    "/api/server/v1/tenants",
    "/api/server/v1/admin-advisory-management/banner",
  ])
}

# The application that Terraform signs in with.
resource "wso2is_application" "terraform" {
  name        = "Terraform"
  description = "Manages this server's configuration"
  template_id = "m2m-application"
  oidc = {
    grant_types = ["client_credentials"]
  }
}

# Look up each management API, and allow the application to use all of its scopes.
data "wso2is_api_resource" "management" {
  for_each   = setunion(local.management_apis, local.system_apis)
  identifier = each.value
}

resource "wso2is_application_authorized_api" "management" {
  for_each        = data.wso2is_api_resource.management
  application_id  = wso2is_application.terraform.id
  api_resource_id = each.value.id
  scopes          = each.value.scope_names
}

output "client_id" {
  value = wso2is_application.terraform.oidc.client_id
}

output "client_secret" {
  value     = wso2is_application.terraform.oidc.client_secret
  sensitive = true
}
```

The list in `management_apis` covers every resource of the provider. If you only manage a few
resources, you can remove the APIs that you do not need. The application then has fewer
permissions.

## Step 2: Apply it with administrator credentials

Run these commands. Replace the URL and password with your own values.

```shell
export WSO2IS_BASE_URL=https://id.example.com
export WSO2IS_USERNAME=admin
export WSO2IS_PASSWORD='your-admin-password'
terraform init
terraform apply
```

## Step 3: Switch to the application's credentials

Read the client ID and secret from the outputs. Then remove the administrator credentials from
your shell:

```shell
export WSO2IS_CLIENT_ID=$(terraform output -raw client_id)
export WSO2IS_CLIENT_SECRET=$(terraform output -raw client_secret)
unset WSO2IS_USERNAME WSO2IS_PASSWORD
```

Your other configurations can now use an empty provider block. The provider reads the URL and the
credentials from the environment variables.

```terraform
provider "wso2is" {}
```

In a CI/CD pipeline, store `WSO2IS_CLIENT_ID` and `WSO2IS_CLIENT_SECRET` as pipeline secrets.

The bootstrap configuration's state file contains the client secret. Store that state somewhere
safe, such as an encrypted remote backend.

## How scopes work

A **scope** is a named permission, such as `internal_user_mgt_create`. When the provider signs in
with client credentials, it asks for the `SYSTEM` scope. The server replaces `SYSTEM` with every
scope that the application is allowed to use. So the APIs that you authorize in Step 1 decide what
Terraform can do.

You only need to set `scopes` (or `WSO2IS_SCOPES`) if you want to ask for fewer scopes than the
application is allowed to use.

If the application is not allowed to call an API, the resources that use that API fail with
HTTP 403. To fix this, add the API to the application:

* In the Console, open the application and go to the **API Authorization** tab.
* With Terraform, look up the API with the `wso2is_api_resource` data source and add a
  `wso2is_application_authorized_api` resource for it.

## What client credentials cannot do

* **Create tenants.** The server fails when it creates a tenant with a client credentials token.
  Use basic authentication with the super tenant's administrator for `wso2is_tenant`.
* **Manage organizations on their own.** The application also needs the `organization_switch`
  grant, and you must share it with each organization. The [Organizations guide](organizations)
  explains the setup.
* **Manage other tenants.** An application belongs to one tenant. To manage another tenant, create
  an application inside that tenant, or use that tenant's administrator.

## Access tokens from your own tooling

You can also give the provider a token directly with `access_token` (or `WSO2IS_ACCESS_TOKEN`).
The provider sends the token as it is. It cannot refresh the token, so the token must stay valid
until Terraform finishes. To manage a sub-organization this way, the token must be issued for that
organization.
