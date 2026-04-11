# Terraform Provider for Aikido Security

The Aikido Terraform provider allows you to manage resources in [Aikido Security](https://www.aikido.dev/) via the [management API](https://apidocs.aikido.dev/).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24

## Supported Resources

- `aikido_team` — Manage teams in your Aikido workspace.

## Supported Data Sources

- `aikido_teams` — List all teams in your Aikido workspace.

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

resource "aikido_team" "platform" {
  name = "Platform Team"
}

data "aikido_teams" "all" {}
```

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
