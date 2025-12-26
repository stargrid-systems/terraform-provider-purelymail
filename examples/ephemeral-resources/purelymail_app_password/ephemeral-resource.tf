# Create an ephemeral app password for temporary access
ephemeral "purelymail_app_password" "temp" {
  user_handle = "alice@example.com"
  name        = "Temporary Terraform Access"
}

# Use the ephemeral password in a module or resource
# Note: This is typically used within a module, not the root configuration
