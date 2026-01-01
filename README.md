# Terraform Provider for Purelymail

A Terraform provider for managing [Purelymail](https://purelymail.com/) email resources.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.24 (for development)
- Purelymail account with API access

## Installation

### Terraform Registry

```hcl
terraform {
  required_providers {
    purelymail = {
      # NOTE: This doesn't work yet -- registry publishing pending!
      source  = "stargrid-systems/purelymail"
      version = "~> 1.0"
    }
  }
}

provider "purelymail" {
  api_token = var.purelymail_api_token
}
```

### Local Development

```sh
go install
```

## Quick Start

```hcl
# Configure the provider
provider "purelymail" {
  api_token = "your-api-token"
}

ephemeral "random_password" "alice_password" {
  length           = 16
  special          = true
  override_special = "_%@"
}

# Create a user with 2FA and password reset methods
resource "purelymail_user" "alice" {
  user_name                         = "alice@example.com"
  password_wo                       = ephemeral.alice_password.result
  enable_search_indexing            = true
  require_two_factor_authentication = true

  password_reset_methods = [
    {
      type            = "email"
      target          = "alice@recovery.example.com"
      description     = "Primary recovery email"
      allow_mfa_reset = true
    }
  ]
}

# Add a domain
resource "purelymail_domain" "example" {
  name = "example.com"
}
```

## Development

### Building

```sh
go build
```

### Testing

Run unit tests:

```sh
go test ./...
```

Run acceptance tests (requires TF_ACC environment variable):

```sh
TF_ACC=1 go test ./internal/provider -v -timeout 120s
```

Run specific test:

```sh
TF_ACC=1 go test ./internal/provider -v -run TestAccUserResourceWith2FA
```

### Generating Documentation

```sh
make generate
```

This will regenerate the API client from the OpenAPI spec and update provider documentation.

## License

MIT - see [LICENSE](LICENSE) for details.
Note that files from the Terraform template are under Mozilla Public License 2.0 (`MPL-2.0`).
