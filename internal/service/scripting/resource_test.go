package scripting_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccScriptLibraryAndSecret(t *testing.T) {
	name := strings.ReplaceAll(acctest.RandomName("lib"), "-", "_")
	cfg := func(threshold, desc string) string {
		return fmt.Sprintf(`
resource "wso2is_script_library" "risk" {
  name        = "%[1]s.js"
  description = %[3]q
  content     = <<-EOT
    var THRESHOLD = %[2]s;
    function isRisky(score) {
      return score > THRESHOLD;
    }
    module.exports.isRisky = isRisky;
  EOT
}

resource "wso2is_secret" "api_key" {
  name        = "%[1]s_key"
  value       = "risk-api-key"
  description = %[3]q
}

resource "wso2is_application" "app" {
  name = %[1]q
  authentication_sequence = {
    steps = [
      { options = [{ idp = "LOCAL", authenticator = "BasicAuthenticator" }] },
      { options = [{ idp = "LOCAL", authenticator = "totp" }] },
    ]
    script = <<-EOT
      var risk = require('${wso2is_script_library.risk.name}');
      var onLoginRequest = function(context) {
        executeStep(1, {
          onSuccess: function(context) {
            httpGet('https://risk.example.com/score', { 'Authorization': 'Bearer ' + secrets.${wso2is_secret.api_key.name} }, {
              onSuccess: function(context, data) {
                if (risk.isRisky(data.score)) { executeStep(2); }
              }
            });
          }
        });
      };
    EOT
  }
}`, name, threshold, desc)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("50", "Risk helpers"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_script_library.risk", "name", name+".js"),
					resource.TestCheckResourceAttrSet("wso2is_secret.api_key", "secret_id"),
					resource.TestCheckResourceAttr("wso2is_secret.api_key", "type", "ADAPTIVE_AUTH_CALL_CHOREO"),
				),
			},
			{
				Config: cfg("80", "Risk helpers v2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_script_library.risk", "description", "Risk helpers v2"),
					resource.TestCheckResourceAttr("wso2is_secret.api_key", "description", "Risk helpers v2"),
				),
			},
			{ResourceName: "wso2is_script_library.risk", ImportState: true, ImportStateId: name + ".js", ImportStateVerify: true},
			{ResourceName: "wso2is_secret.api_key", ImportState: true, ImportStateId: "ADAPTIVE_AUTH_CALL_CHOREO/" + name + "_key", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"value"}},
		},
	})
}
