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

func TestAccDomainResource(t *testing.T) {
	// Create mock server using generated ServerInterface
	mockServer := newMockDomainServer()
	handler := api.Handler(mockServer)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccDomainResourceConfig(ts.URL, "example.com", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_domain.test", "name", "example.com"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "allow_account_reset", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "symbolic_subaddressing", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "is_shared", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_mx", "true"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_spf", "true"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_dkim", "false"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "dns_summary.passes_dmarc", "false"),
				),
			},
			// ImportState testing
			{
				ResourceName:            "purelymail_domain.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateId:           "example.com",
				ImportStateVerifyIgnore: []string{"recheck_dns"},
			},
			// Update and Read testing
			{
				Config: testAccDomainResourceConfig(ts.URL, "example.com", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("purelymail_domain.test", "allow_account_reset", "true"),
					resource.TestCheckResourceAttr("purelymail_domain.test", "symbolic_subaddressing", "true"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccDomainResourceConfig(endpoint string, domainName string, allowAccountReset bool, symbolicSubaddressing bool) string {
	return fmt.Sprintf(`
provider "purelymail" {
	endpoint  = %[1]q
	api_token = "test-token"
}

resource "purelymail_domain" "test" {
	name                     = %[2]q
	allow_account_reset      = %[3]t
	symbolic_subaddressing   = %[4]t
}
`, endpoint, domainName, allowAccountReset, symbolicSubaddressing)
}

// mockDomainServer implements api.ServerInterface for testing domains.
type mockDomainServer struct {
	domains map[string]api.ApiDomainInfo
	mu      sync.Mutex
}

func newMockDomainServer() *mockDomainServer {
	return &mockDomainServer{
		domains: make(map[string]api.ApiDomainInfo),
	}
}

func (m *mockDomainServer) AddDomain(w http.ResponseWriter, r *http.Request) {
	var req api.AddDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Create domain with default settings and DNS summary
	passesMx := true
	passesSpf := true
	passesDkim := false
	passesDmarc := false
	allowAccountReset := false
	symbolicSubaddressing := false
	isShared := false

	m.domains[req.DomainName] = api.ApiDomainInfo{
		Name:                  &req.DomainName,
		AllowAccountReset:     &allowAccountReset,
		SymbolicSubaddressing: &symbolicSubaddressing,
		IsShared:              &isShared,
		DnsSummary: &api.ApiDomainDnsSummary{
			PassesMx:    &passesMx,
			PassesSpf:   &passesSpf,
			PassesDkim:  &passesDkim,
			PassesDmarc: &passesDmarc,
		},
	}

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockDomainServer) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	var req api.DeleteDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.domains, req.Name)

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockDomainServer) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	var req api.UpdateDomainSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	domain, exists := m.domains[req.Name]
	if !exists {
		http.Error(w, "domain not found", http.StatusNotFound)
		return
	}

	// Update settings if provided
	if req.AllowAccountReset != nil {
		domain.AllowAccountReset = req.AllowAccountReset
	}
	if req.SymbolicSubaddressing != nil {
		domain.SymbolicSubaddressing = req.SymbolicSubaddressing
	}

	// If recheckDns is true, simulate DNS checks passing
	if req.RecheckDns != nil && *req.RecheckDns {
		if domain.DnsSummary != nil {
			passesMx := true
			passesSpf := true
			passesDkim := true
			passesDmarc := true
			domain.DnsSummary = &api.ApiDomainDnsSummary{
				PassesMx:    &passesMx,
				PassesSpf:   &passesSpf,
				PassesDkim:  &passesDkim,
				PassesDmarc: &passesDmarc,
			}
		}
	}

	m.domains[req.Name] = domain

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (m *mockDomainServer) ListDomains(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var domains []api.ApiDomainInfo
	for _, domain := range m.domains {
		domainCopy := domain
		domains = append(domains, domainCopy)
	}

	resp := api.ListDomainsResponse{
		Result: &struct {
			Domains *[]api.ApiDomainInfo `json:"domains,omitempty"`
		}{
			Domains: &domains,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Implement remaining ServerInterface methods as no-ops.
func (m *mockDomainServer) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) CreateUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) DeleteUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) GetUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) ModifyUser(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (m *mockDomainServer) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
