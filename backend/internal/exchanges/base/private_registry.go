package base

import "fmt"

// PrivateExchangeRegistry holds authenticated exchange adapters that implement
// the full PrivateExchange interface.
type PrivateExchangeRegistry struct {
	exchanges map[string]PrivateExchange
}

// NewPrivateExchangeRegistry creates an empty private exchange registry.
func NewPrivateExchangeRegistry() *PrivateExchangeRegistry {
	return &PrivateExchangeRegistry{exchanges: make(map[string]PrivateExchange)}
}

// Register adds a private exchange adapter to the registry.
// Panics if an exchange with the same ID is registered twice.
func (r *PrivateExchangeRegistry) Register(e PrivateExchange) {
	id := e.ID()
	if _, exists := r.exchanges[id]; exists {
		panic(fmt.Sprintf("private exchange %q already registered", id))
	}
	r.exchanges[id] = e
}

// Get returns a registered private exchange by ID, or an error if not found.
func (r *PrivateExchangeRegistry) Get(id string) (PrivateExchange, error) {
	e, ok := r.exchanges[id]
	if !ok {
		return nil, fmt.Errorf("private exchange %q not found", id)
	}
	return e, nil
}

// All returns all registered private exchanges in arbitrary order.
func (r *PrivateExchangeRegistry) All() []PrivateExchange {
	out := make([]PrivateExchange, 0, len(r.exchanges))
	for _, e := range r.exchanges {
		out = append(out, e)
	}
	return out
}

// IDs returns all registered private exchange IDs.
func (r *PrivateExchangeRegistry) IDs() []string {
	ids := make([]string, 0, len(r.exchanges))
	for id := range r.exchanges {
		ids = append(ids, id)
	}
	return ids
}
