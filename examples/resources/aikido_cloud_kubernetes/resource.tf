resource "aikido_cloud_kubernetes" "production" {
  name                  = "Production Cluster"
  environment           = "production"
  excluded_namespaces   = ["kube-system", "kube-public"]
  enable_image_scanning = true
}

output "agent_endpoint" {
  value     = aikido_cloud_kubernetes.production.endpoint
  sensitive = true
}

output "agent_token" {
  value     = aikido_cloud_kubernetes.production.agent_token
  sensitive = true
}
