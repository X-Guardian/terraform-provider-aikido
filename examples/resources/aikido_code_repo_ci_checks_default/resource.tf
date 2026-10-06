# The workspace default is applied to newly activated repositories that have no
# repository-specific configuration. Existing repositories are not changed.
resource "aikido_code_repo_ci_checks_default" "this" {
  # Fail the check on new high or worse issues, and on critical license issues only.
  minimum_severity         = "high"
  minimum_license_severity = "critical"

  fail_on_dependency_scan = true
  fail_on_sast_scan       = true
  fail_on_iac_scan        = true
  fail_on_secrets_scan    = true
  fail_on_malware_scan    = true

  # Run the code quality scan without failing the check on it. The code quality comment
  # severity defaults to "low" when omitted.
  enable_code_quality_scan                       = true
  fail_on_code_quality_scan                      = false
  post_code_quality_inline_comments_min_severity = "high"

  post_inline_comments_min_severity = "critical"
}
