# Create an ephemeral app password for temporary access
ephemeral "purelymail_app_password" "temp" {
  user_handle = "alice@example.com"
  name        = "Temporary Terraform Access"
}
