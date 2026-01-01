package provider

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api/mock"
)

func TestAccPasswordResetMethodResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := mock.NewServer()
	handler := api.Handler(mockServer)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPasswordResetMethodResourceConfig(ts.URL, "alice", "email", "alice@recovery.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "user_name", "alice"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "type", "email"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "target", "alice@recovery.example.com"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "allow_mfa_reset", "false"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "purelymail_password_reset_method.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "alice:alice@recovery.example.com",
			},
			// Update and Read testing
			{
				Config: testAccPasswordResetMethodResourceConfigWithDescription(ts.URL, "alice", "email", "alice@recovery.example.com", "Updated recovery email", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "description", "Updated recovery email"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "allow_mfa_reset", "true"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccPasswordResetMethodResourceConfig(endpoint string, userName string, methodType string, target string) string {
	return fmt.Sprintf(`
provider "purelymail" {
  endpoint  = %[1]q
  api_token = "test-token"
}

resource "purelymail_password_reset_method" "test" {
  user_name = %[2]q
  type      = %[3]q
  target    = %[4]q
}
`, endpoint, userName, methodType, target)
}

func testAccPasswordResetMethodResourceConfigWithDescription(endpoint string, userName string, methodType string, target string, description string, allowMfaReset bool) string {
	return fmt.Sprintf(`
provider "purelymail" {
  endpoint  = %[1]q
  api_token = "test-token"
}

resource "purelymail_password_reset_method" "test" {
  user_name       = %[2]q
  type            = %[3]q
  target          = %[4]q
  description     = %[5]q
  allow_mfa_reset = %[6]t
}
`, endpoint, userName, methodType, target, description, allowMfaReset)
}
