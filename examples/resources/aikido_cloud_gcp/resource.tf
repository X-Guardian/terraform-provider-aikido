resource "aikido_cloud_gcp" "production" {
  name        = "GCP Production"
  environment = "production"
  project_id  = "my-gcp-project"
  access_key  = file("service-account-key.json")
}
