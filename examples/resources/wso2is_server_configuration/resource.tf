resource "wso2is_server_configuration" "this" {
  idle_session_timeout           = 30
  remember_me_period             = 10080
  enable_maximum_session_timeout = true
  maximum_session_timeout        = 1440
}
