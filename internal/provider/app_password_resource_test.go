package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api/mock"
)

func TestAccAppPasswordResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := mock.NewServer()
	handler := api.Handler(mockServer)

	// Wrap with auth check middleware
	authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Purelymail-Api-Token") == "" {
			http.Error(w, "Missing API token", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(authHandler)
	defer server.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccAppPasswordResourceConfig(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_app_password.test", tfjsonpath.New("user_handle"), knownvalue.StringExact("alice@example.com")),
					statecheck.ExpectKnownValue("purelymail_app_password.test", tfjsonpath.New("name"), knownvalue.StringExact("test-app")),
				},
			},
			// Delete testing automatically occurs
		},
	})
}

func testAccAppPasswordResourceConfig(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_app_password" "test" {
  user_handle = "alice@example.com"
  name        = "test-app"
}
`
}
