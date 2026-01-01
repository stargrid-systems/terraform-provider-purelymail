# Route exact match emails (e.g., support@example.com)
resource "purelymail_routing_rule" "support" {
  domain_name      = "example.com"
  prefix           = false
  match_user       = "support"
  target_addresses = ["team@example.com"]
}

# Route prefix match emails (e.g., sales-*, sales+something@example.com, etc.)
resource "purelymail_routing_rule" "sales_team" {
  domain_name      = "example.com"
  prefix           = true
  match_user       = "sales"
  target_addresses = ["sales-team@example.com", "manager@example.com"]
}

# Catch-all rule for any unmatched emails on the domain
resource "purelymail_routing_rule" "catchall" {
  domain_name      = "example.com"
  prefix           = false
  match_user       = "*"
  target_addresses = ["catch-all@example.com"]
  catchall         = true
}
