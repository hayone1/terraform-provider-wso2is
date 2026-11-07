---
page_title: "Getting started"
subcategory: ""
description: |-
  Start WSO2 Identity Server on your computer and use Terraform to create your first application, API, role, group and user.
---

# Getting started

In this guide, you start WSO2 Identity Server on your own computer. Then you use Terraform to set
up a small "Notes" app. When you finish, you will have:

* an **API** called "Notes API" with two scopes, `notes:read` and `notes:write`
* a **single-page application** that signs users in and may call the API
* an **application role** called "editor" that grants both scopes
* a **group** called "editors" that has the editor role
* a **user** called Alice who belongs to the group

The guide takes about ten minutes.

## Before you begin

You need these tools on your computer:

* [Docker](https://docs.docker.com/get-docker/), to run the server
* [Terraform](https://developer.hashicorp.com/terraform/install) 1.0 or later, or
  [OpenTofu](https://opentofu.org/docs/intro/install/) 1.6 or later

## Step 1: Start the server

Run this command to start WSO2 Identity Server 7.3.0 in a Docker container:

```shell
docker run -d --name wso2is -p 9443:9443 wso2/wso2is:7.3.0
```

The server needs about one minute to start. It is ready when
[https://localhost:9443/console](https://localhost:9443/console) shows a sign-in page. Your browser
warns you about the certificate, because the server uses a self-signed certificate. You can accept
the warning for this local test. Sign in with the username `admin` and the password `admin`.

This container keeps its data only while it runs. For a server that other people use, you need a
real database and a trusted certificate. The `deploy/` folder in the provider's GitHub repository
has a Kubernetes setup with PostgreSQL that you can start from.

## Step 2: Write the configuration

Create an empty folder and save the following file in it as `main.tf`. The comments explain what
each block does.

```terraform
terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

# Connect to the local server that you started with:
#   docker run -d --name wso2is -p 9443:9443 wso2/wso2is:7.3.0
provider "wso2is" {
  base_url = "https://localhost:9443"
  username = "admin"
  password = "admin"
  # The local server uses a self-signed certificate. Only skip the check on a test server.
  insecure_skip_verify = true
}

# The API that the application calls. Each scope is one permission in the API.
resource "wso2is_api_resource" "notes" {
  name       = "Notes API"
  identifier = "https://api.example.com/notes"
  scopes = [
    { name = "notes:read", display_name = "Read notes" },
    { name = "notes:write", display_name = "Write notes" },
  ]
}

# A single-page application that signs users in with OIDC.
resource "wso2is_application" "notes_spa" {
  name        = "Notes"
  template_id = "single-page-application"
  oidc = {
    grant_types     = ["authorization_code", "refresh_token"]
    public_client   = true
    callback_urls   = ["http://localhost:3000"]
    allowed_origins = ["http://localhost:3000"]
    pkce = {
      mandatory = true
    }
  }
  # The application uses its own roles, such as "editor" below.
  associated_roles = {
    allowed_audience = "APPLICATION"
  }
}

# Allow the application to request both scopes of the API.
resource "wso2is_application_authorized_api" "notes" {
  application_id  = wso2is_application.notes_spa.id
  api_resource_id = wso2is_api_resource.notes.id
  scopes          = ["notes:read", "notes:write"]
}

# An application role that grants both scopes. A role can only grant scopes that the
# application may request, so the role waits for the authorization above.
resource "wso2is_role" "editor" {
  name          = "editor"
  audience_type = "application"
  audience_id   = wso2is_application.notes_spa.id
  permissions   = ["notes:read", "notes:write"]
  depends_on    = [wso2is_application_authorized_api.notes]
}

# A group whose members get the editor role.
resource "wso2is_group" "editors" {
  name = "editors"
}

resource "wso2is_role_assignment" "editors" {
  role_id   = wso2is_role.editor.id
  group_ids = [wso2is_group.editors.id]
}

# A user who can sign in to the application.
resource "wso2is_user" "alice" {
  username    = "alice"
  given_name  = "Alice"
  family_name = "Smith"
  email       = "alice@example.com"
  password    = "Alice-Passw0rd!"
}

# Add Alice to the editors group.
resource "wso2is_group_members" "editors" {
  group_id = wso2is_group.editors.id
  user_ids = [wso2is_user.alice.id]
}

# Use this client ID in your application's OIDC settings.
output "client_id" {
  value = wso2is_application.notes_spa.oidc.client_id
}
```

A few details are worth knowing:

* The provider block uses the `admin` account and turns off the certificate check. Both are fine
  for a local test. The [Authentication guide](authentication) shows a safer setup for real
  servers.
* `wso2is_application_authorized_api` allows the application to request the API's scopes. The
  role can only grant scopes that the application is allowed to request. That is why the role
  uses `depends_on`.
* `associated_roles.allowed_audience = "APPLICATION"` tells the server that the application uses
  its own roles, such as "editor", instead of roles shared by the whole organization.

## Step 3: Create everything

Open a terminal in the folder and run these commands:

```shell
terraform init
terraform apply
```

`terraform init` downloads the provider. `terraform apply` shows the plan and asks you to confirm.
Type `yes`. Terraform then creates the objects and prints the application's client ID.

## Step 4: Check the result

1. Open [https://localhost:9443/console](https://localhost:9443/console) and sign in as `admin`.
2. Go to **Applications**. You should see the "Notes" application.
3. Go to **User Management > Users**. You should see Alice.
4. Open [https://localhost:9443/myaccount](https://localhost:9443/myaccount) in a private browser
   window. Sign in as `alice` with the password from `main.tf`.

To use the application in your own code, copy the `client_id` output into your app's OIDC
settings.

## Step 5: Make a change

Change Alice's `given_name` in `main.tf`, and then run `terraform apply` again. The plan shows
one change to `wso2is_user.alice`. Terraform updates only that attribute.

Now change Alice's first name in the Console instead, and run `terraform plan`. The plan shows that
the server no longer matches your configuration. Run `terraform apply` to set the name back.

## Step 6: Clean up

Run this command to delete everything that Terraform created:

```shell
terraform destroy
```

Then stop and remove the server:

```shell
docker rm -f wso2is
```

## Next steps

* Stop using the administrator password. [Create an application for Terraform](authentication).
* Serve business customers with [organizations](organizations).
* Collect extra details when users sign up with a [custom registration flow](sign-up-flow).
* Learn about [server behaviour that affects your configuration](server-behaviour).
