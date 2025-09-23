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

func (r *Registry) GetTest(module, name string) (Test, error) {
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

	return factory(nil)
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

	return factory(inputs)
}

func (r *Registry) ListModules() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modules := make([]string, 0, len(r.tests))
	for module := range r.tests {
		modules = append(modules, module)
	}
	return modules
}

func (r *Registry) ListTests(module string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	moduleTests, exists := r.tests[module]
	if !exists {
		return nil
	}

	tests := make([]string, 0, len(moduleTests))
	for test := range moduleTests {
		tests = append(tests, test)
	}
	return tests
}

func (r *Registry) ListAllTests() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	allTests := make(map[string][]string)
	for module, moduleTests := range r.tests {
		tests := make([]string, 0, len(moduleTests))
		for test := range moduleTests {
			tests = append(tests, test)
		}
		allTests[module] = tests
	}
	return allTests
}

func RegisterTest(module, name string, factory TestFactory) error {
	return GetRegistry().Register(module, name, factory)
}