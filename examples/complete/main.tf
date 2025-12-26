terraform {
  required_providers {
    purelymail = {
      source  = "stargrid-systems/purelymail"
      version = "~> 1.0"
    }
  }
}

variable "purelymail_api_token" {
  type        = string
  sensitive   = true
  description = "Purelymail API token"
}

variable "admin_password" {
  type        = string
  sensitive   = true
  description = "Password for admin user"
}

variable "sales_password" {
  type        = string
  sensitive   = true
  description = "Password for sales user"
}

provider "purelymail" {
  api_token = var.purelymail_api_token
}

# ============================================================================
# Domain Configuration
# ============================================================================

resource "purelymail_domain" "company" {
  domain_name = "company.example.com"
}

# Get ownership verification code for DNS setup
data "purelymail_ownership_proof" "company" {
  domain_name = purelymail_domain.company.domain_name
}

output "domain_ownership_code" {
  value       = data.purelymail_ownership_proof.company.ownership_code
  description = "Add this as a TXT record at ${data.purelymail_ownership_proof.company.record_name}"
  sensitive   = true
}

output "dns_summary" {
  value       = purelymail_domain.company.dns_summary
  description = "Required DNS records for email delivery"
}

# ============================================================================
# Users
# ============================================================================

# Admin user with 2FA and multiple recovery methods
resource "purelymail_user" "admin" {
  user_name                         = "admin"
  password_wo                       = var.admin_password
  enable_search_indexing            = true
  require_two_factor_authentication = true

  password_reset_methods = [
    {
      type            = "email"
      target          = "admin.personal@gmail.com"
      description     = "Personal Gmail account"
      allow_mfa_reset = true
    },
    {
      type            = "phone"
      target          = "+15551234567"
      description     = "Mobile phone"
      allow_mfa_reset = true
    }
  ]
}

# Sales user without 2FA but with recovery email
resource "purelymail_user" "sales" {
  user_name              = "sales"
  password_wo            = var.sales_password
  enable_search_indexing = false

  password_reset_methods = [
    {
      type        = "email"
      target      = "sales.backup@example.com"
      description = "Backup email"
    }
  ]
}

# Support user (simple configuration)
resource "purelymail_user" "support" {
  user_name   = "support"
  password_wo = "secure-support-password"
}

# ============================================================================
# Routing Rules
# ============================================================================

# Catch-all rule - forward everything not matching other rules to admin
resource "purelymail_routing_rule" "catch_all" {
  domain_name      = purelymail_domain.company.domain_name
  match_user       = "*"
  hostname         = purelymail_domain.company.domain_name
  target_addresses = ["admin@${purelymail_domain.company.domain_name}"]
}

# Forward info@ to support
resource "purelymail_routing_rule" "info" {
  domain_name      = purelymail_domain.company.domain_name
  match_user       = "info"
  hostname         = purelymail_domain.company.domain_name
  target_addresses = ["support@${purelymail_domain.company.domain_name}"]
}

# Forward contact@ to both sales and support
resource "purelymail_routing_rule" "contact" {
  domain_name = purelymail_domain.company.domain_name
  match_user  = "contact"
  hostname    = purelymail_domain.company.domain_name
  target_addresses = [
    "sales@${purelymail_domain.company.domain_name}",
    "support@${purelymail_domain.company.domain_name}"
  ]
}

# External forwarding rule
resource "purelymail_routing_rule" "billing_external" {
  domain_name      = purelymail_domain.company.domain_name
  match_user       = "billing"
  hostname         = purelymail_domain.company.domain_name
  target_addresses = ["accounting@external-firm.com"]
}

# ============================================================================
# App Passwords
# ============================================================================

# App password for admin's mobile device
resource "purelymail_app_password" "admin_mobile" {
  user_name   = purelymail_user.admin.user_name
  description = "iPhone Mail App"
}

# App password for automated system
resource "purelymail_app_password" "monitoring_system" {
  user_name   = purelymail_user.support.user_name
  description = "Server monitoring alerts"
}

# ============================================================================
# Outputs
# ============================================================================

output "admin_app_password" {
  value       = purelymail_app_password.admin_mobile.app_password
  sensitive   = true
  description = "App password for admin's mobile device"
}

output "monitoring_app_password" {
  value       = purelymail_app_password.monitoring_system.app_password
  sensitive   = true
  description = "App password for monitoring system"
}

output "email_addresses" {
  value = {
    admin   = "${purelymail_user.admin.user_name}@${purelymail_domain.company.domain_name}"
    sales   = "${purelymail_user.sales.user_name}@${purelymail_domain.company.domain_name}"
    support = "${purelymail_user.support.user_name}@${purelymail_domain.company.domain_name}"
  }
  description = "Configured email addresses"
}
