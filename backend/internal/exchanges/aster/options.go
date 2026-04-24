package aster

import "time"

// Option is a functional option for configuring an Aster Client.
type Option func(*Client)

// WithFundingPollInterval sets the interval at which funding rates are polled.
func WithFundingPollInterval(d time.Duration) Option {
	return func(c *Client) { c.fundingPollInterval = d }
}

// WithMarketsPollInterval sets the interval at which markets are polled.
func WithMarketsPollInterval(d time.Duration) Option {
	return func(c *Client) { c.marketsPollInterval = d }
}

// WithTickersPollInterval sets the interval at which tickers are polled via REST.
func WithTickersPollInterval(d time.Duration) Option {
	return func(c *Client) { c.tickersPollInterval = d }
}
