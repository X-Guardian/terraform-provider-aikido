resource "aikido_zen_app" "api" {
  name        = "API Server"
  environment = "production"
  repo_id     = "410013"
}

output "zen_token" {
  value     = aikido_zen_app.api.token
  sensitive = true
}
