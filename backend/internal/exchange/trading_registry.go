package exchange

import "fmt"

// TradingRegistry holds authenticated exchange adapters for trade execution.
type TradingRegistry struct {
	exchanges map[string]TradingExchange
}

// NewTradingRegistry creates an empty trading registry.
func NewTradingRegistry() *TradingRegistry {
	return &TradingRegistry{exchanges: make(map[string]TradingExchange)}
}

// Register adds a trading exchange adapter to the registry.
// Panics if an exchange with the same ID is registered twice.
func (r *TradingRegistry) Register(e TradingExchange) {
	id := e.ID()
	if _, exists := r.exchanges[id]; exists {
		panic(fmt.Sprintf("trading exchange %q already registered", id))
	}
	r.exchanges[id] = e
}

// Get returns a registered trading exchange by ID, or an error if not found.
func (r *TradingRegistry) Get(id string) (TradingExchange, error) {
	e, ok := r.exchanges[id]
	if !ok {
		return nil, fmt.Errorf("trading exchange %q not found", id)
	}
	return e, nil
}

// All returns all registered trading exchanges.
func (r *TradingRegistry) All() []TradingExchange {
	out := make([]TradingExchange, 0, len(r.exchanges))
	for _, e := range r.exchanges {
		out = append(out, e)
	}
	return out
}

// IDs returns all registered trading exchange IDs.
func (r *TradingRegistry) IDs() []string {
	ids := make([]string, 0, len(r.exchanges))
	for id := range r.exchanges {
		ids = append(ids, id)
	}
	return ids
}
