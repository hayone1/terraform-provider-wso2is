package flow_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
	"github.com/hayone1/terraform-provider-wso2is/internal/client"
)

const flowConfig = `
resource "wso2is_claim" "membership" {
  claim_uri    = "http://wso2.org/claims/%[1]s"
  display_name = "Membership number"
  attribute_mappings = { PRIMARY = "%[1]s" }
}

resource "wso2is_external_claim" "membership_scim" {
  dialect_uri            = "urn:scim:schemas:extension:custom:User"
  claim_uri              = "urn:scim:schemas:extension:custom:User:%[1]s"
  mapped_local_claim_uri = wso2is_claim.membership.claim_uri
}

resource "wso2is_flow" "signup" {
  flow_type = "REGISTRATION"
  completion_configs = {
    isAutoLoginEnabled = %[3]q
  }
  steps = jsonencode([
    {
      id       = "view_signup"
      type     = "VIEW"
      size     = { width = 350, height = 500 }
      position = { x = 300, y = 200 }
      data = {
        components = [
          { id = "heading", category = "DISPLAY", type = "TYPOGRAPHY", variant = "H3", config = { text = %[2]q } },
          {
            id = "form", category = "BLOCK", type = "FORM", config = {}
            components = [
              { id = "f_username", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Username", required = true, identifier = "http://wso2.org/claims/username" } },
              { id = "f_email", category = "FIELD", type = "INPUT", variant = "EMAIL", config = { type = "email", label = "Email", required = true, identifier = "http://wso2.org/claims/emailaddress" } },
              { id = "f_member", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Membership number", required = true, identifier = wso2is_claim.membership.claim_uri } },
              { id = "f_password", category = "FIELD", type = "INPUT", variant = "PASSWORD", config = { type = "password", label = "Password", required = true, identifier = "password" } },
              { id = "btn_signup", category = "ACTION", type = "BUTTON", variant = "PRIMARY", config = { type = "submit", text = "Sign up" },
                action = { type = "EXECUTOR", executor = { name = "PasswordProvisioningExecutor" }, next = "END" } },
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
  depends_on = [wso2is_external_claim.membership_scim]
}
`

func TestAccFlow_customSignUp(t *testing.T) {
	claim := strings.ReplaceAll(acctest.RandomName("member"), "-", "")
	username := claim + "user"
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(flowConfig, claim, "Join the club", "false"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_flow.signup", "enabled", "true"),
					resource.TestCheckResourceAttr("wso2is_flow.signup", "completion_configs.isAutoLoginEnabled", "false"),
					signUpThroughFlow(t, username, claim),
				),
			},
			{
				Config: fmt.Sprintf(flowConfig, claim, "Create your membership", "true"),
				Check:  resource.TestCheckResourceAttr("wso2is_flow.signup", "completion_configs.isAutoLoginEnabled", "true"),
			},
			{
				ResourceName:            "wso2is_flow.signup",
				ImportState:             true,
				ImportStateId:           "REGISTRATION",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"completion_configs", "steps"},
			},
		},
	})
}

// signUpThroughFlow registers a user with the flow execution API, as the hosted sign-up
// page does, and checks the user and the custom attribute exist.
func signUpThroughFlow(t *testing.T, username, claim string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		c := acctest.Client(t)
		ctx := context.Background()
		var start struct {
			FlowID string `json:"flowId"`
			Type   string `json:"type"`
		}
		resp, err := c.Do(ctx, client.Request{Method: "POST", Path: c.ServerPath("/flow/execute"), Body: map[string]any{"flowType": "REGISTRATION"}, NoAuth: true})
		if err != nil {
			return fmt.Errorf("starting the registration flow: %w", err)
		}
		if err := resp.Decode(&start); err != nil {
			return err
		}
		if start.Type != "VIEW" {
			return fmt.Errorf("expected the sign-up view, got %q", start.Type)
		}
		var done struct {
			FlowStatus string `json:"flowStatus"`
		}
		resp, err = c.Do(ctx, client.Request{Method: "POST", Path: c.ServerPath("/flow/execute"), NoAuth: true, Body: map[string]any{
			"flowId": start.FlowID, "actionId": "btn_signup",
			"inputs": map[string]string{
				"http://wso2.org/claims/username":     username,
				"http://wso2.org/claims/emailaddress": username + "@example.com",
				"http://wso2.org/claims/" + claim:     "M-1234",
				"password":                            "Club-Member-Passw0rd!",
			},
		}})
		if err != nil {
			return fmt.Errorf("submitting the sign-up form: %w", err)
		}
		if err := resp.Decode(&done); err != nil {
			return err
		}
		if done.FlowStatus != "COMPLETE" {
			return fmt.Errorf("expected the flow to complete, got %q (%s)", done.FlowStatus, string(resp.Body))
		}
		users, err := c.FindUsers(ctx, "userName eq "+username)
		if err != nil {
			return err
		}
		if len(users) != 1 {
			return fmt.Errorf("expected the registered user %s, found %d", username, len(users))
		}
		id, _ := users[0]["id"].(string)
		t.Cleanup(func() { _ = c.DeleteUser(context.Background(), id) })
		u, err := c.GetUser(ctx, id)
		if err != nil {
			return err
		}
		v, _ := client.LookupSCIM(u, client.SCIMCustomUser+":"+claim)
		if v != "M-1234" {
			return fmt.Errorf("custom attribute = %v, want M-1234", v)
		}
		return nil
	}
}
