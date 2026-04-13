resource "aikido_zen_app_ip_blocklist" "api" {
  app_id       = aikido_zen_app.api.id
  ip_addresses = ["198.51.100.1", "192.0.2.0/24", "203.0.113.0/24"]
}
