resource "wso2is_branding_text" "login" {
  screen = "login"
  locale = "en-US"
  text = {
    "login.heading" = "Sign in to Acme"
    "login.button"  = "Continue"
  }
}
