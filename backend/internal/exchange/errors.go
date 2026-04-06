package exchange

import "errors"

// ErrPositionAlreadyClosed is returned by CreateMarketOrder when a reduce-only
// order is rejected because the position is already at zero on the exchange.
// Each exchange adapter detects the exchange-specific error variant and wraps
// it with this sentinel so that callers (e.g. the trader reactor) can handle
// the condition without importing CCXT directly.
var ErrPositionAlreadyClosed = errors.New("position already closed on exchange")
