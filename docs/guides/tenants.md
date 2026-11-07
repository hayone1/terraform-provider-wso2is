---
page_title: "Tenants"
subcategory: ""
description: |-
  Create tenants, and manage the objects inside a tenant from the same configuration.
---

# Tenants

A **tenant** is a completely separate root organization on the same server. Each tenant has its
own users, applications and settings. Users of one tenant cannot see another tenant.

Every server starts with one tenant, called the super tenant (`carbon.super`). New tenants are
created from the super tenant by its administrator. The provider must use **basic
authentication** (a username and a password) to create tenants.

## Create a tenant and use it

The example below creates the tenant `acme.com`, and then creates a group inside it:

```terraform
# The super tenant's administrator creates tenants. This needs basic authentication.
provider "wso2is" {
  username = "admin"
  password = var.super_admin_password
}

resource "wso2is_tenant" "acme" {
  domain           = "acme.com"
  owner_username   = "admin"
  owner_password   = var.acme_admin_password
  owner_email      = "it@acme.com"
  owner_first_name = "Acme"
  owner_last_name  = "Admin"
}

# A second provider block manages the new tenant. It signs in as the tenant owner.
provider "wso2is" {
  alias         = "acme"
  tenant_domain = wso2is_tenant.acme.domain
  username      = "${wso2is_tenant.acme.owner_username}@${wso2is_tenant.acme.domain}"
  password      = var.acme_admin_password
}

resource "wso2is_group" "acme_staff" {
  provider = wso2is.acme
  name     = "staff"
}

variable "super_admin_password" {
  type      = string
  sensitive = true
}

variable "acme_admin_password" {
  type      = string
  sensitive = true
}
```

The second provider block, with `alias = "acme"`, manages the new tenant. It signs in as the tenant
owner. The owner's username has the form `owner@tenant-domain`, for example `admin@acme.com`.

A new tenant needs a short time before its APIs answer. `wso2is_tenant` waits until the owner can
call the APIs. For this reason, you can create objects inside the tenant in the same
`terraform apply`.

To automate changes inside the tenant without the owner's password, create an M2M application in
the tenant. Follow the [Authentication guide](authentication), but use the tenant's provider.

## Delete tenants

By default, the server does not delete tenants through the API. To allow it, add this setting to
the server's `deployment.toml`:

```toml
[tenant_mgt]
tenant_deletion = true
```

Without this setting, destroying `wso2is_tenant` only deactivates the tenant, and the provider
shows a warning. A deactivated tenant still owns its domain. You cannot create a new tenant with
the same domain until the old tenant is deleted.

## Limitations

* **Client credentials cannot create tenants.** The server fails with `TM-65002` when it creates a
  tenant with a client credentials token. When this happens, the provider deletes the half-created
  tenant and tells you to use basic authentication.
* **A re-created tenant needs time.** Suppose you delete a tenant and create a new tenant with the
  same domain shortly after. The server keeps cached data about the deleted tenant for about 15
  minutes, or until it restarts. During this time, the new tenant does not work correctly.
  `wso2is_tenant` warns you when this happens.
* **The super tenant's email provider lives in `deployment.toml`.** `wso2is_email_sender` does not
  work in the super tenant. Set up email for the super tenant in `deployment.toml` instead.
