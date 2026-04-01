package bybit

import "time"

// Option is a functional option for configuring a Bybit Client.
type Option func(*Client)

// WithFundingPollInterval sets the interval at which funding rates are polled.
func WithFundingPollInterval(d time.Duration) Option {
	return func(c *Client) { c.fundingPollInterval = d }
}

// WithMarketsPollInterval sets the interval at which markets are polled.
func WithMarketsPollInterval(d time.Duration) Option {
	return func(c *Client) { c.marketsPollInterval = d }
}

// WithAPIKey sets the API key for authenticated requests.
func WithAPIKey(key string) Option {
	return func(c *Client) { c.apiKey = key }
}

// WithSecret sets the API secret for authenticated requests.
func WithSecret(secret string) Option {
	return func(c *Client) { c.secret = secret }
}
