package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

func TestAccPasswordResetMethodResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := newMockPasswordResetMethodServer()
	handler := api.Handler(mockServer)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccPasswordResetMethodResourceConfig(ts.URL, "alice", "email", "alice@recovery.example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "user_name", "alice"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "type", "email"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "target", "alice@recovery.example.com"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "allow_mfa_reset", "false"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "purelymail_password_reset_method.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     "alice:alice@recovery.example.com",
			},
			// Update and Read testing
			{
				Config: testAccPasswordResetMethodResourceConfigWithDescription(ts.URL, "alice", "email", "alice@recovery.example.com", "Updated recovery email", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "description", "Updated recovery email"),
					resource.TestCheckResourceAttr("purelymail_password_reset_method.test", "allow_mfa_reset", "true"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccPasswordResetMethodResourceConfig(endpoint string, userName string, methodType string, target string) string {
	return fmt.Sprintf(`
provider "purelymail" {
  endpoint  = %[1]q
  api_token = "test-token"
}

resource "purelymail_password_reset_method" "test" {
  user_name = %[2]q
  type      = %[3]q
  target    = %[4]q
}
`, endpoint, userName, methodType, target)
}

func testAccPasswordResetMethodResourceConfigWithDescription(endpoint string, userName string, methodType string, target string, description string, allowMfaReset bool) string {
	return fmt.Sprintf(`
provider "purelymail" {
  endpoint  = %[1]q
  api_token = "test-token"
}

resource "purelymail_password_reset_method" "test" {
  user_name       = %[2]q
  type            = %[3]q
  target          = %[4]q
  description     = %[5]q
  allow_mfa_reset = %[6]t
}
`, endpoint, userName, methodType, target, description, allowMfaReset)
}

// mockPasswordResetMethodServer implements api.ServerInterface for testing password reset methods.
type mockPasswordResetMethodServer struct {
	methods map[string]map[string]api.ListPasswordResetResponseItem // userName -> target -> method
	mu      sync.Mutex
}

func newMockPasswordResetMethodServer() *mockPasswordResetMethodServer {
	return &mockPasswordResetMethodServer{
		methods: make(map[string]map[string]api.ListPasswordResetResponseItem),
	}
}

func (m *mockPasswordResetMethodServer) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	var req api.UpsertPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.methods[req.UserName] == nil {
		m.methods[req.UserName] = make(map[string]api.ListPasswordResetResponseItem)
	}

	method := api.ListPasswordResetResponseItem{
		Type:          &req.Type,
		Target:        &req.Target,
		Description:   req.Description,
		AllowMfaReset: req.AllowMfaReset,
	}

	m.methods[req.UserName][req.Target] = method

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockPasswordResetMethodServer) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	var req api.DeletePasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.methods[req.UserName] != nil {
		delete(m.methods[req.UserName], req.Target)
	}

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockPasswordResetMethodServer) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	var req api.ListPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	methods := []api.ListPasswordResetResponseItem{}
	if userMethods, exists := m.methods[req.UserName]; exists {
		for _, method := range userMethods {
			methodCopy := method
			methods = append(methods, methodCopy)
		}
	}

	resp := api.ListPasswordResetResponse{
		Result: &struct {
			Users *[]api.ListPasswordResetResponseItem `json:"users,omitempty"`
		}{
			Users: &methods,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Implement remaining ServerInterface methods as no-ops.
func (m *mockPasswordResetMethodServer) AddDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) GetUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) ListDomains(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) ModifyUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockPasswordResetMethodServer) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
