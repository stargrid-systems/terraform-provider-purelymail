// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// mockAppPasswordServer implements api.ServerInterface for testing
type mockAppPasswordServer struct {
	mu              sync.Mutex
	passwords       map[string]string // appPassword -> userHandle
	nextPasswordNum int
}

func newMockAppPasswordServer() *mockAppPasswordServer {
	return &mockAppPasswordServer{
		passwords:       make(map[string]string),
		nextPasswordNum: 1,
	}
}

func (m *mockAppPasswordServer) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.CreateAppPassword
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a mock app password
	appPassword := fmt.Sprintf("app-password-%d", m.nextPasswordNum)
	m.nextPasswordNum++
	m.passwords[appPassword] = req.UserHandle

	result := api.CreateAppPasswordResponse{
		Result: &struct {
			AppPassword *string `json:"appPassword,omitempty"`
		}{
			AppPassword: &appPassword,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

func (m *mockAppPasswordServer) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.DeleteAppPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(m.passwords, req.AppPassword)

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

// Implement remaining ServerInterface methods as no-ops
func (m *mockAppPasswordServer) AddDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) GetUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) ListDomains(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) ModifyUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockAppPasswordServer) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func TestAccAppPasswordResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := newMockAppPasswordServer()
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
