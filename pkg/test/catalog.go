package test

import (
	"fmt"
	"io"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

type Catalog struct {
	Tests []TestDefinition `yaml:"tests" json:"tests"`
}

func LoadCatalog(path string) (*Catalog, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open catalog file: %w", err)
	}
	defer file.Close()

	return ParseCatalog(file)
}

func ParseCatalog(r io.Reader) (*Catalog, error) {
	var catalog Catalog
	decoder := yaml.NewDecoder(r)
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("failed to parse catalog: %w", err)
	}

	if err := catalog.Validate(); err != nil {
		return nil, fmt.Errorf("catalog validation failed: %w", err)
	}

	return &catalog, nil
}

func (c *Catalog) Validate() error {
	if len(c.Tests) == 0 {
		return fmt.Errorf("catalog must contain at least one test")
	}

	testNames := make(map[string]bool)
	for i, test := range c.Tests {
		if test.Name == "" {
			return fmt.Errorf("test at index %d has no name", i)
		}
		if test.Module == "" {
			return fmt.Errorf("test '%s' has no module specified", test.Name)
		}
		if testNames[test.Name] {
			return fmt.Errorf("duplicate test name: %s", test.Name)
		}
		testNames[test.Name] = true
	}

	return nil
}

// ValidateAgainst checks that every (Module, Name) in the catalog is
// registered. Previously a typo like `VerifyBGPpeers` (lowercase 'p')
// surfaced as a `TestError: "Test not found"` per device — 50 duplicate
// errors instead of one upfront failure. Catch them at parse time so
// the user sees a single, clearly attributed error.
func (c *Catalog) ValidateAgainst(reg *Registry) error {
	if reg == nil {
		return fmt.Errorf("registry is nil")
	}
	reg.mu.RLock()
	defer reg.mu.RUnlock()

	var unknown []string
	for _, test := range c.Tests {
		modTests, ok := reg.tests[test.Module]
		if !ok {
			unknown = append(unknown, fmt.Sprintf("%s/%s (unknown module)", test.Module, test.Name))
			continue
		}
		if _, ok := modTests[test.Name]; !ok {
			unknown = append(unknown, fmt.Sprintf("%s/%s", test.Module, test.Name))
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("catalog references unknown test(s): %v", unknown)
}

// FilterByName keeps tests whose name appears in `names`. The returned
// error names any requested names that didn't match anything — the
// caller can log/exit/ignore as desired. The filtered catalog is still
// returned alongside the error so best-effort callers can proceed.
// Same shape for FilterByModule and FilterByTags below.
func (c *Catalog) FilterByName(names []string) (*Catalog, error) {
	if len(names) == 0 {
		return c, nil
	}

	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[name] = false
	}

	filtered := &Catalog{Tests: make([]TestDefinition, 0)}
	for _, test := range c.Tests {
		if _, ok := wanted[test.Name]; ok {
			wanted[test.Name] = true
			filtered.Tests = append(filtered.Tests, test)
		}
	}
	return filtered, missingErr("test name(s)", wanted)
}

func (c *Catalog) FilterByModule(modules []string) (*Catalog, error) {
	if len(modules) == 0 {
		return c, nil
	}

	wanted := make(map[string]bool, len(modules))
	for _, module := range modules {
		wanted[module] = false
	}

	filtered := &Catalog{Tests: make([]TestDefinition, 0)}
	for _, test := range c.Tests {
		if _, ok := wanted[test.Module]; ok {
			wanted[test.Module] = true
			filtered.Tests = append(filtered.Tests, test)
		}
	}
	return filtered, missingErr("module(s)", wanted)
}

func (c *Catalog) FilterByTags(tags []string) (*Catalog, error) {
	if len(tags) == 0 {
		return c, nil
	}

	wanted := make(map[string]bool, len(tags))
	for _, tag := range tags {
		wanted[tag] = false
	}

	filtered := &Catalog{Tests: make([]TestDefinition, 0)}
	for _, test := range c.Tests {
		appended := false
		for _, testTag := range test.Tags {
			if _, ok := wanted[testTag]; ok {
				wanted[testTag] = true
				if !appended {
					filtered.Tests = append(filtered.Tests, test)
					appended = true
				}
			}
		}
	}
	return filtered, missingErr("tag(s)", wanted)
}

// missingErr returns nil when every key was matched, or a sorted error
// listing the keys that found no rows. Shared by all three Catalog
// filters and (separately defined) by the Inventory filters.
func missingErr(label string, seen map[string]bool) error {
	var missing []string
	for k, found := range seen {
		if !found {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("no matches for %s: %v", label, missing)
}
