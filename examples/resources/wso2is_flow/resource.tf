# Custom self-registration form with an extra attribute.
resource "wso2is_flow" "sign_up" {
  flow_type = "REGISTRATION"
  completion_configs = {
    isEmailVerificationEnabled = "false"
    isAutoLoginEnabled         = "true"
  }
  steps = jsonencode([
    {
      id       = "sign_up_form"
      type     = "VIEW"
      size     = { width = 350, height = 500 }
      position = { x = 300, y = 200 }
      data = {
        components = [
          { id = "heading", category = "DISPLAY", type = "TYPOGRAPHY", variant = "H3", config = { text = "Create your account" } },
          {
            id = "form", category = "BLOCK", type = "FORM", config = {}
            components = [
              { id = "username", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Username", required = true, identifier = "http://wso2.org/claims/username" } },
              { id = "email", category = "FIELD", type = "INPUT", variant = "EMAIL", config = { type = "email", label = "Email", required = true, identifier = "http://wso2.org/claims/emailaddress" } },
              { id = "member_no", category = "FIELD", type = "INPUT", variant = "TEXT", config = { type = "text", label = "Membership number", required = false, identifier = "http://wso2.org/claims/membershipNumber" } },
              { id = "password", category = "FIELD", type = "INPUT", variant = "PASSWORD", config = { type = "password", label = "Password", required = true, identifier = "password" } },
              { id = "submit", category = "ACTION", type = "BUTTON", variant = "PRIMARY", config = { type = "submit", text = "Sign up" },
              action = { type = "EXECUTOR", executor = { name = "PasswordProvisioningExecutor" }, next = "END" } },
            ]
          },
        ]
      }
    },
    {
      id       = "END"
      type     = "END"
      size     = { width = 200, height = 100 }
      position = { x = 800, y = 200 }
      data     = { action = { type = "EXECUTOR", executor = { name = "UserProvisioningExecutor" } } }
    },
  ])
}
