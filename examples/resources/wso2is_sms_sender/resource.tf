resource "wso2is_sms_sender" "twilio" {
  provider_name = "Twilio"
  provider_url  = "https://api.twilio.com/2010-04-01/Accounts/ACxxxx/Messages.json"
  key           = "ACxxxx"
  secret        = var.twilio_token
  sender        = "+15550000000"
  content_type  = "FORM"
}

variable "twilio_token" {
  type      = string
  sensitive = true
}
