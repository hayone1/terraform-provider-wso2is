resource "wso2is_branding_preference" "org" {
  locale = "en-US"
  preference = jsonencode({
    configs = { isBrandingEnabled = true }
    organizationDetails = {
      displayName  = "Acme"
      supportEmail = "support@acme.example"
    }
    images = {
      logo = { imgURL = "https://cdn.acme.example/logo.svg", altText = "Acme" }
    }
    theme = {
      activeTheme = "LIGHT"
      LIGHT       = { colors = { primary = { main = "#FF7300" } } }
    }
    layout = { activeLayout = "centered" }
  })
}
