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

func TestAccRoutingRuleResource(t *testing.T) {
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
				Config: testAccRoutingRuleResourceConfig(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("domain_name"), knownvalue.StringExact("example.com")),
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("match_user"), knownvalue.StringExact("support")),
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("prefix"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("catchall"), knownvalue.Bool(false)),
				},
			},
			// Update and Read testing
			{
				Config: testAccRoutingRuleResourceConfigUpdated(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("domain_name"), knownvalue.StringExact("example.com")),
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("match_user"), knownvalue.StringExact("sales")),
					statecheck.ExpectKnownValue("purelymail_routing_rule.test", tfjsonpath.New("prefix"), knownvalue.Bool(true)),
				},
			},
			// ImportState testing
			{
				ResourceName:      "purelymail_routing_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs
		},
	})
}

func testAccRoutingRuleResourceConfig(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_routing_rule" "test" {
  domain_name      = "example.com"
  prefix           = false
  match_user       = "support"
  target_addresses = ["team@example.com"]
  catchall         = false
}
`
}

func testAccRoutingRuleResourceConfigUpdated(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

resource "purelymail_routing_rule" "test" {
  domain_name      = "example.com"
  prefix           = true
  match_user       = "sales"
  target_addresses = ["sales-team@example.com", "manager@example.com"]
}
`
}
