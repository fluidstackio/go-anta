package inventory

import (
	"context"
	"fmt"
	"os"
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
// RegisterSource and LoadSource functions; constructed as a pointer so
// registers and lookups share one map.
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

// LoadSource reads a YAML inventory file, picks the source kind, and
// returns a constructed Source. Callers then invoke src.Load(ctx) to
// fetch the actual Inventory.
//
// Kind detection:
//   - Explicit `kind:` field wins.
//   - A legacy top-level `netbox:` block (the pre-abstraction format)
//     is treated as kind: netbox.
//   - Otherwise the file is assumed to be a static device list (kind: file).
func LoadSource(path string) (Source, error) {
	return loadSourceWithRegistry(path, "", global)
}

// LoadSourceAs is like LoadSource but routes the YAML body to the
// factory for the explicit kindOverride argument instead of using the
// YAML's `kind:` field. The rest of the YAML body is passed to the
// override kind's factory unchanged; missing required fields produce a
// clear error from that factory. Used by CLI --source flag.
func LoadSourceAs(path, kindOverride string) (Source, error) {
	return loadSourceWithRegistry(path, kindOverride, global)
}

func loadSourceWithRegistry(path, kindOverride string, r *registry) (Source, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read inventory file: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("parse inventory YAML: %w", err)
	}
	// yaml.Unmarshal wraps the top-level node in a DocumentNode; the
	// MappingNode we care about is its first content child.
	if len(node.Content) == 0 {
		return nil, fmt.Errorf("inventory YAML is empty")
	}
	doc := node.Content[0]

	kind := kindOverride
	if kind == "" {
		kind = detectKind(doc)
	}
	factory, err := r.factory(kind)
	if err != nil {
		return nil, fmt.Errorf("inventory file %s: %w", path, err)
	}
	return factory(doc)
}

// detectKind looks at the YAML document's top-level keys to decide
// which source factory should handle the file.
func detectKind(doc *yaml.Node) string {
	var explicit string
	var hasNetbox bool
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key := doc.Content[i].Value
		switch key {
		case "kind":
			explicit = doc.Content[i+1].Value
		case "netbox":
			hasNetbox = true
		}
	}
	if explicit != "" {
		return explicit
	}
	if hasNetbox {
		return "netbox"
	}
	return "file"
}
