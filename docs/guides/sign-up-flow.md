---
page_title: "Custom registration (sign-up) flow"
subcategory: ""
description: |-
  Add your own fields to the self sign-up page with a registration flow, a custom attribute (claim) and an OIDC scope.
---

# Custom registration (sign-up) flow

WSO2 Identity Server 7.3 builds its self sign-up page from a **registration flow**. A flow is a
graph of steps. Some steps are **views**, which show a form to the user. Other steps run
**executors**, which do work on the server, such as creating the user. In the Console, you edit
this graph with the flow builder. With Terraform, you manage the same graph with `wso2is_flow`.
This lets you keep your sign-up form in version control.

In this guide, you add a required *membership number* field to the sign-up form. You then:

1. store the membership number on the user as a custom attribute
2. show the attribute in the SCIM user APIs and in My Account
3. release the attribute in ID tokens through a new `membership` scope

## The complete example

```terraform
terraform {
  required_providers {
    wso2is = {
      source = "hayone1/wso2is"
    }
  }
}

provider "wso2is" {}

# 1. The new attribute. A local claim stores it in the user store.
resource "wso2is_claim" "membership_number" {
  claim_uri    = "http://wso2.org/claims/membershipNumber"
  display_name = "Membership number"
  regex        = "^M-[0-9]{4,8}$"
  attribute_mappings = {
    PRIMARY = "membershipNumber"
  }
}

# A SCIM claim shows the attribute in the user APIs and in My Account.
resource "wso2is_external_claim" "membership_number_scim" {
  dialect_uri            = "urn:scim:schemas:extension:custom:User"
  claim_uri              = "urn:scim:schemas:extension:custom:User:membershipNumber"
  mapped_local_claim_uri = wso2is_claim.membership_number.claim_uri
}

# 2. The registration flow. Each field's identifier is the local claim URI that stores
# the value, or "password" for the password. While the flow is enabled, the login page
# shows a "Create an account" link.
resource "wso2is_flow" "sign_up" {
  flow_type = "REGISTRATION"
  enabled   = true
  completion_configs = {
    isAutoLoginEnabled         = "true"
    isEmailVerificationEnabled = "false"
  }
  steps = jsonencode([
    {
      id       = "sign_up"
      type     = "VIEW"
      size     = { width = 350, height = 560 }
      position = { x = 300, y = 200 }
      data = {
        components = [
          { id = "title", category = "DISPLAY", type = "TYPOGRAPHY", variant = "H3", config = { text = "Become a member" } },
          {
            id = "form", category = "BLOCK", type = "FORM", config = {}
            components = [
              { id = "username", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Username", required = true, identifier = "http://wso2.org/claims/username" } },
              { id = "email", category = "FIELD", type = "INPUT", variant = "EMAIL", config = { type = "email", label = "Email", required = true, identifier = "http://wso2.org/claims/emailaddress" } },
              { id = "membership", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Membership number", placeholder = "M-1234", required = true, identifier = wso2is_claim.membership_number.claim_uri } },
              { id = "password", category = "FIELD", type = "INPUT", variant = "PASSWORD", config = { type = "password", label = "Password", required = true, identifier = "password" } },
              {
                id     = "submit", category = "ACTION", type = "BUTTON", variant = "PRIMARY", config = { type = "submit", text = "Create account" }
                action = { type = "EXECUTOR", executor = { name = "PasswordProvisioningExecutor" }, next = "END" }
              },
            ]
          },
        ]
      }
    },
    {
      id       = "END"
      type     = "END"
      size     = { width = 200, height = 100 }
      position = { x = 800, y = 200 }
      data     = { components = [], action = { type = "EXECUTOR", executor = { name = "UserProvisioningExecutor" } } }
    },
  ])
  depends_on = [wso2is_external_claim.membership_number_scim]
}

# 3. Optional: release the attribute in ID tokens through a "membership" scope.
resource "wso2is_external_claim" "membership_number_oidc" {
  dialect_uri            = "http://wso2.org/oidc/claim"
  claim_uri              = "membership_number"
  mapped_local_claim_uri = wso2is_claim.membership_number.claim_uri
}

resource "wso2is_oidc_scope" "membership" {
  name         = "membership"
  display_name = "Membership"
  claims       = [wso2is_external_claim.membership_number_oidc.claim_uri]
}
```

## How the parts fit together

**Form fields.** Each field is an `INPUT` component inside a `FORM` block. The field's
`config.identifier` names the place where the server stores the value. For most fields, this is
a local claim URI, such as `http://wso2.org/claims/emailaddress`. For the password field, the
identifier is `password`.

**The submit button.** The button runs the `PasswordProvisioningExecutor`, and then moves to the
`END` step. The `END` step runs the `UserProvisioningExecutor`, which creates the user.

**The custom attribute.** A new attribute needs two claims:

* a **local claim** that maps the attribute to a column in the user store, and
* a mapping in the `urn:scim:schemas:extension:custom:User` SCIM dialect, so that the user APIs
  return the attribute.

If the server does not have the custom SCIM dialect yet, `wso2is_external_claim` creates it.

**The "Create an account" link.** While the flow is enabled, the hosted login page shows a "Create
an account" link. The link opens `/accounts/register?flowType=REGISTRATION`. The Console
application does not show the link. The older `self-sign-up` governance connector does not control
this page.

**What happens after sign-up.** `completion_configs` controls what happens after a user signs up.
The available settings are `isAutoLoginEnabled`, `isEmailVerificationEnabled`,
`isAccountLockOnCreationEnabled` and `isFlowCompletionNotificationEnabled`. Email verification and
notifications only work after you set up an email provider.

## Design the flow in the Console first

Writing the flow's JSON by hand takes time. It is often faster to design the flow in the
Console's flow builder, and then copy the result into Terraform:

1. Design and save the flow in the Console.
2. Read the flow with a data source and print it:

   ```terraform
   data "wso2is_flow" "designed" {
     flow_type = "REGISTRATION"
   }

   output "registration_flow" {
     value = data.wso2is_flow.designed.steps
   }
   ```

3. Save the output to a file:

   ```shell
   terraform output -raw registration_flow > registration-flow.json
   ```

4. Use the file in your `wso2is_flow` resource with `steps = file("registration-flow.json")`.

The provider compares flows by their content. A different key order or different formatting does
not show up as a change in the plan.

## Other flows

`flow_type` also accepts `PASSWORD_RECOVERY` and `INVITED_USER_REGISTRATION`. You build these flows
in the same way.

## Limitations

* Each tenant (or organization) has exactly one flow of each type.
* By default, WSO2 IS 7.3.0 does not allow the API to delete a flow. When you destroy
  `wso2is_flow`, the provider turns the flow off instead and shows a warning. To allow deletion,
  change `deployment.toml` as described in
  [Known server behaviour](server-behaviour#flows).
