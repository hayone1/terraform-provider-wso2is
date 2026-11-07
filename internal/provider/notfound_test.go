package provider_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

// Every object resource must report a missing object as "not found", so that Terraform
// plans to create it again after someone deletes it outside Terraform. The servers answer
// with different status codes and error codes for missing objects; importing an ID that
// does not exist exercises the same read path as a refresh after an out-of-band delete.
func TestAccResources_missingObjectIsNotFound(t *testing.T) {
	const missing = "6f9a2c1e-0b7d-4e8a-9c3f-1d2e3f4a5b6c"
	cases := []struct {
		resource string
		config   string
		importID string
	}{
		{"wso2is_action", `type = "pre_issue_access_token"
  name = "x"
  endpoint_uri = "https://example.com/hook"
  authentication_type = "NONE"`, "pre_issue_access_token/" + missing},
		{"wso2is_api_resource", `name = "x"
  identifier = "https://example.com/x"`, missing},
		{"wso2is_application", `name = "x"`, missing},
		{"wso2is_application_authorized_api", `application_id = "a"
  api_resource_id = "b"
  scopes = []`, missing + "/" + missing},
		{"wso2is_application_share", `application_id = "a"`, missing},
		{"wso2is_claim", `claim_uri = "http://wso2.org/claims/x"
  display_name = "x"
  attribute_mappings = { PRIMARY = "x" }`, "http://wso2.org/claims/tfaccmissing"},
		{"wso2is_claim_dialect", `dialect_uri = "urn:x"`, "urn:tfacc:missing"},
		{"wso2is_custom_authenticator", `name = "x"
  display_name = "x"
  endpoint_uri = "https://example.com/auth"
  endpoint_authentication_type = "NONE"`, "tfacc-missing"},
		{"wso2is_external_claim", `dialect_uri = "urn:x"
  claim_uri = "x"
  mapped_local_claim_uri = "http://wso2.org/claims/x"`, "http://wso2.org/oidc/claim|tfacc_missing"},
		{"wso2is_group", `name = "x"`, missing},
		{"wso2is_group_members", `group_id = "x"
  user_ids = []`, missing},
		{"wso2is_identity_provider", `name = "x"`, missing},
		{"wso2is_identity_verification_provider", `name = "x"
  type = "ONFIDO"`, missing},
		{"wso2is_notification_template", `channel = "email"
  type = "AccountConfirmation"
  body = "x"`, "email/AccountConfirmation/zu_ZA"},
		{"wso2is_notification_template_type", `channel = "email"
  name = "x"`, "email/TfaccMissingType"},
		{"wso2is_oidc_scope", `name = "x"
  display_name = "x"`, "tfacc_missing"},
		{"wso2is_organization", `name = "x"`, missing},
		{"wso2is_organization_discovery", `organization_id = "x"
  email_domains = ["example.com"]`, missing},
		{"wso2is_role", `name = "x"`, missing},
		{"wso2is_role_assignment", `role_id = "x"`, missing},
		{"wso2is_script_library", `name = "x.js"
  content = "x"`, "tfacc-missing.js"},
		{"wso2is_secret", `name = "x"
  value = "x"`, "ADAPTIVE_AUTH_CALL_CHOREO/tfacc_missing"},
		{"wso2is_user", `username = "x"`, missing},
		{"wso2is_user_share", `user_id = "x"`, missing},
		{"wso2is_userstore", `name = "X"
  type = "UniqueIDJDBCUserStoreManager"
  properties = {}`, "TFACC-MISSING"},
		{"wso2is_vc_template", `identifier = "x"
  display_name = "x"
  expires_in = 60
  claims = { x = "http://wso2.org/claims/x" }`, missing},
		{"wso2is_webhook", `name = "x"
  endpoint = "https://example.com/hook"
  channels = ["x"]`, missing},
		{"wso2is_workflow", `name = "x"
  steps = [{ role_ids = ["x"] }]`, missing},
		{"wso2is_workflow_association", `name = "x"
  workflow_id = "x"
  operation = "ADD_USER"`, missing},
	}
	for _, tc := range cases {
		t.Run(tc.resource, func(t *testing.T) {
			t.Parallel()
			acctest.Test(t, resource.TestCase{
				PreCheck:                 func() { acctest.PreCheck(t) },
				ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					// Import-only steps get no generated required_providers block, so name
					// the source; the test tasks register the provider as hayone1/wso2is.
					Config: "terraform {\n  required_providers {\n    wso2is = { source = \"hayone1/wso2is\" }\n  }\n}\n\n" +
						"resource \"" + tc.resource + "\" \"test\" {\n  " + tc.config + "\n}\n",
					ResourceName:  tc.resource + ".test",
					ImportState:   true,
					ImportStateId: tc.importID,
					ExpectError:   regexp.MustCompile(`Cannot import non-existent remote object`),
				}},
			})
		})
	}
}
