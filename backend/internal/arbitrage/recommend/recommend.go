// Package recommend evaluates funding arbitrage opportunities and flags
// which ones are worth opening. The logic is deliberately pluggable:
// callers supply a set of Rule functions, and an opportunity is recommended
// only when every rule passes.
package recommend

import (
	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

// Rule is a predicate that inspects a single arbitrage opportunity.
// Return true to signal that the opportunity satisfies this rule.
type Rule func(arb *domain.FundingArbitrage) bool

// Recommender holds an ordered list of rules. An opportunity is recommended
// only if all rules pass.
type Recommender struct {
	rules []Rule
}

// New creates a Recommender with the given rules. If no rules are supplied
// the recommender approves everything.
func New(rules ...Rule) *Recommender {
	return &Recommender{rules: rules}
}

// IsRecommended returns true when every rule passes for the given opportunity.
func (rec *Recommender) IsRecommended(arb *domain.FundingArbitrage) bool {
	for _, rule := range rec.rules {
		if !rule(arb) {
			return false
		}
	}
	return true
}

// Annotate sets the Recommended field on each arbitrage in the slice.
func (rec *Recommender) Annotate(arbs []domain.FundingArbitrage) {
	for i := range arbs {
		arbs[i].Recommended = rec.IsRecommended(&arbs[i])
	}
}

// ---------------------------------------------------------------------------
// Built-in rules
// ---------------------------------------------------------------------------

var zero = decimal.Zero

// ProfitableFunding requires the net 8h funding rate to be positive
// (i.e. the short leg pays more than the long leg costs).
func ProfitableFunding() Rule {
	return func(arb *domain.FundingArbitrage) bool {
		return arb.NetRate8h.GreaterThan(zero)
	}
}

// ProfitableSpread requires the open spread to be non-negative
// (i.e. entering the position does not immediately cost money via spread).
func ProfitableSpread() Rule {
	return func(arb *domain.FundingArbitrage) bool {
		return arb.OpenSpread.GreaterThanOrEqual(zero)
	}
}

// Default returns a Recommender with the standard rule set used for the
// funding arbitrage list endpoint.
func Default() *Recommender {
	return New(
		ProfitableFunding(),
		ProfitableSpread(),
	)
}
