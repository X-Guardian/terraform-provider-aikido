data "aikido_zen_apps" "all" {}

output "zen_apps" {
  value = [for app in data.aikido_zen_apps.all.apps : "${app.id}: ${app.name} (${app.environment}, blocking=${app.blocking})"]
}
