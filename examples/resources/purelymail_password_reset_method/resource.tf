resource "purelymail_password_reset_method" "example" {
  user_name       = "alice@example.com"
  type            = "email"
  target          = "alice@recovery.example.com"
  description     = "Recovery email for Alice"
  allow_mfa_reset = true
}
