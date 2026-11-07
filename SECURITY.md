# Security policy

Please do not report security problems in public issues. Use GitHub's
[private vulnerability reporting](https://github.com/hayone1/terraform-provider-wso2is/security/advisories/new)
instead, with the provider version and steps to reproduce.

Vulnerabilities in WSO2 Identity Server itself go to WSO2: see
<https://security.docs.wso2.com/en/latest/security-reporting/vulnerability-reporting-guidelines/>.

## Notes for users

* The provider keeps the secrets you configure (passwords, client secrets) in the Terraform state,
  like any provider. Protect the state, or use the write-only `*_wo` arguments (Terraform 1.11+)
  where a resource offers them.
* `TF_LOG=TRACE` logs API requests and responses with known secret fields redacted. Review logs
  before sharing them.
* `insecure_skip_verify` disables TLS verification; use `ca_certificate` instead outside labs.
