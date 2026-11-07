package config_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

// Server configurations are tenant-wide; the last step puts the defaults back.
func TestAccServerConfigurations(t *testing.T) {
	cfg := func(idle, remember, maxAge, banner string, signing bool) string {
		return `
resource "wso2is_server_configuration" "this" {
  idle_session_timeout   = ` + idle + `
  remember_me_period     = ` + remember + `
  home_realm_identifiers = ["lab.example", "lab2.example"]
}

resource "wso2is_inbound_provisioning_configuration" "this" {
  proxy_mode = false
}

resource "wso2is_cors_configuration" "this" {
  supports_credentials = true
  max_age              = ` + maxAge + `
  exposed_headers      = ["Location", "X-Trace-Id"]
}

resource "wso2is_impersonation_configuration" "this" {
  enable_email_notification = false
}

resource "wso2is_jwt_key_validator_configuration" "this" {
  enable_token_reuse = true
}

resource "wso2is_dcr_configuration" "this" {
  authentication_required = true
}

resource "wso2is_saml_idp_configuration" "this" {
  metadata_validity_period = 120
  enable_metadata_signing  = ` + boolStr(signing) + `
}

resource "wso2is_passive_sts_configuration" "this" {
  enable_request_signing = ` + boolStr(signing) + `
}

resource "wso2is_oauth2_configuration" "this" {
  enable_jwt_scope_as_array = ` + boolStr(signing) + `
}

resource "wso2is_admin_advisory_banner" "this" {
  enabled = ` + boolStr(signing) + `
  content = "` + banner + `"
}

resource "wso2is_fraud_detection_configuration" "this" {
  log_request_payload = false
}

resource "wso2is_remote_logging" "audit" {
  log_type               = "AUDIT"
  remote_url             = "https://logs.example.com/audit"
  connect_timeout_millis = "5000"
  username               = "logger"
  password               = "logger-secret"
}
`
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("30", "10080", "3600", "Lab system: authorized use only", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_server_configuration.this", "idle_session_timeout", "30"),
					resource.TestCheckResourceAttr("wso2is_cors_configuration.this", "max_age", "3600"),
					resource.TestCheckTypeSetElemAttr("wso2is_cors_configuration.this", "exposed_headers.*", "Location"),
					resource.TestCheckResourceAttr("wso2is_saml_idp_configuration.this", "enable_metadata_signing", "true"),
					resource.TestCheckResourceAttr("wso2is_admin_advisory_banner.this", "enabled", "true"),
					resource.TestCheckResourceAttr("wso2is_remote_logging.audit", "remote_url", "https://logs.example.com/audit"),
				),
			},
			{ResourceName: "wso2is_server_configuration.this", ImportState: true, ImportStateId: "server_configuration", ImportStateVerify: true},
			{ResourceName: "wso2is_cors_configuration.this", ImportState: true, ImportStateId: "cors_configuration", ImportStateVerify: true},
			{
				// Back to the server defaults.
				Config: cfg("15", "20160", "-1", "Warning - unauthorized use of this tool is strictly prohibited. All activities performed using this tool are logged and monitored.", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_server_configuration.this", "idle_session_timeout", "15"),
					resource.TestCheckResourceAttr("wso2is_admin_advisory_banner.this", "enabled", "false"),
				),
			},
		},
	})
}

// A provisioning user store forces every SCIM user into it, so destroying the resource
// must clear it. Runs in its own tenant: the setting would break secondary user stores
// in parallel tests.
func TestAccInboundProvisioning_resetOnDestroy(t *testing.T) {
	acctest.PreCheck(t)
	domain, user, pass := acctest.TestTenant(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			c := acctest.TenantClient(t, domain, user, pass)
			var doc map[string]any
			if err := c.Get(acctest.Context(t), c.ServerPath("/configs/provisioning/inbound/scim"), nil, &doc); err != nil {
				return err
			}
			if us, _ := doc["provisioningUserstore"].(string); us != "" {
				return fmt.Errorf("provisioning user store not reset on destroy: %q", us)
			}
			return nil
		},
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
provider "wso2is" {
  tenant_domain = %q
  username      = %q
  password      = %q
}

resource "wso2is_inbound_provisioning_configuration" "this" {
  provisioning_userstore = "PRIMARY"
}`, domain, user, pass),
			Check: resource.TestCheckResourceAttr("wso2is_inbound_provisioning_configuration.this", "provisioning_userstore", "PRIMARY"),
		}},
	})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
