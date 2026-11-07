# Firebase Cloud Messaging, as the default provider.
resource "wso2is_push_sender" "fcm" {
  provider_name = "FCM"
  default       = true
  fcm = {
    service_account_json = var.firebase_service_account_json # file("service-account.json")
  }
}

# Amazon SNS, for devices that register with it.
resource "wso2is_push_sender" "sns" {
  provider_name = "AmazonSNS"
  amazon_sns = {
    access_key_id     = var.aws_access_key_id
    secret_access_key = var.aws_secret_access_key
    region            = "eu-west-1"
    platform_arns = {
      FCM  = "arn:aws:sns:eu-west-1:123456789012:app/GCM/acme-authenticator"
      APNS = "arn:aws:sns:eu-west-1:123456789012:app/APNS/acme-authenticator"
    }
  }
}

variable "firebase_service_account_json" {
  type      = string
  sensitive = true
}

variable "aws_access_key_id" {
  type = string
}

variable "aws_secret_access_key" {
  type      = string
  sensitive = true
}
