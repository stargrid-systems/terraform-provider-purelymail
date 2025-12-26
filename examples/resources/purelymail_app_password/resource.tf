# Create an app password for a mobile email client
resource "purelymail_app_password" "mobile" {
  user_handle = "alice@example.com"
  name        = "Mobile Email App"
}

# Create an app password without a name
resource "purelymail_app_password" "desktop" {
  user_handle = "bob@example.com"
}

# Output the password (will be marked as sensitive)
output "mobile_app_password" {
  value     = purelymail_app_password.mobile.app_password
  sensitive = true
}
