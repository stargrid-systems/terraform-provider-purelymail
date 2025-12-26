## 1.0.0 (December 26, 2025)

FEATURES:

* **New Resource**: `purelymail_user` - Manage user accounts with optional 2FA and nested password reset methods
* **New Resource**: `purelymail_password_reset_method` - Standalone password reset method management
* **New Resource**: `purelymail_domain` - Add and configure email domains with DNS settings
* **New Resource**: `purelymail_routing_rule` - Configure email routing and forwarding rules
* **New Resource**: `purelymail_app_password` - Generate application-specific passwords
* **New Ephemeral Resource**: `purelymail_app_password` - Generate temporary app passwords
* **New Data Source**: `purelymail_ownership_proof` - Retrieve domain ownership verification codes

ENHANCEMENTS:

* User resource automatically handles 2FA workflow ordering (create user → add password reset methods → enable 2FA)
* Password reset methods can be managed as nested attributes on user resource or as standalone resources
* All resources support Terraform import for managing existing Purelymail infrastructure
* Comprehensive test coverage with acceptance tests for all resources
* Detailed documentation with examples for all use cases

DOCUMENTATION:

* Complete provider documentation with authentication guide
* Individual resource documentation with schema details
* Full examples directory with common use cases
* Complete infrastructure example demonstrating all features
* README with quick start guide and development instructions
