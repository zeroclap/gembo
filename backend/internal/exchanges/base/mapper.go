package base

import (
	ccxt "github.com/ccxt/ccxt/go/v4"

	"github.com/zeroclap/gembo/internal/exchanges/base/ccxtutil"
)

// MapPosition converts a CCXT Position to a base.ExchangePosition.
func MapPosition(p ccxt.Position) *ExchangePosition {
	return &ExchangePosition{
		Symbol:           ccxtutil.DerefString(p.Symbol),
		Side:             ccxtutil.NormalizePositionSide(ccxtutil.DerefString(p.Side)),
		Contracts:        ccxtutil.FloatToDecimal(p.Contracts),
		EntryPrice:       ccxtutil.FloatToDecimal(p.EntryPrice),
		MarkPrice:        ccxtutil.FloatToDecimal(p.MarkPrice),
		LiquidationPrice: ccxtutil.FloatToDecimal(p.LiquidationPrice),
		UnrealizedPnL:    ccxtutil.FloatToDecimal(p.UnrealizedPnl),
		Leverage:         ccxtutil.FloatToDecimal(p.Leverage),
		MarginMode:       ccxtutil.DerefString(p.MarginMode),
		Notional:         ccxtutil.FloatToDecimal(p.Notional),
	}
}

// MapBalances converts CCXT Balances to a slice of base.Balance.
func MapBalances(b ccxt.Balances) []Balance {
	var out []Balance
	for currency, bal := range b.Balances {
		out = append(out, Balance{
			Currency: currency,
			Free:     ccxtutil.FloatToDecimal(bal.Free),
			Used:     ccxtutil.FloatToDecimal(bal.Used),
			Total:    ccxtutil.FloatToDecimal(bal.Total),
		})
	}
	return out
}
