data "aikido_clouds" "all" {}

# Look up a cloud by name
locals {
  production_aws = one([for c in data.aikido_clouds.all.clouds : c if c.provider_name == "aws" && c.environment == "production"])
}

# Link it to a team
resource "aikido_team_resource_link" "cloud" {
  team_id       = aikido_team.platform.id
  resource_type = "cloud"
  resource_id   = local.production_aws.id
}
