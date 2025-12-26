# Test Coverage Summary

## Overview

The Purelymail Terraform Provider has comprehensive test coverage with acceptance tests for all resources, data sources, and ephemeral resources.

## Test Execution

```sh
# Run all tests
TF_ACC=1 go test ./internal/provider -v -timeout 120s

# Run specific test
TF_ACC=1 go test ./internal/provider -v -run TestAccUserResourceWith2FA

# Run without acceptance tests (quick)
go test ./... -v
```

## Test Results

All tests passing ✅

```
=== RUN   TestAccAppPasswordEphemeralResource
--- PASS: TestAccAppPasswordEphemeralResource (1.46s)
=== RUN   TestAccAppPasswordResource
--- PASS: TestAccAppPasswordResource (1.44s)
=== RUN   TestAccDomainResource
--- PASS: TestAccDomainResource (1.93s)
=== RUN   TestAccOwnershipProofDataSource
--- PASS: TestAccOwnershipProofDataSource (1.66s)
=== RUN   TestAccPasswordResetMethodResource
--- PASS: TestAccPasswordResetMethodResource (1.63s)
=== RUN   TestAccRoutingRuleResource
--- PASS: TestAccRoutingRuleResource (1.65s)
=== RUN   TestAccUserResource
--- PASS: TestAccUserResource (1.58s)
=== RUN   TestAccUserResourcePasswordWo
--- PASS: TestAccUserResourcePasswordWo (1.64s)
=== RUN   TestAccUserResourceWith2FA
--- PASS: TestAccUserResourceWith2FA (1.70s)
PASS
ok      github.com/stargrid-systems/terraform-provider-purelymail/internal/provider     14.691s
```

## Coverage by Resource

### Resources

| Resource | Test File | Coverage |
|----------|-----------|----------|
| purelymail_user | user_resource_test.go | ✅ Create, Read, Update, Delete, Import |
| purelymail_user (2FA) | user_resource_with_2fa_test.go | ✅ 2FA workflow, password reset methods |
| purelymail_user (password_wo) | user_resource_test.go | ✅ Password tracking variant |
| purelymail_password_reset_method | password_reset_method_resource_test.go | ✅ Create, Read, Update, Delete, Import |
| purelymail_domain | domain_resource_test.go | ✅ Create, Read, Update, Delete, Import |
| purelymail_routing_rule | routing_rule_resource_test.go | ✅ Create, Read, Update, Delete |
| purelymail_app_password | app_password_resource_test.go | ✅ Create, Read, Delete |

### Ephemeral Resources

| Resource | Test File | Coverage |
|----------|-----------|----------|
| purelymail_app_password | app_password_ephemeral_resource_test.go | ✅ Open, Renew, Close |

### Data Sources

| Data Source | Test File | Coverage |
|-------------|-----------|----------|
| purelymail_ownership_proof | ownership_proof_data_source_test.go | ✅ Read |

## Test Scenarios Covered

### User Resource Tests

1. **Basic User Lifecycle** (`TestAccUserResource`)
   - Create user with search indexing enabled
   - Read user state
   - Update search indexing setting
   - Delete user
   - Verify ID and attributes

2. **Password Tracking** (`TestAccUserResourcePasswordWo`)
   - Create user with `password_wo`
   - Update password using `password_wo`
   - Verify password changes are tracked in state

3. **2FA Workflow** (`TestAccUserResourceWith2FA`)
   - Create user with 2FA enabled and password reset methods
   - Verify automatic ordering (user → methods → 2FA)
   - Add additional password reset method
   - Disable 2FA
   - Verify state updates

### Password Reset Method Tests

1. **Standalone Resource** (`TestAccPasswordResetMethodResource`)
   - Create password reset method
   - Read method state
   - Update description and allow_mfa_reset
   - Import using `username:target` format
   - Delete method

### Domain Tests

1. **Domain Management** (`TestAccDomainResource`)
   - Add domain
   - Read DNS summary (computed attribute)
   - Update domain settings
   - Import existing domain
   - Delete domain

### Routing Rule Tests

1. **Email Routing** (`TestAccRoutingRuleResource`)
   - Create routing rule
   - Read rule state
   - Update target addresses
   - Delete rule
   - Verify match patterns

### App Password Tests

1. **Resource Variant** (`TestAccAppPasswordResource`)
   - Generate app password
   - Read password value
   - Delete app password
   - Verify password in state

2. **Ephemeral Variant** (`TestAccAppPasswordEphemeralResource`)
   - Open ephemeral password
   - Renew if needed
   - Close and cleanup
   - Verify lifecycle

### Data Source Tests

1. **Ownership Proof** (`TestAccOwnershipProofDataSource`)
   - Read ownership code for domain
   - Verify record name format
   - Check ownership code value

## Mock Server Implementation

All tests use mock HTTP servers that:
- Simulate Purelymail API responses
- Track state changes across requests
- Validate request formats
- Support all CRUD operations
- Handle authentication

Example mock server features:
```go
// State tracking
userState := map[string]interface{}{
    "enableSearchIndexing": true,
    "requireTwoFactorAuthentication": false,
}

// Request routing
switch r.RequestURI {
case "/api/v0/createUser":
    // Handle user creation
case "/api/v0/modifyUser":
    // Update tracked state
case "/api/v0/getUser":
    // Return current state
}
```

## Test Quality Metrics

- **Code Coverage**: All resource CRUD operations tested
- **State Validation**: Every test verifies state attributes
- **Import Testing**: Resources with import support have import tests
- **Error Handling**: API errors properly surfaced to user
- **Concurrent Safety**: Mock servers use mutex for state protection
- **Isolation**: Each test uses independent mock server

## Adding New Tests

To add a new test:

1. Create test file: `*_test.go`
2. Implement mock server for API endpoints
3. Write test cases using `resource.Test`
4. Add state checks with `statecheck.ExpectKnownValue`
5. Run with `TF_ACC=1 go test -v -run YourTest`

Example structure:
```go
func TestAccYourResource(t *testing.T) {
    // Setup mock server
    server := httptest.NewServer(http.HandlerFunc(handler))
    defer server.Close()

    resource.Test(t, resource.TestCase{
        PreCheck:                 func() { testAccPreCheck(t) },
        ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
        Steps: []resource.TestStep{
            {
                Config: testConfig(server.URL),
                ConfigStateChecks: []statecheck.StateCheck{
                    statecheck.ExpectKnownValue(...),
                },
            },
        },
    })
}
```

## Continuous Integration

For CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run Tests
  run: |
    TF_ACC=1 go test ./internal/provider -v -timeout 120s
  env:
    TF_ACC: 1
```

## Known Limitations

- Tests use mock servers (not real Purelymail API)
- Network operations not tested
- Rate limiting not tested
- Real DNS propagation not tested

For production validation, test against actual Purelymail API in a sandbox environment.
