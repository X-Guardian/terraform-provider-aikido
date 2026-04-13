resource "aikido_zen_app_blocking" "api" {
  app_id                     = aikido_zen_app.api.id
  block                      = true
  disable_minimum_wait_check = true
}
