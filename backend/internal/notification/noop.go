package notification

import "context"

// NoopChannel silently discards all messages. Useful as a fallback when
// a channel is misconfigured or notifications are disabled.
type NoopChannel struct{}

// Send discards the message and returns nil.
func (n *NoopChannel) Send(_ context.Context, _ Message) error {
	return nil
}
