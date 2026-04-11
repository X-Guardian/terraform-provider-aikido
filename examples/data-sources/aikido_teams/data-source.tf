data "aikido_teams" "all" {}

output "team_names" {
  value = [for team in data.aikido_teams.all.teams : team.name]
}
