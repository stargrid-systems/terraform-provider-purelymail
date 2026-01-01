package provider

import (
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api/mock"
)

func TestAccUserResourceWith2FA(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := mock.NewServer()
	handler := api.Handler(mockServer)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create user with password reset methods and 2FA
			{
				Config: testAccUserResourceWith2FAConfig(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("charlie")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("require_two_factor_authentication"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("password_reset_methods").AtSliceIndex(0).AtMapKey("type"), knownvalue.StringExact("email")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("password_reset_methods").AtSliceIndex(0).AtMapKey("target"), knownvalue.StringExact("charlie@recovery.example.com")),
				},
			},
			// Update: Add another password reset method
			{
				Config: testAccUserResourceWith2FAConfigUpdated(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("require_two_factor_authentication"), knownvalue.Bool(true)),
				},
			},
			// Update: Disable 2FA
			{
				Config: testAccUserResourceWith2FAConfigNo2FA(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("require_two_factor_authentication"), knownvalue.Bool(false)),
				},
			},
			// Delete testing automatically occurs
		},
	})
}

func testAccUserResourceWith2FAConfig(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name                         = "charlie"
  password_wo                       = "secure-password-123"
  require_two_factor_authentication = true

  password_reset_methods = [
    {
      type            = "email"
      target          = "charlie@recovery.example.com"
      description     = "Primary recovery email"
      allow_mfa_reset = true
    }
  ]
}
`
}

func testAccUserResourceWith2FAConfigUpdated(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name                         = "charlie"
  password_wo                       = "secure-password-123"
  require_two_factor_authentication = true

  password_reset_methods = [
    {
      type            = "email"
      target          = "charlie@recovery.example.com"
      description     = "Primary recovery email"
      allow_mfa_reset = true
    },
    {
      type            = "phone"
      target          = "+15551234567"
      description     = "Recovery phone"
      allow_mfa_reset = false
    }
  ]
}
`
}

func testAccUserResourceWith2FAConfigNo2FA(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name                         = "charlie"
  password_wo                       = "secure-password-123"
  require_two_factor_authentication = false

  password_reset_methods = [
    {
      type            = "email"
      target          = "charlie@recovery.example.com"
      description     = "Primary recovery email"
      allow_mfa_reset = true
    }
  ]
}
`
}
