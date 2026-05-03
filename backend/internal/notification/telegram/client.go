// Package telegram implements notification.Channel using the Telegram Bot API.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zeroclap/gembo/internal/notification"
)

const apiBase = "https://api.telegram.org"

// Client sends messages via a Telegram bot.
type Client struct {
	botToken   string
	chatID     string
	httpClient *http.Client
}

// New creates a Telegram notification client.
// botToken is the token issued by @BotFather.
// chatID is the destination chat (user ID, group ID, or @channel_username).
func New(botToken, chatID string) *Client {
	return &Client{
		botToken:   botToken,
		chatID:     chatID,
		httpClient: &http.Client{},
	}
}

// Send delivers a message to the configured Telegram chat using sendMessage.
// The title and body are combined as HTML-formatted text.
func (c *Client) Send(ctx context.Context, msg notification.Message) error {
	text := fmt.Sprintf("<b>%s</b>\n\n%s", escapeHTML(msg.Title), msg.Body)

	payload := map[string]string{
		"chat_id":    c.chatID,
		"text":       text,
		"parse_mode": "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram send marshal: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", apiBase, c.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram send new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telegram send http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiResp struct {
			Description string `json:"description"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&apiResp)
		return fmt.Errorf("telegram send: HTTP %d: %s", resp.StatusCode, apiResp.Description)
	}

	return nil
}

// escapeHTML replaces HTML special characters to avoid Telegram parse errors.
func escapeHTML(s string) string {
	s = replaceAll(s, "&", "&amp;")
	s = replaceAll(s, "<", "&lt;")
	s = replaceAll(s, ">", "&gt;")
	return s
}

func replaceAll(s, old, new string) string {
	var buf bytes.Buffer
	for i := 0; i < len(s); {
		if len(s)-i >= len(old) && s[i:i+len(old)] == old {
			buf.WriteString(new)
			i += len(old)
		} else {
			buf.WriteByte(s[i])
			i++
		}
	}
	return buf.String()
}
