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

func TestAccUserResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := newMockUserServer()
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

// mockUserServer implements api.ServerInterface for testing users.
type mockUserServer struct {
	mu        sync.Mutex
	userState map[string]interface{}
}

func newMockUserServer() *mockUserServer {
	return &mockUserServer{
		userState: map[string]interface{}{
			"enableSearchIndexing":           true,
			"recoveryEnabled":                false,
			"requireTwoFactorAuthentication": false,
			"enableSpamFiltering":            true,
		},
	}
}

func (m *mockUserServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (m *mockUserServer) ModifyUser(w http.ResponseWriter, r *http.Request) {
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
	if req.EnablePasswordReset != nil {
		m.userState["recoveryEnabled"] = *req.EnablePasswordReset
	}
	if req.RequireTwoFactorAuthentication != nil {
		m.userState["requireTwoFactorAuthentication"] = *req.RequireTwoFactorAuthentication
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (m *mockUserServer) GetUser(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	enableSearchIndexing, _ := m.userState["enableSearchIndexing"].(bool)
	recoveryEnabled, _ := m.userState["recoveryEnabled"].(bool)
	requireTwoFactorAuthentication, _ := m.userState["requireTwoFactorAuthentication"].(bool)
	enableSpamFiltering, _ := m.userState["enableSpamFiltering"].(bool)

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
			EnableSpamFiltering:            &enableSpamFiltering,
			RecoveryEnabled:                &recoveryEnabled,
			RequireTwoFactorAuthentication: &requireTwoFactorAuthentication,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (m *mockUserServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

// Implement remaining ServerInterface methods as no-ops.
func (m *mockUserServer) AddDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) ListDomains(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockUserServer) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
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
	mockServer := newMockUserServer()
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
