# Troubleshooting Guide

Common issues and solutions for the Purelymail Terraform Provider.

## Authentication Issues

### Error: "Missing API token"

**Symptom**: 
```
Error: Missing API token
```

**Solution**:
Ensure you've configured the API token in one of these ways:

1. Provider configuration:
```hcl
provider "purelymail" {
  api_token = "your-token"
}
```

2. Environment variable:
```sh
export PURELYMAIL_API_TOKEN="your-token"
```

3. Terraform variable:
```hcl
variable "purelymail_api_token" {
  type = string
  sensitive = true
}

provider "purelymail" {
  api_token = var.purelymail_api_token
}
```

### Error: "Unauthorized" (401)

**Symptom**:
```
Error: API returned status 401
```

**Solution**:
- Verify your API token is correct
- Check that the token hasn't expired
- Ensure the token has the necessary permissions

## 2FA and Password Reset Issues

### Error: "Failed to enable 2FA"

**Symptom**:
```
Error: Failed to enable 2FA (status: 400)
```

**Solution**:
You must configure at least one password reset method before enabling 2FA. Use the nested approach:

```hcl
resource "purelymail_user" "user" {
  user_name                         = "alice"
  password_wo                       = "password"
  require_two_factor_authentication = true

  # Required: At least one password reset method
  password_reset_methods = [
    {
      type   = "email"
      target = "alice@recovery.com"
    }
  ]
}
```

### Password Reset Methods Not Appearing

**Symptom**:
Password reset methods configured but not showing in state.

**Solution**:
- Ensure you're using the latest version of the provider
- Run `terraform refresh` to update state from API
- Check that the API response includes the methods (enable debug logging)

## Import Issues

### Error: "Cannot import non-existent remote object"

**Symptom**:
```
Error: Cannot import non-existent remote object
```

**Solution**:
- Verify the resource exists in Purelymail
- Check the import ID format is correct:
  - User: `terraform import purelymail_user.alice alice`
  - Domain: `terraform import purelymail_domain.example example.com`
  - Password Reset: `terraform import purelymail_password_reset_method.recovery "alice:alice@recovery.com"`

### Import ID Format

Each resource has a specific import format:

| Resource | Format | Example |
|----------|--------|---------|
| user | `username` | `alice` |
| domain | `domain_name` | `example.com` |
| password_reset_method | `username:target` | `alice:alice@recovery.com` |
| routing_rule | `rule_id` | Get from API or state |
| app_password | Not supported | Recreate instead |

## State Issues

### Sensitive Data in State

**Symptom**:
Passwords or tokens visible in state file.

**Solution**:
- Use `password_wo` instead of `password` to keep passwords in state
- Store state remotely with encryption (S3, Terraform Cloud, etc.)
- Mark outputs as sensitive:
```hcl
output "app_password" {
  value     = purelymail_app_password.mobile.app_password
  sensitive = true
}
```

### State Drift

**Symptom**:
```
Note: Objects have changed outside of Terraform
```

**Solution**:
1. Review changes: `terraform plan`
2. Accept external changes: `terraform apply -refresh-only`
3. Override with Terraform: `terraform apply`

## DNS Configuration Issues

### Domain Not Verifying

**Symptom**:
Domain ownership verification failing.

**Solution**:
1. Get the ownership code:
```hcl
data "purelymail_ownership_proof" "domain" {
  domain_name = "example.com"
}

output "ownership_code" {
  value = data.purelymail_ownership_proof.domain.ownership_code
}
```

2. Add TXT record at the specified location
3. Wait for DNS propagation (can take up to 48 hours)
4. Verify with: `dig TXT _purelymail.example.com`

### Missing DNS Records

**Symptom**:
Email not working after domain setup.

**Solution**:
1. Get DNS summary:
```hcl
output "dns_summary" {
  value = purelymail_domain.example.dns_summary
}
```

2. Configure all required records:
   - MX records for mail delivery
   - SPF record for sender verification
   - DKIM record for authentication
   - DMARC record for policy

## Performance Issues

### Slow Apply Operations

**Symptom**:
`terraform apply` taking a long time.

**Solution**:
- API operations are sequential by design
- Creating many users/rules will take time
- Consider batching operations
- Use `terraform apply -parallelism=1` to avoid rate limiting

### Rate Limiting

**Symptom**:
```
Error: Too many requests (status: 429)
```

**Solution**:
- Reduce parallelism: `terraform apply -parallelism=1`
- Add delays between operations
- Contact Purelymail support for rate limit increase

## Debugging

### Enable Debug Logging

```sh
export TF_LOG=DEBUG
export TF_LOG_PATH=./terraform-debug.log
terraform apply
```

### Check Provider Version

```sh
terraform version
```

### Validate Configuration

```sh
terraform validate
terraform fmt -check
```

### Test with Mock Server

The provider includes acceptance tests with mock servers:

```sh
cd internal/provider
TF_ACC=1 go test -v -run TestAccUserResourceWith2FA
```

## Common Patterns

### Creating Multiple Users

Use `for_each` for multiple similar resources:

```hcl
variable "users" {
  type = map(object({
    password = string
    enable_2fa = bool
  }))
}

resource "purelymail_user" "users" {
  for_each = var.users

  user_name                         = each.key
  password_wo                       = each.value.password
  require_two_factor_authentication = each.value.enable_2fa

  dynamic "password_reset_methods" {
    for_each = each.value.enable_2fa ? [1] : []
    content {
      type   = "email"
      target = "${each.key}@recovery.com"
    }
  }
}
```

### Managing Existing Infrastructure

1. Import existing resources:
```sh
terraform import purelymail_user.alice alice
```

2. Generate configuration:
```sh
terraform show
```

3. Copy to `.tf` files and adjust

### Rolling Updates

For zero-downtime user updates:

1. Create new user with different name
2. Update routing rules to point to new user
3. Delete old user

## Getting Help

- **Documentation**: Check [docs/](../docs/) directory
- **Examples**: See [examples/](../examples/) directory
- **Issues**: [GitHub Issues](https://github.com/stargrid-systems/terraform-provider-purelymail/issues)
- **Purelymail**: [Purelymail Support](https://purelymail.com/docs)

## Reporting Bugs

When reporting issues, include:

1. Provider version: `terraform version`
2. Terraform version
3. Redacted configuration
4. Full error message
5. Debug logs (with sensitive data removed)

```sh
# Generate debug logs
export TF_LOG=DEBUG
export TF_LOG_PATH=./debug.log
terraform apply
# Redact sensitive information before sharing debug.log
```
