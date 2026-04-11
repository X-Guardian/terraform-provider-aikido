data "aikido_domains" "all" {}

output "domains" {
  value = [for d in data.aikido_domains.all.domains : "${d.id}: ${d.domain} (${d.kind})"]
}
