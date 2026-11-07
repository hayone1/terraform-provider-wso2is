package notification_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/hayone1/terraform-provider-wso2is/internal/acctest"
)

func TestAccEmailSender_tenant(t *testing.T) {
	acctest.PreCheck(t)
	domain, user, pass := acctest.TestTenant(t)
	cfg := func(display string) string {
		return fmt.Sprintf(`
provider "wso2is" {
  tenant_domain = %q
  username      = %q
  password      = %q
}

resource "wso2is_email_sender" "smtp" {
  smtp_host    = "mailpit"
  smtp_port    = 1025
  from_address = "no-reply@%[1]s"
  display_name = %[4]q
  reply_to     = "support@%[1]s"
  username     = "mailer"
  password     = "mailer-secret"
  properties = {
    "mail.smtp.starttls.enable" = "false"
  }
}`, domain, user, pass, display)
	}
	acctest.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Acme Identity"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_email_sender.smtp", "id", "EmailPublisher"),
					resource.TestCheckResourceAttr("wso2is_email_sender.smtp", "provider_type", "SMTP"),
					resource.TestCheckResourceAttr("wso2is_email_sender.smtp", "auth_type", "BASIC"),
					resource.TestCheckResourceAttr("wso2is_email_sender.smtp", "display_name", "Acme Identity"),
				),
			},
			{
				Config: cfg("Acme IAM"),
				Check:  resource.TestCheckResourceAttr("wso2is_email_sender.smtp", "display_name", "Acme IAM"),
			},
		},
	}, acctest.UpjetReadProviderConfig(map[string]any{"tenant_domain": domain, "username": user, "password": pass}))
}

func TestAccEmailSender_superTenantRejected(t *testing.T) {
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: `
resource "wso2is_email_sender" "smtp" {
  smtp_host    = "smtp.example.com"
  smtp_port    = 587
  from_address = "no-reply@example.com"
  display_name = "Example"
  reply_to     = "support@example.com"
  username     = "u"
  password     = "p"
}`,
			ExpectError: regexp.MustCompile(`deployment.toml`),
		}},
	})
}

func TestAccSMSSenderAndTemplates(t *testing.T) {
	name := acctest.RandomName("tmpl")
	cfg := func(subject, sms string) string {
		return fmt.Sprintf(`
resource "wso2is_sms_sender" "custom" {
  provider_name       = "Custom"
  provider_url        = "https://sms.example.com/send"
  content_type        = "JSON"
  authentication_type = "BASIC"
  authentication_properties = {
    username = "sms-user"
    password = "sms-pass"
  }
  properties = {
    body                 = "{\"to\": \"{{mobile}}\", \"text\": \"{{body}}\"}"
    "http.client.method" = "POST"
  }
}

resource "wso2is_notification_template" "confirm" {
  channel = "email"
  type    = "AccountConfirmation"
  locale  = "fr_FR"
  subject = %[2]q
  body    = "<p>Bonjour {{user-name}}, confirmez votre compte : {{carbon.product-url}}</p>"
  footer  = "---"
}

resource "wso2is_application" "app" {
  name = %[1]q
}

resource "wso2is_notification_template" "app_confirm" {
  channel        = "email"
  type           = "AccountConfirmation"
  locale         = "en_US"
  application_id = wso2is_application.app.id
  subject        = "Welcome to %[1]s"
  body           = "<p>Hi {{user-name}}</p>"
}

resource "wso2is_notification_template_type" "custom" {
  channel = "sms"
  name    = "%[1]sNotice"
}

resource "wso2is_notification_template" "custom_sms" {
  channel = "sms"
  type    = wso2is_notification_template_type.custom.name
  body    = %[3]q
}`, name, subject, sms)
	}
	acctest.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Confirmez votre compte", "Your code is {{otp}}"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_sms_sender.custom", "provider_name", "Custom"),
					resource.TestCheckResourceAttr("wso2is_notification_template.confirm", "content_type", "text/html"),
					resource.TestCheckResourceAttr("wso2is_notification_template.custom_sms", "locale", "en_US"),
				),
			},
			{
				Config: cfg("Activez votre compte", "Code: {{otp}}"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("wso2is_notification_template.confirm", "subject", "Activez votre compte"),
					resource.TestCheckResourceAttr("wso2is_notification_template.custom_sms", "body", "Code: {{otp}}"),
				),
			},
			{ResourceName: "wso2is_notification_template.confirm", ImportState: true, ImportStateId: "email/AccountConfirmation/fr_FR", ImportStateVerify: true},
			{ResourceName: "wso2is_sms_sender.custom", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"authentication_properties", "secret", "key"}},
		},
	})
}
