resource "aikido_zen_app_bot_lists" "api" {
  app_id = aikido_zen_app.api.id

  bots {
    code = "scrapers"
    mode = "block"
  }

  bots {
    code = "ai_scrapers"
    mode = "monitor"
  }
}
