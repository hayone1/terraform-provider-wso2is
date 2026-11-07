resource "wso2is_notification_template" "account_confirmation_fr" {
  channel = "email"
  type    = "AccountConfirmation"
  locale  = "fr_FR"
  subject = "Confirmez votre compte"
  body    = "<p>Bonjour {{user-name}}, confirmez votre compte : <a href=\"{{carbon.product-url}}/accountrecoveryendpoint/confirmregistration.do?confirmation={{confirmation-code}}\">confirmer</a></p>"
}

resource "wso2is_notification_template" "sms_otp" {
  channel = "sms"
  type    = "SMSOTP"
  body    = "Your Acme code is {{confirmation-code}}"
}
