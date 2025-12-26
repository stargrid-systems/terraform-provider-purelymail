// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccUserResourceWith2FA(t *testing.T) {
	// Mock server state
	var (
		mu                 sync.Mutex
		userState          map[string]interface{}
		passwordResetState []map[string]interface{}
	)

	// Initialize state
	userState = map[string]interface{}{
		"enableSearchIndexing":           false,
		"requireTwoFactorAuthentication": false,
	}
	passwordResetState = []map[string]interface{}{}

	// Create a mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

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
			body, err := io.ReadAll(r.Body)
			if err == nil {
				var reqBody map[string]interface{}
				if err := json.Unmarshal(body, &reqBody); err == nil {
					if v, ok := reqBody["enableSearchIndexing"]; ok {
						userState["enableSearchIndexing"] = v
					}
					if v, ok := reqBody["requireTwoFactorAuthentication"]; ok {
						userState["requireTwoFactorAuthentication"] = v
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

		case "/api/v0/listPasswordResetMethods":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			resp, _ := json.Marshal(map[string]interface{}{
				"result": map[string]interface{}{
					"users": passwordResetState,
				},
			})
			_, _ = w.Write(resp)

		case "/api/v0/upsertPasswordReset":
			body, err := io.ReadAll(r.Body)
			if err == nil {
				var reqBody map[string]interface{}
				if err := json.Unmarshal(body, &reqBody); err == nil {
					// Check if method already exists
					target, ok := reqBody["target"].(string)
					if !ok {
						http.Error(w, "invalid target", http.StatusBadRequest)
						return
					}
					found := false
					for i, method := range passwordResetState {
						if method["target"] == target {
							passwordResetState[i] = reqBody
							found = true
							break
						}
					}
					if !found {
						passwordResetState = append(passwordResetState, reqBody)
					}
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		case "/api/v0/deletePasswordReset":
			body, err := io.ReadAll(r.Body)
			if err == nil {
				var reqBody map[string]interface{}
				if err := json.Unmarshal(body, &reqBody); err == nil {
					target, ok := reqBody["target"].(string)
					if !ok {
						http.Error(w, "invalid target", http.StatusBadRequest)
						return
					}
					newState := []map[string]interface{}{}
					for _, method := range passwordResetState {
						if method["target"] != target {
							newState = append(newState, method)
						}
					}
					passwordResetState = newState
				}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{}}`))

		case "/api/v0/deleteUser":
			passwordResetState = []map[string]interface{}{}
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
			// Create user with password reset methods and 2FA
			{
				Config: testAccUserResourceWith2FAConfig(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("user_name"), knownvalue.StringExact("charlie")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("require_two_factor_authentication"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("password_reset_methods").AtSliceIndex(0).AtMapKey("type"), knownvalue.StringExact("email")),
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("password_reset_methods").AtSliceIndex(0).AtMapKey("target"), knownvalue.StringExact("charlie@recovery.example.com")),
				},
			},
			// Update: Add another password reset method
			{
				Config: testAccUserResourceWith2FAConfigUpdated(server.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("purelymail_user.test", tfjsonpath.New("require_two_factor_authentication"), knownvalue.Bool(true)),
				},
			},
			// Update: Disable 2FA
			{
				Config: testAccUserResourceWith2FAConfigNo2FA(server.URL),
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
