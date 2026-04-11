data "aikido_containers" "all" {}

# Filter by name
data "aikido_containers" "nginx" {
  filter_name = "nginx"
}

# Include inactive containers
data "aikido_containers" "everything" {
  filter_status = "all"
}
