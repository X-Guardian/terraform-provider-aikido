resource "aikido_cloud_aws" "production" {
  name        = "AWS Production"
  environment = "production"
  role_arn    = "arn:aws:iam::000000000000:role/aikido-security-readonly-AikidoSecurityRole"
}
