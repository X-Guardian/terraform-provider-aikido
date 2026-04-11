provider "aikido" {
  # client_id and client_secret can also be set via
  # AIKIDO_CLIENT_ID and AIKIDO_CLIENT_SECRET environment variables.
  client_id     = var.aikido_client_id
  client_secret = var.aikido_client_secret

  # Optional: "eu" (default), "us", or "me"
  region = "eu"
}
