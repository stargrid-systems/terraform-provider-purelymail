# Create a user with password
resource "purelymail_user" "alice" {
  user_name = "alice"
  password  = "initial-password-123"
}

# Create a user with password_wo (remains in state for change tracking)
resource "purelymail_user" "bob" {
  user_name   = "bob"
  password_wo = "secure-password-456"
}

# Create a user with custom settings and password reset methods
resource "purelymail_user" "charlie" {
  user_name              = "charlie"
  password               = "another-password-789"
  enable_search_indexing = true

  # Configure password reset methods
  password_reset_methods = [
    {
      type            = "email"
      target          = "charlie.backup@example.com"
      description     = "Backup email"
      allow_mfa_reset = true
    }
  ]
}
