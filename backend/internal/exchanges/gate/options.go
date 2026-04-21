package gate

import "time"

// Option is a functional option for configuring a Gate.io Client.
type Option func(*Client)

// WithFundingPollInterval sets the interval at which funding rates are polled.
func WithFundingPollInterval(d time.Duration) Option {
	return func(c *Client) { c.fundingPollInterval = d }
}

// WithMarketsPollInterval sets the interval at which markets are polled.
func WithMarketsPollInterval(d time.Duration) Option {
	return func(c *Client) { c.marketsPollInterval = d }
}
