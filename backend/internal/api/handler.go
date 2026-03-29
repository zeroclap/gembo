// Package api implements the HTTP REST and WebSocket API for gembo.
package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// response is the standard JSON envelope for all REST responses.
type response struct {
	Data  interface{}   `json:"data"`
	Error *errorPayload `json:"error"`
	TS    string        `json:"ts"`
}

// errorPayload carries a machine-readable code and human message.
type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON encodes v as JSON to w with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Nothing useful we can do at this point.
		return
	}
}

// writeOK writes a successful response envelope.
func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, response{
		Data:  data,
		Error: nil,
		TS:    time.Now().UTC().Format(time.RFC3339),
	})
}

// writeErr writes an error response envelope.
func writeErr(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, response{
		Data:  nil,
		Error: &errorPayload{Code: code, Message: message},
		TS:    time.Now().UTC().Format(time.RFC3339),
	})
}
