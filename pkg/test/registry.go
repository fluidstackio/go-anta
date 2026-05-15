package test

import (
	"fmt"
	"sync"
)

type TestFactory func(inputs map[string]interface{}) (Test, error)

type Registry struct {
	mu    sync.RWMutex
	tests map[string]map[string]TestFactory
}

var (
	globalRegistry *Registry
	once           sync.Once
)

func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			tests: make(map[string]map[string]TestFactory),
		}
	})
	return globalRegistry
}

func (r *Registry) Register(module, name string, factory TestFactory) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if module == "" || name == "" {
		return fmt.Errorf("module and name cannot be empty")
	}

	if r.tests[module] == nil {
		r.tests[module] = make(map[string]TestFactory)
	}

	if _, exists := r.tests[module][name]; exists {
		return fmt.Errorf("test %s/%s already registered", module, name)
	}

	r.tests[module][name] = factory
	return nil
}

func (r *Registry) GetTestWithInputs(module, name string, inputs map[string]interface{}) (Test, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	moduleTests, exists := r.tests[module]
	if !exists {
		return nil, fmt.Errorf("module %s not found", module)
	}

	factory, exists := moduleTests[name]
	if !exists {
		return nil, fmt.Errorf("test %s not found in module %s", name, module)
	}

	t, err := factory(inputs)
	if err != nil {
		return nil, err
	}
	// Catch top-level input typos centrally — the constructor silently
	// drops unknown keys, so without this check a typo like
	// `peer_addres:` shows up much later as a misleading "no peers
	// found" failure.
	if err := ValidateInputKeys(inputs, t); err != nil {
		return nil, fmt.Errorf("%s/%s: %w", module, name, err)
	}
	return t, nil
}

