data "aikido_containers" "all" {}

locals {
  my_container = one([for c in data.aikido_containers.all.containers : c if c.name == "my-app"])
}

resource "aikido_container_config" "my_app" {
  container_repo_id = local.my_container.id
  active            = true
  sensitivity       = "sensitive"
  internet_exposed  = "connected"
  tag_filter        = "v*"
}
