package api

import "net/http"

// handleFlushAll deletes all stored data (markets, tickers, funding rates)
// from Redis and returns the number of keys removed.
func (h *Handler) handleFlushAll(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.store.FlushAll(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	writeOK(w, map[string]int64{"deleted_keys": deleted})
}
