// Package mock provides a unified mock server implementing api.ServerInterface for testing.
package mock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/stargrid-systems/terraform-provider-purelymail/internal/api"
)

// Server is a unified mock implementation of api.ServerInterface for testing.
// It maintains in-memory state for all resource types.
type Server struct {
	mu sync.Mutex

	// Resource state
	users          map[string]userState
	domains        map[string]api.ApiDomainInfo
	routingRules   map[int32]api.RoutingRule
	appPasswords   map[string]string                              // appPassword -> userHandle
	passwordResets map[string][]api.ListPasswordResetResponseItem // userName -> slice of methods

	// ID generators
	nextRoutingRuleID int32
	nextAppPasswordID int
}

// userState tracks user configuration.
type userState struct {
	enableSearchIndexing           bool
	recoveryEnabled                bool
	requireTwoFactorAuthentication bool
	enableSpamFiltering            bool
}

// NewServer creates a new mock server with empty state.
func NewServer() *Server {
	return &Server{
		users:             make(map[string]userState),
		domains:           make(map[string]api.ApiDomainInfo),
		routingRules:      make(map[int32]api.RoutingRule),
		appPasswords:      make(map[string]string),
		passwordResets:    make(map[string][]api.ListPasswordResetResponseItem),
		nextRoutingRuleID: 1,
		nextAppPasswordID: 1,
	}
}

// User Management

func (s *Server) CreateUser(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Initialize user with defaults
	s.users[req.UserName] = userState{
		enableSearchIndexing:           true,
		recoveryEnabled:                false,
		requireTwoFactorAuthentication: false,
		enableSpamFiltering:            true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (s *Server) ModifyUser(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.ModifyUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, exists := s.users[req.UserName]
	if !exists {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Update user state based on request
	if req.EnableSearchIndexing != nil {
		user.enableSearchIndexing = *req.EnableSearchIndexing
	}
	if req.EnablePasswordReset != nil {
		user.recoveryEnabled = *req.EnablePasswordReset
	}
	if req.RequireTwoFactorAuthentication != nil {
		user.requireTwoFactorAuthentication = *req.RequireTwoFactorAuthentication
	}

	s.users[req.UserName] = user

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (s *Server) GetUser(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.GetUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user, exists := s.users[req.UserName]
	if !exists {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

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
			EnableSearchIndexing:           &user.enableSearchIndexing,
			EnableSpamFiltering:            &user.enableSpamFiltering,
			RecoveryEnabled:                &user.recoveryEnabled,
			RequireTwoFactorAuthentication: &user.requireTwoFactorAuthentication,
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) DeleteUser(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.DeleteUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(s.users, req.UserName)
	delete(s.passwordResets, req.UserName)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &map[string]interface{}{}})
}

func (s *Server) ListUsers(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

// Domain Management

func (s *Server) AddDomain(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.AddDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create domain with default settings and DNS summary
	passesMx := true
	passesSpf := true
	passesDkim := false
	passesDmarc := false
	allowAccountReset := false
	symbolicSubaddressing := false
	isShared := false

	s.domains[req.DomainName] = api.ApiDomainInfo{
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

func (s *Server) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.DeleteDomainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(s.domains, req.Name)

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (s *Server) UpdateDomainSettings(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.UpdateDomainSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	domain, exists := s.domains[req.Name]
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

	s.domains[req.Name] = domain

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (s *Server) ListDomains(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var domains []api.ApiDomainInfo
	for _, domain := range s.domains {
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

func (s *Server) GetOwnershipCode(w http.ResponseWriter, r *http.Request) {
	// Return mock ownership code
	ownershipCode := "mock-ownership-code-123"
	result := map[string]interface{}{
		"code": ownershipCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": result})
}

// Routing Rule Management

func (s *Server) CreateRoutingRule(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.CreateRoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Create new rule
	id := s.nextRoutingRuleID
	s.nextRoutingRuleID++

	rule := api.RoutingRule{
		Id:              &id,
		DomainName:      &req.DomainName,
		Prefix:          &req.Prefix,
		MatchUser:       &req.MatchUser,
		TargetAddresses: &req.TargetAddresses,
	}

	if req.Prefix {
		rule.Catchall = nil
	} else {
		catchall := req.MatchUser == "*"
		rule.Catchall = &catchall
	}

	s.routingRules[id] = rule

	result := map[string]interface{}{
		"routingRuleId": id,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"result": result})
}

func (s *Server) DeleteRoutingRule(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.DeleteRoutingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(s.routingRules, req.RoutingRuleId)

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (s *Server) ListRoutingRules(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rules []api.RoutingRule
	for _, rule := range s.routingRules {
		ruleCopy := rule
		rules = append(rules, ruleCopy)
	}

	resp := api.ListRoutingResponse{
		Result: &struct {
			Rules *[]api.RoutingRule `json:"rules,omitempty"`
		}{
			Rules: &rules,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// App Password Management

func (s *Server) CreateAppPassword(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.CreateAppPassword
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate a mock app password
	appPassword := fmt.Sprintf("app-password-%d", s.nextAppPasswordID)
	s.nextAppPasswordID++
	s.appPasswords[appPassword] = req.UserHandle

	result := api.CreateAppPasswordResponse{
		Result: &struct {
			AppPassword *string `json:"appPassword,omitempty"`
		}{
			AppPassword: &appPassword,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (s *Server) DeleteAppPassword(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.DeleteAppPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	delete(s.appPasswords, req.AppPassword)

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

// Password Reset Method Management

func (s *Server) CreateOrUpdatePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.UpsertPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if s.passwordResets[req.UserName] == nil {
		s.passwordResets[req.UserName] = []api.ListPasswordResetResponseItem{}
	}

	method := api.ListPasswordResetResponseItem{
		Type:          &req.Type,
		Target:        &req.Target,
		Description:   req.Description,
		AllowMfaReset: req.AllowMfaReset,
	}

	// Update or create the method
	found := false
	for i, m := range s.passwordResets[req.UserName] {
		if m.Target != nil && *m.Target == req.Target {
			s.passwordResets[req.UserName][i] = method
			found = true
			break
		}
	}
	if !found {
		s.passwordResets[req.UserName] = append(s.passwordResets[req.UserName], method)
	}

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (s *Server) DeletePasswordResetMethod(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.DeletePasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if userMethods, exists := s.passwordResets[req.UserName]; exists {
		// Remove the method with matching target
		for i, m := range userMethods {
			if m.Target != nil && *m.Target == req.Target {
				s.passwordResets[req.UserName] = append(userMethods[:i], userMethods[i+1:]...)
				break
			}
		}
		if len(s.passwordResets[req.UserName]) == 0 {
			delete(s.passwordResets, req.UserName)
		}
	}

	result := make(map[string]interface{})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.EmptyResponse{Result: &result})
}

func (s *Server) ListPasswordResetMethods(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req api.ListPasswordResetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var methods []api.ListPasswordResetResponseItem
	if userMethods, exists := s.passwordResets[req.UserName]; exists {
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

// CheckAccountCredit is not implemented for tests.
func (s *Server) CheckAccountCredit(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
