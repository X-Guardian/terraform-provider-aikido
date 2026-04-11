resource "aikido_zen_app_countries" "api" {
  app_id = aikido_zen_app.api.id
  mode   = "block"
  list   = ["CN", "RU", "KP"]
}
