# Terraform Provider for Aikido Security

The Aikido Terraform provider allows you to manage resources in [Aikido Security](https://www.aikido.dev/) via the [management API](https://apidocs.aikido.dev/).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.26

## Supported Resources

### Teams
- `aikido_team` — Manage teams.
- `aikido_team_membership` — Manage user membership in a team.
- `aikido_team_resource_link` — Link code repos, containers, clouds, or domains to a team.

### Code Repositories
- `aikido_code_repo_config` — Manage scanning configuration (sensitivity, connectivity, excluded paths) of an existing code repository.

### Containers
- `aikido_container_config` — Manage scanning configuration (sensitivity, connectivity, tag filter) of an existing container repository.

### Cloud Environments
- `aikido_cloud_aws` — Connect an AWS cloud environment.
- `aikido_cloud_azure` — Connect an Azure cloud environment.
- `aikido_cloud_gcp` — Connect a GCP cloud environment.
- `aikido_cloud_kubernetes` — Connect a Kubernetes cluster.

### Domains
- `aikido_domain` — Manage domains for surface monitoring and DAST scanning.

### Webhooks
- `aikido_webhook` — Manage webhooks for event notifications.

### Custom SAST Rules
- `aikido_custom_sast_rule` — Manage custom semgrep SAST rules.

### Zen (Runtime Firewall)
- `aikido_zen_app` — Manage Zen runtime firewall apps.
- `aikido_zen_app_blocking` — Enable/disable blocking mode.
- `aikido_zen_app_countries` — Manage country-based IP blocking.
- `aikido_zen_app_ip_blocklist` — Manage custom IP blocklist.
- `aikido_zen_app_bot_lists` — Manage bot list subscriptions.
- `aikido_zen_app_ip_lists` — Manage threat actor IP lists and Tor traffic.

## Supported Data Sources

- `aikido_teams` — List all teams.
- `aikido_users` — List users with optional team and inactive filters.
- `aikido_code_repos` — List code repositories with optional name, branch, and inactive filters.
- `aikido_clouds` — List all connected cloud environments.
- `aikido_containers` — List container repositories with optional name, tag, team, and status filters.
- `aikido_domains` — List all domains.
- `aikido_zen_apps` — List all Zen runtime firewall apps.

## Authentication

The provider authenticates using OAuth2 client credentials. You can obtain a client ID and secret from the Aikido dashboard.

```hcl
provider "aikido" {
  client_id     = var.aikido_client_id
  client_secret = var.aikido_client_secret
  region        = "eu" # "eu" (default), "us", or "me"
}
```

Credentials can also be provided via environment variables:

- `AIKIDO_CLIENT_ID`
- `AIKIDO_CLIENT_SECRET`
- `AIKIDO_REGION` (optional, defaults to `eu`)
- `AIKIDO_API_URL` (optional, overrides region)

## Usage

```hcl
terraform {
  required_providers {
    aikido = {
      source = "X-Guardian/aikido"
    }
  }
}

provider "aikido" {}

data "aikido_users" "all" {}

locals {
  simon = one([for user in data.aikido_users.all.users : user if user.email == "simon@example.com"])
}

resource "aikido_team" "platform" {
  name = "Platform Team"
}

resource "aikido_team_membership" "simon" {
  team_id = aikido_team.platform.id
  user_id = local.simon.id
}

data "aikido_code_repos" "all" {}

resource "aikido_code_repo_config" "api" {
  code_repo_id = one([for repo in data.aikido_code_repos.all.repos : repo if repo.name == "my-api"]).id
  sensitivity  = "extreme"
  active       = true
}
```

## Rate Limiting

The Aikido API has a rate limit of 20 requests per minute per workspace. The provider includes a built-in rate limiter (18 req/min) to stay within this limit, plus automatic retry with `Retry-After` header support for 429 responses.

## Building the Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider:

```shell
go install
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

### Running Tests

Unit tests:

```shell
make test
```

Acceptance tests (require valid Aikido credentials):

```shell
export AIKIDO_CLIENT_ID="your-client-id"
export AIKIDO_CLIENT_SECRET="your-client-secret"
make testacc
```

**Note:** Acceptance tests create real resources in your Aikido workspace.
