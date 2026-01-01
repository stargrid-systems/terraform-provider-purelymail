data "purelymail_ownership_proof" "this" {}

# Create a DNS TXT record with the verification code.
resource "my_dns_provider_record" "purelymail_verification" {
  type  = "TXT"
  key   = "@"
  value = data.purelymail_ownership_proof.this.code
}
