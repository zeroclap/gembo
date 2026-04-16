package recommend

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

func arb(netRate8h, openSpread float64) domain.FundingArbitrage {
	return domain.FundingArbitrage{
		Symbol:     "BTC/USDT:USDT",
		NetRate8h:  decimal.NewFromFloat(netRate8h),
		OpenSpread: decimal.NewFromFloat(openSpread),
	}
}

func TestDefault_BothPositive(t *testing.T) {
	rec := Default()
	a := arb(0.01, 0.05)
	if !rec.IsRecommended(&a) {
		t.Fatal("expected recommended when both funding and spread are positive")
	}
}

func TestDefault_NegativeFunding(t *testing.T) {
	rec := Default()
	a := arb(-0.01, 0.05)
	if rec.IsRecommended(&a) {
		t.Fatal("expected not recommended when funding is negative")
	}
}

func TestDefault_NegativeSpread(t *testing.T) {
	rec := Default()
	a := arb(0.01, -0.02)
	if rec.IsRecommended(&a) {
		t.Fatal("expected not recommended when spread is negative")
	}
}

func TestDefault_ZeroValues(t *testing.T) {
	rec := Default()
	// Zero funding → not profitable
	a := arb(0, 0.05)
	if rec.IsRecommended(&a) {
		t.Fatal("expected not recommended when funding is zero")
	}
	// Positive funding, zero spread → OK (spread is non-negative)
	b := arb(0.01, 0)
	if !rec.IsRecommended(&b) {
		t.Fatal("expected recommended when funding positive and spread is zero")
	}
}

func TestAnnotate(t *testing.T) {
	rec := Default()
	arbs := []domain.FundingArbitrage{
		arb(0.01, 0.05),  // recommended
		arb(-0.01, 0.05), // not
		arb(0.01, -0.02), // not
		arb(0.02, 0.10),  // recommended
	}
	rec.Annotate(arbs)

	want := []bool{true, false, false, true}
	for i, w := range want {
		if arbs[i].Recommended != w {
			t.Errorf("arbs[%d].Recommended = %v, want %v", i, arbs[i].Recommended, w)
		}
	}
}

func TestNoRules_AlwaysRecommended(t *testing.T) {
	rec := New()
	a := arb(-999, -999)
	if !rec.IsRecommended(&a) {
		t.Fatal("recommender with no rules should approve everything")
	}
}

func TestCustomRule(t *testing.T) {
	minAPR := decimal.NewFromFloat(10)
	rec := New(func(a *domain.FundingArbitrage) bool {
		return a.APR.GreaterThan(minAPR)
	})

	low := domain.FundingArbitrage{APR: decimal.NewFromFloat(5)}
	high := domain.FundingArbitrage{APR: decimal.NewFromFloat(15)}

	if rec.IsRecommended(&low) {
		t.Fatal("expected low APR to not be recommended")
	}
	if !rec.IsRecommended(&high) {
		t.Fatal("expected high APR to be recommended")
	}
}
