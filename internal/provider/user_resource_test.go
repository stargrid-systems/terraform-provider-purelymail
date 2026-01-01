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

func TestAccUserResource(t *testing.T) {
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
				Config: testAccUserResourceConfig(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("alice")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("enable_search_indexing"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("id"), knownvalue.StringExact("alice")),
				},
			},
			// Update and Read testing
			{
				Config: testAccUserResourceConfigUpdated(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("alice")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("enable_search_indexing"), knownvalue.Bool(false)),
				},
			},
			// Delete testing automatically occurs
		},
	})
}

func testAccUserResourceConfig(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name              = "alice"
  enable_search_indexing = true
}
`
}

func testAccUserResourceConfigUpdated(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name              = "alice"
  enable_search_indexing = false
}
`
}

func TestAccUserResourcePasswordWo(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := mock.NewServer()
	handler := api.Handler(mockServer)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with password_wo
			{
				Config: testAccUserResourceConfigPasswordWo(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("bob")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("id"), knownvalue.StringExact("bob")),
				},
			},
			// Update with password_wo
			{
				Config: testAccUserResourceConfigPasswordWoUpdated(ts.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("bob")),
				},
			},
			// Delete testing automatically occurs
		},
	})
}

func testAccUserResourceConfigPasswordWo(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name   = "bob"
  password_wo = "initial-password-123"
}
`
}

func testAccUserResourceConfigPasswordWoUpdated(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_user" "test" {
  user_name   = "bob"
  password_wo = "updated-password-456"
}
`
}
