# Example: Creating a user with 2FA enabled
#
# With nested password_reset_methods, Terraform handles the proper ordering automatically:
# 1. Create the user
# 2. Upsert password reset methods
# 3. Enable 2FA requirement
#
# All in a single apply!

resource "purelymail_user" "alice" {
  user_name              = "alice@example.com"
  password_wo            = "initial-secure-password"
  enable_search_indexing = true

  # Enable 2FA (password reset methods are configured below)
  require_two_factor_authentication = true

  # Password reset methods - at least one is required for 2FA
  password_reset_methods = [
    {
      type            = "email"
      target          = "alice.recovery@example.com"
      description     = "Primary recovery email"
      allow_mfa_reset = true
    },
    {
      type            = "phone"
      target          = "+15551234567"
      description     = "Recovery phone"
      allow_mfa_reset = false
    }
  ]
}

