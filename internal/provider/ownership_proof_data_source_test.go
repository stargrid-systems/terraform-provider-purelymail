// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccOwnershipProofDataSource(t *testing.T) {
	// Create a mock API server that responds with a valid ownership code
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the API token header is present
		if r.Header.Get("Purelymail-Api-Token") == "" {
			http.Error(w, "Missing API token", http.StatusUnauthorized)
			return
		}

		// Return a mock ownership code response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"result": {
				"code": "v=purelymail1 test-ownership-code-12345"
			}
		}`))
	}))
	defer server.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing with mock server
			{
				Config: testAccOwnershipProofDataSourceConfig(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(
						"data.purelymail_ownership_proof.test",
						tfjsonpath.New("code"),
						knownvalue.StringExact("v=purelymail1 test-ownership-code-12345"),
					),
					statecheck.ExpectKnownValue(
						"data.purelymail_ownership_proof.test",
						tfjsonpath.New("id"),
						knownvalue.StringExact("v=purelymail1 test-ownership-code-12345"),
					),
				},
			},
		},
	})
}

func testAccOwnershipProofDataSourceConfig(endpoint string) string {
	return `
provider "purelymail" {
  endpoint  = "` + endpoint + `"
  api_token = "test-token"
}

data "purelymail_ownership_proof" "test" {}
`
}
