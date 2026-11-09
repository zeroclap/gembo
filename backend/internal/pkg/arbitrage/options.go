package arbitrage

import "github.com/zeroclap/gembo/internal/domain"

// Option configures a Calculator or SpreadCalculator.
type Option func(*domain.QualityPolicy)

// WithQualityPolicy overrides the default quote validity/freshness thresholds.
func WithQualityPolicy(p domain.QualityPolicy) Option {
	return func(dst *domain.QualityPolicy) { *dst = p }
}

func applyOptions(opts []Option) domain.QualityPolicy {
	p := domain.DefaultQualityPolicy()
	for _, o := range opts {
		o(&p)
	}
	return p
}
