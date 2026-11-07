# Not available in the super tenant (carbon.super): configure [output_adapter.email]
# in deployment.toml there. Use a provider configured for another tenant.
provider "wso2is" {
  alias         = "acme"
  tenant_domain = "acme.com"
  username      = "admin@acme.com"
  password      = var.acme_admin_password
}

resource "wso2is_email_sender" "smtp" {
  provider     = wso2is.acme
  smtp_host    = "smtp.sendgrid.net"
  smtp_port    = 587
  from_address = "no-reply@acme.com"
  display_name = "Acme"
  reply_to     = "support@acme.com"
  username     = "apikey"
  password     = var.smtp_password
  properties = {
    "mail.smtp.starttls.enable" = "true"
  }
}

variable "acme_admin_password" {
  type      = string
  sensitive = true
}

variable "smtp_password" {
  type      = string
  sensitive = true
}
