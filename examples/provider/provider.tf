terraform {
  required_providers {
    aikido = {
      source = "X-Guardian/aikido"
    }
  }
}

provider "aikido" {
  client_id     = var.aikido_client_id
  client_secret = var.aikido_client_secret

  # Optional: "eu" (default), "us", or "me"
  region = "eu"
}
