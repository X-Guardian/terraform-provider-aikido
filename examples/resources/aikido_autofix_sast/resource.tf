data "aikido_code_repos" "all" {}

# SAST and IaC AutoFix settings are workspace-wide, so only one of these should exist in a
# real configuration. The variants below are shown together for reference.
resource "aikido_autofix_sast" "this" {
  enabled         = true
  severity_filter = "critical_and_high_only"
  repos_scope     = "all"
}

# To scope AutoFix to specific code repositories, set `repos_scope` to "selected" and list
# the repository IDs.
resource "aikido_autofix_sast" "selected_repos" {
  enabled         = true
  severity_filter = "all"
  repos_scope     = "selected"
  repo_ids        = [for repo in data.aikido_code_repos.all.repos : repo.id]
}

# To turn AutoFix off, only `enabled` is required; the API ignores the other settings.
resource "aikido_autofix_sast" "disabled" {
  enabled = false
}
