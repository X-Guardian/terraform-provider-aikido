resource "aikido_cloud_azure" "production" {
  name            = "Azure Production"
  environment     = "production"
  application_id  = "00000000-0000-0000-0000-000000000000"
  directory_id    = "00000000-0000-0000-0000-000000000000"
  subscription_id = "00000000-0000-0000-0000-000000000000"
  key_value       = var.azure_client_secret
}
