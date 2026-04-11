data "aikido_code_repos" "all" {}

locals {
  api_server = one([for r in data.aikido_code_repos.all.repos : r if r.name == "yulife-api-server"])
}

resource "aikido_code_repo_config" "api_server" {
  code_repo_id             = local.api_server.id
  active                   = true
  sensitivity              = "extreme"
  connectivity             = "connected"
  dev_dep_scanning_enabled = true
  excluded_paths           = ["vendor/", "node_modules/", "test/fixtures/"]
}
