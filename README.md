# Terraform Provider for Purelymail

A Terraform provider for managing [Purelymail](https://purelymail.com/) email resources.

## Features

- **User Management**: Create and manage Purelymail user accounts with optional 2FA
- **Password Reset Methods**: Configure email/phone recovery methods (nested or standalone)
- **Routing Rules**: Manage email routing and forwarding rules
- **Domain Management**: Add and configure domains with DNS settings
- **App Passwords**: Generate application-specific passwords
- **Ownership Verification**: Data source for domain ownership codes

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

# Create a user with 2FA and password reset methods
resource "purelymail_user" "alice" {
  user_name                         = "alice"
  password_wo                       = "secure-password-123"
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
  domain_name = "example.com"
}

# Create a routing rule
resource "purelymail_routing_rule" "forward" {
  domain_name = purelymail_domain.example.domain_name
  match_user  = "info"
  hostname    = purelymail_domain.example.domain_name
  target_addresses = ["alice@otherdomain.com"]
}
```

## Resources

- **purelymail_user**: Manage user accounts with optional 2FA and nested password reset methods
- **purelymail_password_reset_method**: Standalone resource for managing password reset methods
- **purelymail_routing_rule**: Configure email routing and forwarding
- **purelymail_domain**: Add and manage domains
- **purelymail_app_password**: Generate app-specific passwords (also available as ephemeral resource)

## Data Sources

- **purelymail_ownership_proof**: Get domain ownership verification codes

## Documentation

Comprehensive documentation is available in the `docs/` directory:

- [Provider Configuration](docs/index.md)
- [User Resource](docs/resources/user.md)
- [Password Reset Method Resource](docs/resources/password_reset_method.md)
- [Domain Resource](docs/resources/domain.md)
- [Routing Rule Resource](docs/resources/routing_rule.md)
- [App Password Resource](docs/resources/app_password.md)

Examples are available in the `examples/` directory.

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
go generate ./...
```

This will regenerate the API client from the OpenAPI spec and update provider documentation.

## Architecture

### API Client Generation

The API client (`internal/api/purelymail.gen.go`) is generated from the OpenAPI specification using [oapi-codegen](https://github.com/deepmap/oapi-codegen):

```sh
cd internal/api
oapi-codegen -config config.yaml openapi.yaml > purelymail.gen.go
```

### Resource Organization

- `internal/provider/`: Terraform provider implementation
  - `*_resource.go`: Resource implementations
  - `*_data_source.go`: Data source implementations
  - `*_ephemeral_resource.go`: Ephemeral resource implementations
  - `*_test.go`: Acceptance tests
- `internal/api/`: Generated API client
- `examples/`: Example Terraform configurations
- `docs/`: Generated documentation

## Key Features

### Automatic 2FA Ordering

The user resource automatically handles the proper ordering for enabling 2FA:

1. Create user
2. Add password reset methods
3. Enable 2FA requirement

All in a single `terraform apply`!

```hcl
resource "purelymail_user" "user" {
  user_name                         = "alice"
  password_wo                       = "password"
  require_two_factor_authentication = true

  password_reset_methods = [
    {
      type   = "email"
      target = "recovery@example.com"
    }
  ]
}
```

### Flexible Password Reset Management

Password reset methods can be managed either:

1. **Nested (Recommended)**: As part of the user resource
2. **Standalone**: As separate `purelymail_password_reset_method` resources

### Ephemeral App Passwords

Generate temporary app passwords using ephemeral resources:

```hcl
ephemeral "purelymail_app_password" "temp" {
  user_name   = "alice"
  description = "Temporary access"
}
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `go generate ./...` to update generated files
6. Submit a pull request

## License

Mozilla Public License 2.0 - see [LICENSE](LICENSE) for details.

## Support

- [Purelymail Documentation](https://purelymail.com/docs)
- [Terraform Provider Documentation](docs/)
- [Issue Tracker](https://github.com/stargrid-systems/terraform-provider-purelymail/issues)

