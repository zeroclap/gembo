package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/exchanges/base"
	"github.com/zeroclap/gembo/internal/store"
)

type mockUserStore struct {
	mu           sync.Mutex
	usersByID    map[string]*domain.User
	usersByEmail map[string]*domain.User
	credsByUser  map[string][]domain.ExchangeCredential
	nextCredID   int
}

func newMockUserStore() *mockUserStore {
	return &mockUserStore{
		usersByID:    make(map[string]*domain.User),
		usersByEmail: make(map[string]*domain.User),
		credsByUser:  make(map[string][]domain.ExchangeCredential),
		nextCredID:   1,
	}
}

func (m *mockUserStore) GetUserByEmail(_ context.Context, email string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.usersByEmail[email]
	if u == nil {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (m *mockUserStore) GetUserByID(_ context.Context, id string) (*domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := m.usersByID[id]
	if u == nil {
		return nil, nil
	}
	cp := *u
	return &cp, nil
}

func (m *mockUserStore) CreateUser(_ context.Context, u *domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if u.ID == "" {
		u.ID = "user-created"
	}
	m.usersByID[u.ID] = u
	m.usersByEmail[u.Email] = u
	return nil
}

func (m *mockUserStore) ListExchangeCredentialsByUserID(_ context.Context, userID string) ([]domain.ExchangeCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.credsByUser[userID]
	out := make([]domain.ExchangeCredential, len(items))
	copy(out, items)
	return out, nil
}

func (m *mockUserStore) ListExchangeCredentials(_ context.Context) ([]domain.ExchangeCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]domain.ExchangeCredential, 0)
	for _, creds := range m.credsByUser {
		items = append(items, creds...)
	}
	return items, nil
}

func (m *mockUserStore) GetExchangeCredentialByID(_ context.Context, userID, credentialID string) (*domain.ExchangeCredential, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.credsByUser[userID] {
		if c.ID == credentialID {
			cp := c
			return &cp, nil
		}
	}
	return nil, nil
}

func (m *mockUserStore) CreateExchangeCredential(_ context.Context, c *domain.ExchangeCredential) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.IsDefault {
		for i := range m.credsByUser[c.UserID] {
			if m.credsByUser[c.UserID][i].ExchangeID == c.ExchangeID {
				m.credsByUser[c.UserID][i].IsDefault = false
			}
		}
	}
	c.ID = "cred-" + strconv.Itoa(m.nextCredID)
	m.nextCredID++
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	m.credsByUser[c.UserID] = append(m.credsByUser[c.UserID], *c)
	return nil
}

func (m *mockUserStore) UpdateExchangeCredential(_ context.Context, c *domain.ExchangeCredential) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.credsByUser[c.UserID]
	idx := -1
	for i := range items {
		if items[i].ID == c.ID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, nil
	}

	if c.IsDefault {
		for i := range items {
			if items[i].ExchangeID == c.ExchangeID && items[i].ID != c.ID {
				items[i].IsDefault = false
			}
		}
	}

	items[idx].ExchangeID = c.ExchangeID
	items[idx].Title = c.Title
	items[idx].APIKey = c.APIKey
	items[idx].APISecret = c.APISecret
	items[idx].Passphrase = c.Passphrase
	items[idx].IsDefault = c.IsDefault
	items[idx].UpdatedAt = time.Now().UTC()
	m.credsByUser[c.UserID] = items
	return true, nil
}

func (m *mockUserStore) DeleteExchangeCredential(_ context.Context, userID, credentialID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := m.credsByUser[userID]
	for i := range items {
		if items[i].ID == credentialID {
			m.credsByUser[userID] = append(items[:i], items[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (m *mockUserStore) GetWorkspaces(_ context.Context, _ string) ([]domain.ArbitrageWorkspace, error) {
	return []domain.ArbitrageWorkspace{}, nil
}

func (m *mockUserStore) SaveWorkspaces(_ context.Context, _ string, _ []domain.ArbitrageWorkspace) error {
	return nil
}

var _ store.UserStore = (*mockUserStore)(nil)

func newAccountHandler(us store.UserStore) *Handler {
	reg := base.NewRegistry()
	st := &mockStore{}
	return NewHandler(reg, st, WithUserStore(us), WithJWTSecret(testJWTSecret), WithTokenTTL(time.Hour))
}

func TestGetAccount(t *testing.T) {
	us := newMockUserStore()
	us.usersByID["test-user-id"] = &domain.User{ID: "test-user-id", Email: "acc@example.com"}
	h := newAccountHandler(us)

	req := newAuthRequest(http.MethodGet, "/v1/account", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestExchangeCredentialLifecycle(t *testing.T) {
	us := newMockUserStore()
	us.usersByID["test-user-id"] = &domain.User{ID: "test-user-id", Email: "acc@example.com"}
	h := newAccountHandler(us)

	createBody := `{"exchange_id":"okx","title":"Main","api_key":"key123456","api_secret":"secret123","passphrase":"pass123","is_default":true}`
	createReq := newAuthRequest(http.MethodPost, "/v1/account/credentials", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	h.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	listReq := newAuthRequest(http.MethodGet, "/v1/account/credentials", nil)
	listRec := httptest.NewRecorder()
	h.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", listRec.Code)
	}

	var listEnvelope struct {
		Data []domain.ExchangeCredential `json:"data"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&listEnvelope); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listEnvelope.Data) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(listEnvelope.Data))
	}
	credentialID := listEnvelope.Data[0].ID

	updateBody := `{"exchange_id":"okx","title":"Updated","api_key":"key-updated","api_secret":"secret-updated","passphrase":"pass-updated","is_default":false}`
	updateReq := newAuthRequest(http.MethodPut, "/v1/account/credentials/"+credentialID, bytes.NewBufferString(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	h.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", updateRec.Code, updateRec.Body.String())
	}

	deleteReq := newAuthRequest(http.MethodDelete, "/v1/account/credentials/"+credentialID, nil)
	deleteRec := httptest.NewRecorder()
	h.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", deleteRec.Code)
	}
}

func TestCreateExchangeCredential_RequiresPassphraseForOKX(t *testing.T) {
	us := newMockUserStore()
	us.usersByID["test-user-id"] = &domain.User{ID: "test-user-id", Email: "acc@example.com"}
	h := newAccountHandler(us)

	body := `{"exchange_id":"okx","title":"Main","api_key":"key123456","api_secret":"secret123","is_default":true}`
	req := newAuthRequest(http.MethodPost, "/v1/account/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
