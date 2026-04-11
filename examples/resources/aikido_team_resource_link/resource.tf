# Link a code repository to a team
resource "aikido_team_resource_link" "repo" {
  team_id       = aikido_team.platform.id
  resource_type = "code_repository"
  resource_id   = "410020"
}

# Link a cloud account to a team
resource "aikido_team_resource_link" "cloud" {
  team_id       = aikido_team.platform.id
  resource_type = "cloud"
  resource_id   = "8206"
}
