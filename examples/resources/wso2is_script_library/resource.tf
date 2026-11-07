resource "wso2is_script_library" "risk" {
  name        = "risk.js"
  description = "Risk helpers for adaptive authentication"
  content     = <<-EOT
    function isRisky(score) {
      return score > 70;
    }
    module.exports.isRisky = isRisky;
  EOT
}
