resource "wso2is_validation_rules" "password" {
  field = "password"
  rules = [
    { validator = "LengthValidator", properties = { "min.length" = "12", "max.length" = "64" } },
    { validator = "NumeralValidator", properties = { "min.length" = "1" } },
    { validator = "UpperCaseValidator", properties = { "min.length" = "1" } },
    { validator = "LowerCaseValidator", properties = { "min.length" = "1" } },
    { validator = "SpecialCharacterValidator", properties = { "min.length" = "1" } },
    { validator = "RepeatedCharacterValidator", properties = { "max.consecutive.character" = "3" } },
  ]
}

# Usernames: rule validators only (the server rejects a regular expression here).
resource "wso2is_validation_rules" "username" {
  field = "username"
  rules = [
    { validator = "LengthValidator", properties = { "min.length" = "3", "max.length" = "40" } },
    { validator = "AlphanumericValidator", properties = { "enable.validator" = "true", "enable.special.characters" = "true" } },
  ]
}

# Alternatively, one regular expression for passwords:
# resource "wso2is_validation_rules" "password" {
#   field = "password"
#   regex = "^(?=.*[A-Z])(?=.*[0-9]).{12,64}$"
# }
