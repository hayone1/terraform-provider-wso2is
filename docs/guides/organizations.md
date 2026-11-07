---
page_title: "Organizations (B2B)"
subcategory: ""
description: |-
  Create organizations for your business customers, and manage the users, groups, roles and sub-organizations inside them from one Terraform configuration.
---

# Organizations (B2B)

Organizations let one tenant serve many business customers. Each **organization** has its own
users, groups, roles and sub-organizations. Users of an organization sign in to the applications
that you share with that organization.

In this guide, you build a complete B2B setup in one Terraform configuration. You create:

* an organization called "Partner Inc"
* a customer portal application that all organizations can use
* an application that Terraform uses to make changes inside the organization
* a user, a group and a role inside the organization
* a sub-organization called "Partner EU"

## How Terraform reaches into an organization

A provider block normally manages the root organization. To manage objects inside an
organization, you add a second provider block that sets `organization_id`.

This second provider cannot use a username and password, because the organization APIs reject
basic authentication. Instead, it signs in with an application of the root organization. The
application must meet three conditions:

1. The application has the `client_credentials` and `organization_switch` grants.
2. The application is allowed to call the organization APIs that you need. The identifiers of
   these APIs start with `/o/`.
3. The application is shared with the organization.

The provider first gets a token for the root organization. Then it trades that token for an
organization token with the `organization_switch` grant. Finally, it calls the organization APIs
with the organization token.

## The complete example

The example below does everything in one configuration. Read the comments to see what each block
does.

```terraform
terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# The root organization. The provider signs in with administrator credentials or an
# M2M application, read from the WSO2IS_* environment variables.
provider "wso2is" {}

# A customer organization.
resource "wso2is_organization" "partner" {
  name        = "Partner Inc"
  description = "Reseller in the EU"
}

# The portal that customers sign in to. It is shared with every organization.
resource "wso2is_application" "portal" {
  name = "Partner Portal"
  oidc = {
    grant_types   = ["authorization_code", "refresh_token"]
    callback_urls = ["https://portal.example.com/callback"]
  }
}

resource "wso2is_application_share" "portal" {
  application_id          = wso2is_application.portal.id
  share_with_all_children = true
}

# The application that Terraform uses to make changes inside the organization. It needs
# the client_credentials and organization_switch grants.
resource "wso2is_application" "b2b_admin" {
  name = "Organization administration"
  oidc = {
    grant_types = ["client_credentials", "organization_switch"]
  }
}

# Allow the application to call the organization APIs (their identifiers start with /o/).
data "wso2is_api_resource" "org" {
  for_each = toset([
    "/o/scim2/Users",
    "/o/scim2/Groups",
    "/o/scim2/Roles",
    "/o/api/server/v1/organizations",
    "/o/api/server/v1/applications",
  ])
  identifier = each.value
}

resource "wso2is_application_authorized_api" "org" {
  for_each        = data.wso2is_api_resource.org
  application_id  = wso2is_application.b2b_admin.id
  api_resource_id = each.value.id
  scopes          = each.value.scope_names
}

# Share the application with the organization, after the application may call the APIs.
resource "wso2is_application_share" "b2b_admin" {
  application_id   = wso2is_application.b2b_admin.id
  organization_ids = [wso2is_organization.partner.id]
  depends_on       = [wso2is_application_authorized_api.org]
}

# A provider for the organization. It reads organization_id from the share, so Terraform
# configures it only after the application is shared and may call the APIs.
provider "wso2is" {
  alias           = "partner"
  organization_id = one(wso2is_application_share.b2b_admin.organization_ids)
  client_id       = wso2is_application.b2b_admin.oidc.client_id
  client_secret   = wso2is_application.b2b_admin.oidc.client_secret
}

# Everything below this line is created inside the organization.
resource "wso2is_user" "partner_admin" {
  provider = wso2is.partner
  username = "pat"
  email    = "pat@partner.example"
  password = var.partner_admin_password
}

resource "wso2is_group" "partner_admins" {
  provider = wso2is.partner
  name     = "administrators"
}

resource "wso2is_group_members" "partner_admins" {
  provider = wso2is.partner
  group_id = wso2is_group.partner_admins.id
  user_ids = [wso2is_user.partner_admin.id]
}

resource "wso2is_role" "partner_user_managers" {
  provider    = wso2is.partner
  name        = "user-managers"
  permissions = ["internal_org_user_mgt_view", "internal_org_user_mgt_list", "internal_org_user_mgt_create", "internal_org_user_mgt_update"]
}

resource "wso2is_role_assignment" "partner_user_managers" {
  provider  = wso2is.partner
  role_id   = wso2is_role.partner_user_managers.id
  group_ids = [wso2is_group.partner_admins.id]
}

# A sub-organization. Sub-organizations are created with their parent's provider.
resource "wso2is_organization" "partner_eu" {
  provider = wso2is.partner
  name     = "Partner EU"
}

variable "partner_admin_password" {
  type      = string
  sensitive = true
}
```

### Why the order matters

Terraform must not configure the organization's provider until the application is shared with the
organization and allowed to call the APIs. Otherwise the organization switch fails. Two details
in the example make sure that this happens in the right order:

* The `partner` provider reads `organization_id` from `wso2is_application_share.b2b_admin`, not
  from `wso2is_organization.partner`. Terraform therefore waits until the share exists.
* `wso2is_application_share.b2b_admin` has `depends_on` for the API authorizations. The share
  therefore waits until the application may call the APIs.

## Share applications with organizations

Use `wso2is_application_share` to share a root application with organizations. You have two
choices:

* List the organizations in `organization_ids`.
* Set `share_with_all_children = true`. This choice also covers organizations that you create later.

## Roles inside an organization

A role inside an organization uses the organization's own permission names. For example,
`internal_org_user_mgt_view` allows a user to view the organization's users. To create such a role,
use `wso2is_role` with the organization's provider and leave out `audience_type`.

## Send users to their organization with email domains

The login page can send users to the right organization based on their email address. For
example, users with an `@partner.example` address go to Partner Inc's login. This feature is
called **email domain discovery**.

```terraform
# Turn on email domain discovery for the tenant first.
resource "wso2is_organization_discovery_configuration" "this" {
  email_domain_enabled = true
}

resource "wso2is_organization" "partner" {
  name = "Partner Inc"
}

# Users with an @partner.example or @partner-eu.example address go to Partner Inc's login.
resource "wso2is_organization_discovery" "partner" {
  organization_id = wso2is_organization.partner.id
  email_domains   = ["partner.example", "partner-eu.example"]
  depends_on      = [wso2is_organization_discovery_configuration.this]
}
```

## Give a root user access to organizations

Sometimes a person from your own company, such as a support engineer, needs to work inside a
customer's organization. `wso2is_user_share` gives a user of the root organization access to
organizations, with roles there. The person does not need a second account. See the
`wso2is_user_share` page for an example.

## Manage many organizations

Terraform does not allow `for_each` on provider blocks. If you manage many organizations, choose
one of these approaches:

* Write a module for the contents of one organization. Call the module once per organization, and
  pass each call its own provider alias.
* Use a separate configuration (or workspace) per organization, and set
  `WSO2IS_ORGANIZATION_ID` for each one.

## Limitations

* **Sub-organizations are created from their parent.** To create "Partner EU" inside "Partner
  Inc", use the provider of "Partner Inc". The server rejects the request from the root
  organization with `ORG-60021`. For this reason, `parent_id` is read-only.
* **Replacing the organization or the application needs two steps.** When Terraform replaces one
  of them, the organization's provider depends on values that are only known after the apply.
  Planning changes to existing objects inside the organization then fails with a message about
  an unknown provider configuration. To fix this, apply the replacement first with
  `terraform apply -target=wso2is_application_share.<name>`, and then run a normal apply.
* **An organization with sub-organizations cannot be deleted.** Delete the sub-organizations
  first. Terraform does this for you when they are in the same configuration.
