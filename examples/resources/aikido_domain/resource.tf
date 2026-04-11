# Frontend domain
resource "aikido_domain" "website" {
  domain = "example.com"
  kind   = "front_end"
}

# REST API with OpenAPI spec
resource "aikido_domain" "api" {
  domain           = "api.example.com"
  kind             = "rest_api"
  openapi_spec_url = "https://api.example.com/openapi.json"
}
