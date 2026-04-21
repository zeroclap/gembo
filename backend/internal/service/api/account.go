package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zeroclap/gembo/internal/domain"
)

var exchangePassphraseRequired = map[string]bool{
	"binance": false,
	"bybit":   false,
	"gate":    false,
	"okx":     true,
}

func (h *Handler) requireUserStore(w http.ResponseWriter) bool {
	if h.userStore == nil {
		writeErr(w, http.StatusServiceUnavailable, "account_unavailable", "account endpoints require a configured database")
		return false
	}
	return true
}

func (h *Handler) currentAuthUser(w http.ResponseWriter, r *http.Request) (authUser, bool) {
	u, ok := userFromContext(r.Context())
	if !ok || u.ID == "" {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "invalid auth context")
		return authUser{}, false
	}
	return u, true
}

func (h *Handler) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	u, err := h.userStore.GetUserByID(r.Context(), au.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to load account")
		return
	}
	if u == nil {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "account no longer exists")
		return
	}

	writeOK(w, u)
}

func (h *Handler) handleListExchangeCredentials(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	creds, err := h.userStore.ListExchangeCredentialsByUserID(r.Context(), au.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to list credentials")
		return
	}

	writeOK(w, creds)
}

type upsertExchangeCredentialRequest struct {
	ExchangeID string `json:"exchange_id"`
	Title      string `json:"title"`
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	Passphrase string `json:"passphrase"`
	IsDefault  bool   `json:"is_default"`
}

func validateCredentialRequest(req upsertExchangeCredentialRequest) (upsertExchangeCredentialRequest, string) {
	req.ExchangeID = strings.ToLower(strings.TrimSpace(req.ExchangeID))
	req.Title = strings.TrimSpace(req.Title)
	req.APIKey = strings.TrimSpace(req.APIKey)
	req.APISecret = strings.TrimSpace(req.APISecret)
	req.Passphrase = strings.TrimSpace(req.Passphrase)

	hasPassphrase, ok := exchangePassphraseRequired[req.ExchangeID]
	if !ok {
		return req, "exchange_id must be one of: binance, bybit, gate, okx"
	}
	if req.Title == "" {
		return req, "title is required"
	}
	if req.APIKey == "" {
		return req, "api_key is required"
	}
	if req.APISecret == "" {
		return req, "api_secret is required"
	}
	if hasPassphrase && req.Passphrase == "" {
		return req, "passphrase is required for okx"
	}
	if !hasPassphrase {
		req.Passphrase = ""
	}

	return req, ""
}

func (h *Handler) handleCreateExchangeCredential(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var req upsertExchangeCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	normalized, msg := validateCredentialRequest(req)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, "invalid_credential", msg)
		return
	}

	cred := &domain.ExchangeCredential{
		UserID:     au.ID,
		ExchangeID: normalized.ExchangeID,
		Title:      normalized.Title,
		APIKey:     normalized.APIKey,
		APISecret:  normalized.APISecret,
		Passphrase: normalized.Passphrase,
		IsDefault:  normalized.IsDefault,
	}

	if err := h.userStore.CreateExchangeCredential(r.Context(), cred); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to create credential")
		return
	}

	writeJSON(w, http.StatusCreated, response{Data: cred, Error: nil, TS: nowTS()})
}

func (h *Handler) handleUpdateExchangeCredential(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	credentialID := strings.TrimSpace(r.PathValue("id"))
	if credentialID == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "credential id is required")
		return
	}

	var req upsertExchangeCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}

	normalized, msg := validateCredentialRequest(req)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, "invalid_credential", msg)
		return
	}

	cred := &domain.ExchangeCredential{
		ID:         credentialID,
		UserID:     au.ID,
		ExchangeID: normalized.ExchangeID,
		Title:      normalized.Title,
		APIKey:     normalized.APIKey,
		APISecret:  normalized.APISecret,
		Passphrase: normalized.Passphrase,
		IsDefault:  normalized.IsDefault,
	}

	updated, err := h.userStore.UpdateExchangeCredential(r.Context(), cred)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to update credential")
		return
	}
	if !updated {
		writeErr(w, http.StatusNotFound, "not_found", "credential not found")
		return
	}

	writeOK(w, cred)
}

func (h *Handler) handleDeleteExchangeCredential(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}

	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	credentialID := strings.TrimSpace(r.PathValue("id"))
	if credentialID == "" {
		writeErr(w, http.StatusBadRequest, "missing_id", "credential id is required")
		return
	}

	deleted, err := h.userStore.DeleteExchangeCredential(r.Context(), au.ID, credentialID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to delete credential")
		return
	}
	if !deleted {
		writeErr(w, http.StatusNotFound, "not_found", "credential not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
