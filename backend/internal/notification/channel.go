// Package notification defines the abstraction for sending alert notifications.
package notification

import "context"

// Message is the content to send over a notification channel.
type Message struct {
	Title string
	Body  string
}

// Channel delivers notification messages to a user through a specific transport.
type Channel interface {
	// Send delivers the message. Returns an error if delivery fails.
	Send(ctx context.Context, msg Message) error
}
