data "aikido_code_repos" "all" {}

locals {
  api_server = one([for r in data.aikido_code_repos.all.repos : r if r.name == "example-api-server"])
  docs_site  = one([for r in data.aikido_code_repos.all.repos : r if r.name == "example-docs-site"])
}

# Aikido's dashboard calls these PR gating or PR Check settings. On GitLab the same checks
# run on merge requests.
resource "aikido_code_repo_ci_checks" "api_server" {
  code_repo_id = local.api_server.id

  # Fail the check on new high or worse issues, and on critical license issues only.
  minimum_severity         = "high"
  minimum_license_severity = "critical"

  fail_on_dependency_scan = true
  fail_on_sast_scan       = true
  fail_on_iac_scan        = true
  fail_on_secrets_scan    = true
  fail_on_malware_scan    = false

  # Run the code quality scan and comment on it, without failing the check on it.
  enable_code_quality_scan                       = true
  fail_on_code_quality_scan                      = false
  post_code_quality_inline_comments_min_severity = "high"

  post_inline_comments_min_severity = "critical"
}

# Run the enabled scans without ever failing the check, reporting issues only.
resource "aikido_code_repo_ci_checks" "docs_site" {
  code_repo_id = local.docs_site.id

  minimum_severity         = "always_pass_check"
  minimum_license_severity = "none"

  fail_on_dependency_scan = false
  fail_on_sast_scan       = false
  fail_on_iac_scan        = false
  fail_on_secrets_scan    = false
  fail_on_malware_scan    = false

  # With the code quality scan off, omit the code quality comment severity: setting it here
  # is rejected, because there are no code quality issues to comment on. The reverse is not
  # required, so it may be omitted with the scan enabled too.
  enable_code_quality_scan  = false
  fail_on_code_quality_scan = false
}
