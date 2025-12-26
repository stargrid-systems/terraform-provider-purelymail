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

// mockRoutingRuleServer implements api.ServerInterface for testing.
type mockRoutingRuleServer struct {
	mu     sync.Mutex
	rules  map[int32]api.RoutingRule
	nextId int32
}

func newMockRoutingRuleServer() *mockRoutingRuleServer {
	return &mockRoutingRuleServer{
		rules:  make(map[int32]api.RoutingRule),
		nextId: 1,
	}
}

func (m *mockRoutingRuleServer) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.CreateRoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create new rule
	id := m.nextId
	m.nextId++

	rule := api.RoutingRule{
		Id:              &id,
		DomainName:      &req.DomainName,
		Prefix:          &req.Prefix,
		MatchUser:       &req.MatchUser,
		TargetAddresses: &req.TargetAddresses,
		Catchall:        req.Catchall,
	}
	if rule.Catchall == nil {
		catchall := false
		rule.Catchall = &catchall
	}

	m.rules[id] = rule

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	result := make(map[string]interface{})
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockRoutingRuleServer) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var req api.DeleteRoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(m.rules, req.RoutingRuleId)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	result := make(map[string]interface{})
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockRoutingRuleServer) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rules := make([]api.RoutingRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}

	result := api.ListRoutingResponse{
		Result: &struct {
			Rules *[]api.RoutingRule `json:"rules,omitempty"`
		}{
			Rules: &rules,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Implement remaining ServerInterface methods as no-ops.
func (m *mockRoutingRuleServer) AddDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) GetUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) ListDomains(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) ModifyUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockRoutingRuleServer) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func TestAccRoutingRuleResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := newMockRoutingRuleServer()
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
