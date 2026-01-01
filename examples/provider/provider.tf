# Configure the provider
provider "purelymail" {
  api_token = var.purelymail_api_token
}

# Create a domain
resource "purelymail_domain" "example" {
  name = "example.com"
}

# Create a user with 2FA enabled
resource "purelymail_user" "alice" {
  user_name                         = "alice@example.com"
  password_wo                       = var.alice_password
  enable_search_indexing            = true
  require_two_factor_authentication = true

  # Password reset methods (required for 2FA)
  password_reset_methods = [
    {
      type            = "email"
      target          = "alice@recovery.example.com"
      description     = "Primary recovery email"
      allow_mfa_reset = true
    }
  ]
}

# Create a routing rule - catch-all that routes all mail to alice
resource "purelymail_routing_rule" "catch_all" {
  domain_name      = purelymail_domain.example.name
  prefix           = false
  match_user       = "*"
  target_addresses = ["alice@example.com"]
  catchall         = true
}

# Generate an app password for the user
resource "purelymail_app_password" "mobile" {
  user_handle = "alice@example.com"
  name        = "Mobile email client"
}
