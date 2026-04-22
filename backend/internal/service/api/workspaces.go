package api

import (
	"encoding/json"
	"net/http"

	"github.com/zeroclap/gembo/internal/domain"
)

// handleGetWorkspaces returns the saved arbitrage workspaces for the authenticated user.
func (h *Handler) handleGetWorkspaces(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	workspaces, err := h.userStore.GetWorkspaces(r.Context(), au.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to load workspaces")
		return
	}
	writeOK(w, workspaces)
}

type saveWorkspacesRequest struct {
	Workspaces []domain.ArbitrageWorkspace `json:"workspaces"`
}

// handleSaveWorkspaces replaces the saved arbitrage workspaces for the authenticated user.
func (h *Handler) handleSaveWorkspaces(w http.ResponseWriter, r *http.Request) {
	if !h.requireUserStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var req saveWorkspacesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if req.Workspaces == nil {
		req.Workspaces = []domain.ArbitrageWorkspace{}
	}

	if err := h.userStore.SaveWorkspaces(r.Context(), au.ID, req.Workspaces); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to save workspaces")
		return
	}
	writeOK(w, req.Workspaces)
}
