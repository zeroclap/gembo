package positionmanager

import "context"

// runFundingTracker is disabled: funding events are ingested from Redis
// channels published by the private collector.
func (t *Trader) runFundingTracker(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
