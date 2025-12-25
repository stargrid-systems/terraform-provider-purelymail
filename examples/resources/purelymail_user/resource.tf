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

# Create a user with custom settings
resource "purelymail_user" "charlie" {
  user_name                         = "charlie"
  password                          = "another-password-789"
  enable_search_indexing            = true
  enable_password_reset             = true
  require_two_factor_authentication = false
}
