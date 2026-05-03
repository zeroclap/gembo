package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zeroclap/gembo/internal/domain"
	"github.com/zeroclap/gembo/internal/notification"
	"github.com/zeroclap/gembo/internal/notification/telegram"
)

func (h *Handler) requireNotificationStore(w http.ResponseWriter) bool {
	if h.notificationStore == nil {
		writeErr(w, http.StatusServiceUnavailable, "notifications_unavailable", "notification endpoints require a configured database")
		return false
	}
	return true
}

// ---- Channels ---------------------------------------------------------------

func (h *Handler) handleListChannels(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	channels, err := h.notificationStore.ListChannels(r.Context(), au.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to list channels")
		return
	}
	writeOK(w, channels)
}

func (h *Handler) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var ch domain.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if ch.Name == "" {
		writeErr(w, http.StatusBadRequest, "missing_fields", "name is required")
		return
	}
	if ch.Type != domain.ChannelTypeTelegram {
		writeErr(w, http.StatusBadRequest, "unsupported_channel_type", "supported types: telegram")
		return
	}
	if len(ch.Config) == 0 {
		ch.Config = json.RawMessage("{}")
	}

	ch.UserID = au.ID
	if err := h.notificationStore.CreateChannel(r.Context(), &ch); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to create channel")
		return
	}
	writeJSON(w, http.StatusCreated, response{Data: ch, TS: nowTS()})
}

func (h *Handler) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var ch domain.NotificationChannel
	if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	ch.ID = r.PathValue("id")
	ch.UserID = au.ID
	if len(ch.Config) == 0 {
		ch.Config = json.RawMessage("{}")
	}

	found, err := h.notificationStore.UpdateChannel(r.Context(), &ch)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to update channel")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}
	writeOK(w, ch)
}

func (h *Handler) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	channelID := r.PathValue("id")
	found, err := h.notificationStore.DeleteChannel(r.Context(), au.ID, channelID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to delete channel")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}
	writeOK(w, nil)
}

// handleTestChannel sends a test message through the specified channel.
func (h *Handler) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	channelID := r.PathValue("id")
	ch, err := h.notificationStore.GetChannel(r.Context(), au.ID, channelID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to load channel")
		return
	}
	if ch == nil {
		writeErr(w, http.StatusNotFound, "not_found", "channel not found")
		return
	}

	sender, err := buildChannel(ch)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_config", err.Error())
		return
	}

	msg := notification.Message{
		Title: "Gembo test notification",
		Body:  "Your notification channel is configured correctly.",
	}
	if err := sender.Send(r.Context(), msg); err != nil {
		writeErr(w, http.StatusBadGateway, "send_failed", "failed to send test message: "+err.Error())
		return
	}
	writeOK(w, nil)
}

// ---- Rules ------------------------------------------------------------------

func (h *Handler) handleListRules(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	rules, err := h.notificationStore.ListRules(r.Context(), au.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to list rules")
		return
	}
	writeOK(w, rules)
}

func (h *Handler) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var rule domain.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	if rule.ChannelID == "" {
		writeErr(w, http.StatusBadRequest, "missing_fields", "channel_id is required")
		return
	}
	if !validAlertRuleType(rule.Type) {
		writeErr(w, http.StatusBadRequest, "invalid_type", "unsupported alert rule type")
		return
	}
	if len(rule.Params) == 0 {
		rule.Params = json.RawMessage("{}")
	}
	if rule.CooldownMinutes <= 0 {
		rule.CooldownMinutes = 60
	}
	rule.UserID = au.ID

	if err := h.notificationStore.CreateRule(r.Context(), &rule); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to create rule")
		return
	}
	writeJSON(w, http.StatusCreated, response{Data: rule, TS: nowTS()})
}

func (h *Handler) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}

	var rule domain.AlertRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_body", "request body must be valid JSON")
		return
	}
	rule.ID = r.PathValue("id")
	rule.UserID = au.ID
	if len(rule.Params) == 0 {
		rule.Params = json.RawMessage("{}")
	}
	if rule.CooldownMinutes <= 0 {
		rule.CooldownMinutes = 60
	}

	found, err := h.notificationStore.UpdateRule(r.Context(), &rule)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to update rule")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "rule not found")
		return
	}
	writeOK(w, rule)
}

func (h *Handler) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	if !h.requireNotificationStore(w) {
		return
	}
	au, ok := h.currentAuthUser(w, r)
	if !ok {
		return
	}
	ruleID := r.PathValue("id")
	found, err := h.notificationStore.DeleteRule(r.Context(), au.ID, ruleID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal_error", "failed to delete rule")
		return
	}
	if !found {
		writeErr(w, http.StatusNotFound, "not_found", "rule not found")
		return
	}
	writeOK(w, nil)
}

// ---- Helpers ----------------------------------------------------------------

func validAlertRuleType(t domain.AlertRuleType) bool {
	switch t {
	case domain.AlertFundingAPR,
		domain.AlertSpreadThreshold,
		domain.AlertPositionOpened,
		domain.AlertPositionClosed,
		domain.AlertFundingPayment,
		domain.AlertMarginWarning:
		return true
	}
	return false
}

// buildChannel constructs a notification.Channel from a stored NotificationChannel.
func buildChannel(ch *domain.NotificationChannel) (notification.Channel, error) {
	switch ch.Type {
	case domain.ChannelTypeTelegram:
		var cfg domain.TelegramConfig
		if err := json.Unmarshal(ch.Config, &cfg); err != nil {
			return nil, fmt.Errorf("invalid telegram config: %w", err)
		}
		if cfg.BotToken == "" || cfg.ChatID == "" {
			return nil, fmt.Errorf("telegram channel requires bot_token and chat_id")
		}
		return telegram.New(cfg.BotToken, cfg.ChatID), nil
	default:
		return nil, fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}
