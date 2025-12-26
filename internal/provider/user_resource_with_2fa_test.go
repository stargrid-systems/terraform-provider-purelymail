package provider

import (
	"encoding/json"
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

func TestAccUserResourceWith2FA(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := newMockUserServerWith2FA()
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

// mockUserServerWith2FA implements api.ServerInterface for testing users with 2FA.
type mockUserServerWith2FA struct {
	mu                 sync.Mutex
	userState          map[string]interface{}
	passwordResetState []map[string]interface{}
}

func newMockUserServerWith2FA() *mockUserServerWith2FA {
	return &mockUserServerWith2FA{
		userState: map[string]interface{}{
			"enableSearchIndexing":           false,
			"requireTwoFactorAuthentication": false,
		},
		passwordResetState: []map[string]interface{}{},
	}
}

func (m *mockUserServerWith2FA) CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (m *mockUserServerWith2FA) ModifyUser(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.ModifyUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Update tracked state based on request
	if req.EnableSearchIndexing != nil {
		m.userState["enableSearchIndexing"] = *req.EnableSearchIndexing
	}
	if req.RequireTwoFactorAuthentication != nil {
		m.userState["requireTwoFactorAuthentication"] = *req.RequireTwoFactorAuthentication
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (m *mockUserServerWith2FA) GetUser(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	enableSearchIndexing, _ := m.userState["enableSearchIndexing"].(bool)
	requireTwoFactorAuthentication, _ := m.userState["requireTwoFactorAuthentication"].(bool)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := api.GetUserResponse{
		Result: &struct {
			EnableSearchIndexing           *bool                             `json:"enableSearchIndexing,omitempty"`
			EnableSpamFiltering            *bool                             `json:"enableSpamFiltering,omitempty"`
			RecoveryEnabled                *bool                             `json:"recoveryEnabled,omitempty"`
			RequireTwoFactorAuthentication *bool                             `json:"requireTwoFactorAuthentication,omitempty"`
			ResetMethods                   *[]api.GetUserPasswordResetMethod `json:"resetMethods,omitempty"`
		}{
			EnableSearchIndexing:           &enableSearchIndexing,
			RequireTwoFactorAuthentication: &requireTwoFactorAuthentication,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (m *mockUserServerWith2FA) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]interface{}{
		"result": map[string]interface{}{
			"users": m.passwordResetState,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (m *mockUserServerWith2FA) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.UpsertPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert request to map for storage
	reqData := map[string]interface{}{
		"type":   req.Type,
		"target": req.Target,
	}
	if req.Description != nil {
		reqData["description"] = *req.Description
	}
	if req.AllowMfaReset != nil {
		reqData["allowMfaReset"] = *req.AllowMfaReset
	}

	// Check if method already exists
	found := false
	for i, method := range m.passwordResetState {
		if method["target"] == req.Target {
			m.passwordResetState[i] = reqData
			found = true
			break
		}
	}
	if !found {
		m.passwordResetState = append(m.passwordResetState, reqData)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (m *mockUserServerWith2FA) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.DeletePasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newState := []map[string]interface{}{}
	for _, method := range m.passwordResetState {
		if method["target"] != req.Target {
			newState = append(newState, method)
		}
	}
	m.passwordResetState = newState

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (m *mockUserServerWith2FA) DeleteUser(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.passwordResetState = []map[string]interface{}{}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

// Implement remaining ServerInterface methods as no-ops.
func (m *mockUserServerWith2FA) AddDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) ListDomains(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServerWith2FA) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
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
