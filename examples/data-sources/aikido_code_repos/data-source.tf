data "aikido_code_repos" "all" {}

# Look up a repo by name
locals {
  my_repo = one([for r in data.aikido_code_repos.all.repos : r if r.name == "my-service"])
}

# Link it to a team
resource "aikido_team_resource_link" "repo" {
  team_id       = aikido_team.platform.id
  resource_type = "code_repository"
  resource_id   = local.my_repo.id
}
