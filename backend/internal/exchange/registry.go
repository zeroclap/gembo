package exchange

import "fmt"

// Registry holds all registered exchange adapters.
type Registry struct {
	exchanges map[string]Exchange
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{exchanges: make(map[string]Exchange)}
}

// Register adds an exchange adapter to the registry.
// Panics if an exchange with the same ID is registered twice.
func (r *Registry) Register(e Exchange) {
	id := e.ID()
	if _, exists := r.exchanges[id]; exists {
		panic(fmt.Sprintf("exchange %q already registered", id))
	}
	r.exchanges[id] = e
}

// Get returns a registered exchange by ID, or an error if not found.
func (r *Registry) Get(id string) (Exchange, error) {
	e, ok := r.exchanges[id]
	if !ok {
		return nil, fmt.Errorf("exchange %q not found", id)
	}
	return e, nil
}

// All returns all registered exchanges in arbitrary order.
func (r *Registry) All() []Exchange {
	out := make([]Exchange, 0, len(r.exchanges))
	for _, e := range r.exchanges {
		out = append(out, e)
	}
	return out
}

// IDs returns all registered exchange IDs.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.exchanges))
	for id := range r.exchanges {
		ids = append(ids, id)
	}
	return ids
}
