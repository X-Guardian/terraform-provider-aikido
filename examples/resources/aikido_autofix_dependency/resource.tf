# Dependency AutoFix settings are workspace-wide, so only one instance should exist.
resource "aikido_autofix_dependency" "this" {
  enabled                      = true
  severity_filter              = "critical_and_high_only"
  repos_scope                  = "all"
  use_aikido_library_for_major = true
}

# To scope AutoFix to specific code repositories, set `repos_scope` to "selected" and
# list the repository IDs:
#
#   data "aikido_code_repos" "all" {}
#
#   resource "aikido_autofix_dependency" "this" {
#     enabled         = true
#     severity_filter = "upgrade_all_packages"
#     repos_scope     = "selected"
#     repo_ids        = [for repo in data.aikido_code_repos.all.repos : repo.id]
#
#     use_aikido_library_for_major = false
#   }
#
# To turn AutoFix off, only `enabled` is required; the API ignores the other settings:
#
#   resource "aikido_autofix_dependency" "this" {
#     enabled = false
#   }
