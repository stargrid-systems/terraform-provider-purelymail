// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccUserResource(t *testing.T) {
	// Track user state for mock API responses
	userState := map[string]interface{}{
		"enableSearchIndexing":           true,
		"recoveryEnabled":                false,
		"requireTwoFactorAuthentication": false,
		"enableSpamFiltering":            true,
	}

	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API token
		if r.Header.Get("Purelymail-Api-Token") == "" {
			http.Error(w, "Missing API token", http.StatusUnauthorized)
			return
		}

		// Route to appropriate handler
		switch r.RequestURI {
		case "/api/v0/createUser":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		case "/api/v0/modifyUser":
			// Parse request body to capture modifications
			body, err := io.ReadAll(r.Body)
			if err == nil {
				var reqBody map[string]interface{}
				if err := json.Unmarshal(body, &reqBody); err == nil {
					// Update tracked state based on request
					if v, ok := reqBody["enableSearchIndexing"]; ok {
						userState["enableSearchIndexing"] = v
					}
					if v, ok := reqBody["enablePasswordReset"]; ok {
						userState["recoveryEnabled"] = v
					}
					if v, ok := reqBody["requireTwoFactorAuthentication"]; ok {
						userState["requireTwoFactorAuthentication"] = v
					}
					if v, ok := reqBody["enableSpamFiltering"]; ok {
						userState["enableSpamFiltering"] = v
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		case "/api/v0/getUser":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp, _ := json.Marshal(map[string]interface{}{
				"result": userState,
			})
			_, _ = w.Write(resp)

		case "/api/v0/deleteUser":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccUserResourceConfig(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("alice")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("enable_search_indexing"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("id"), knownvalue.StringExact("alice")),
				},
			},
			// Update and Read testing
			{
				Config: testAccUserResourceConfigUpdated(server.URL),
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
	// Track user state for mock API responses
	userState := map[string]interface{}{
		"enableSearchIndexing":           true,
		"recoveryEnabled":                false,
		"requireTwoFactorAuthentication": false,
		"enableSpamFiltering":            true,
	}

	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API token
		if r.Header.Get("Purelymail-Api-Token") == "" {
			http.Error(w, "Missing API token", http.StatusUnauthorized)
			return
		}

		// Route to appropriate handler
		switch r.RequestURI {
		case "/api/v0/createUser":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		case "/api/v0/modifyUser":
			// Parse request body to capture modifications
			body, err := io.ReadAll(r.Body)
			if err == nil {
				var reqBody map[string]interface{}
				if err := json.Unmarshal(body, &reqBody); err == nil {
					// Update tracked state based on request
					if v, ok := reqBody["enableSearchIndexing"]; ok {
						userState["enableSearchIndexing"] = v
					}
					if v, ok := reqBody["enablePasswordReset"]; ok {
						userState["recoveryEnabled"] = v
					}
					if v, ok := reqBody["requireTwoFactorAuthentication"]; ok {
						userState["requireTwoFactorAuthentication"] = v
					}
					if v, ok := reqBody["enableSpamFiltering"]; ok {
						userState["enableSpamFiltering"] = v
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		case "/api/v0/getUser":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp, _ := json.Marshal(map[string]interface{}{
				"result": userState,
			})
			_, _ = w.Write(resp)

		case "/api/v0/deleteUser":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with password_wo
			{
				Config: testAccUserResourceConfigPasswordWo(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("bob")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("id"), knownvalue.StringExact("bob")),
				},
			},
			// Update with password_wo
			{
				Config: testAccUserResourceConfigPasswordWoUpdated(server.URL),
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
