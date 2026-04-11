resource "aikido_team" "platform" {
  name = "Platform Team"
}

resource "aikido_team_membership" "example" {
  team_id = aikido_team.platform.id
  user_id = "123"
}
