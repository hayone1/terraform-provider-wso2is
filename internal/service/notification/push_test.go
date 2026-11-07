package notification_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccPushSender(t *testing.T) {
	cfg := func(region string, fcmDefault bool) string {
		return fmt.Sprintf(`
resource "wso2is_push_sender" "fcm" {
  provider_name = "FCM"
  default       = %[2]t
  fcm = {
    service_account_json = jsonencode({
      type         = "service_account"
      project_id   = "tfacc-demo"
      client_email = "push@tfacc-demo.iam.gserviceaccount.com"
    })
  }
}

resource "wso2is_push_sender" "sns" {
  provider_name = "AmazonSNS"
  default       = %[3]t
  amazon_sns = {
    access_key_id     = "AKIATFACCEXAMPLE"
    secret_access_key = "tfacc-secret"
    region            = %[1]q
    platform_arns = {
      FCM  = "arn:aws:sns:%[1]s:123456789012:app/GCM/tfacc"
      APNS = "arn:aws:sns:%[1]s:123456789012:app/APNS/tfacc"
    }
  }
  # One default at a time: switch in a deterministic order.
  depends_on = [wso2is_push_sender.fcm]
}
`, region, fcmDefault, !fcmDefault)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: func(*terraform.State) error {
			c := acctest.Client(t)
			var cfg map[string]any
			if err := c.Get(acctest.Context(t), c.ServerPath("/notification-senders/configs/push"), nil, &cfg); err != nil {
				return err
			}
			if v, ok := cfg["defaultPushProvider"]; ok && v != nil {
				return fmt.Errorf("default push provider left behind: %v", v)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: cfg("eu-west-1", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_push_sender.fcm", "id", "FCM_PushPublisher"),
					resource.TestCheckResourceAttr("wso2is_push_sender.fcm", "default", "true"),
					resource.TestCheckResourceAttr("wso2is_push_sender.sns", "amazon_sns.platform_arns.%", "2"),
				),
			},
			{
				Config: cfg("us-east-1", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_push_sender.sns", "amazon_sns.region", "us-east-1"),
					resource.TestCheckResourceAttr("wso2is_push_sender.sns", "default", "true"),
				),
			},
			{ResourceName: "wso2is_push_sender.fcm", ImportState: true, ImportStateId: "FCM_PushPublisher", ImportStateVerify: true, ImportStateVerifyIgnore: []string{"fcm.service_account_json"}},
			{ResourceName: "wso2is_push_sender.sns", ImportState: true, ImportStateId: "AmazonSNS_PushPublisher", ImportStateVerify: true},
		},
	})
}
