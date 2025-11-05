package test

import (
	"fmt"
	"io"
	"os"

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

func (c *Catalog) FilterByName(names []string) *Catalog {
	if len(names) == 0 {
		return c
	}

	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	filtered := &Catalog{
		Tests: make([]TestDefinition, 0),
	}

	for _, test := range c.Tests {
		if nameSet[test.Name] {
			filtered.Tests = append(filtered.Tests, test)
		}
	}

	return filtered
}

func (c *Catalog) FilterByModule(modules []string) *Catalog {
	if len(modules) == 0 {
		return c
	}

	moduleSet := make(map[string]bool)
	for _, module := range modules {
		moduleSet[module] = true
	}

	filtered := &Catalog{
		Tests: make([]TestDefinition, 0),
	}

	for _, test := range c.Tests {
		if moduleSet[test.Module] {
			filtered.Tests = append(filtered.Tests, test)
		}
	}

	return filtered
}

func (c *Catalog) FilterByTags(tags []string) *Catalog {
	if len(tags) == 0 {
		return c
	}

	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}

	filtered := &Catalog{
		Tests: make([]TestDefinition, 0),
	}

	for _, test := range c.Tests {
		for _, testTag := range test.Tags {
			if tagSet[testTag] {
				filtered.Tests = append(filtered.Tests, test)
				break
			}
		}
	}

	return filtered
}