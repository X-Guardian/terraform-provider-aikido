# List all active users
data "aikido_users" "all" {}

# List users in a specific team
data "aikido_users" "platform" {
  team_id = aikido_team.platform.id
}

# Include inactive users
data "aikido_users" "everyone" {
  include_inactive = true
}
