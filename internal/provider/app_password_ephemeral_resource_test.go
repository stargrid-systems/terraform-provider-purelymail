package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api/mock"
)

func TestAccAppPasswordEphemeralResource(t *testing.T) {
	// Ephemeral resources are only available in Terraform 1.10 and later
	terraformVersionChecks := []tfversion.TerraformVersionCheck{
		tfversion.SkipBelow(tfversion.Version1_10_0),
	}

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
		TerraformVersionChecks:   terraformVersionChecks,
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAppPasswordEphemeralResourceConfig(server.URL),
			},
		},
	})
}

func testAccAppPasswordEphemeralResourceConfig(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

ephemeral "purelymail_app_password" "test" {
  user_handle = "bob@example.com"
  name        = "ephemeral-test"
}
`
}
