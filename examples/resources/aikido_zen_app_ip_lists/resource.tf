resource "aikido_zen_app_ip_lists" "api" {
  app_id   = aikido_zen_app.api.id
  tor_mode = "block"

  known_threat_actors {
    code = "apt1"
    mode = "block"
  }
}
