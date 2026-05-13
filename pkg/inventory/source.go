package inventory

import (
	"context"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

// Source loads an Inventory from a configured backend. Implementations
// register themselves via RegisterSource in init() and are selected by
// the `kind:` field in an inventory YAML file. Credentials are NOT
// populated by Load — see Inventory.ApplyDefaults.
type Source interface {
	// Kind returns the registered name (for diagnostics).
	Kind() string
	// Load fetches devices and returns an Inventory. The caller's ctx is
	// honored for cancellation.
	Load(ctx context.Context) (*Inventory, error)
}

// Factory constructs a Source from the raw YAML node of an inventory
// file. The factory typically calls node.Decode(&cfg) into a config
// struct and returns a Source ready to Load.
type Factory func(node *yaml.Node) (Source, error)

// registry maps kind strings to factories. Exposed via the package-level
// RegisterSource and LoadSource functions; constructed as a value (not
// a pointer) so tests can build private registries.
type registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

func newRegistry() *registry {
	return &registry{factories: make(map[string]Factory)}
}

func (r *registry) register(kind string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[kind] = f
}

func (r *registry) factory(kind string) (Factory, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[kind]
	if !ok {
		return nil, fmt.Errorf("unknown inventory source kind %q", kind)
	}
	return f, nil
}

// global is the package-level registry. Each source's init() calls
// RegisterSource(kind, factory) to make itself available.
var global = newRegistry()

// RegisterSource adds a source factory to the global registry. Called
// from each Source implementation's init() function.
func RegisterSource(kind string, f Factory) {
	global.register(kind, f)
}
