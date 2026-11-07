# Lock accounts after 5 failed attempts for 15 minutes.
resource "wso2is_governance_connector" "account_lock" {
  name = "account.lock.handler"
  properties = {
    "account.lock.handler.lock.on.max.failed.attempts.enable" = "true"
    "account.lock.handler.On.Failure.Max.Attempts"            = "5"
    "account.lock.handler.Time"                               = "15"
  }
}

# Allow self sign-up with email verification.
resource "wso2is_governance_connector" "self_sign_up" {
  name = "self-sign-up"
  properties = {
    "SelfRegistration.Enable"         = "true"
    "SelfRegistration.LockOnCreation" = "true"
  }
}
