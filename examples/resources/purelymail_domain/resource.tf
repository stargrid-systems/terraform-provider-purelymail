# Basic domain setup
resource "purelymail_domain" "example" {
  name = "example.com"
}

# Domain with password reset enabled
resource "purelymail_domain" "with_reset" {
  name                   = "mail.example.com"
  allow_account_reset    = true
  symbolic_subaddressing = true
}

# Domain with DNS recheck
resource "purelymail_domain" "recheck_dns" {
  name        = "mail.company.com"
  recheck_dns = true
}
