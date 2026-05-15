package test

import (
	"context"
	"strings"
	"testing"

	"github.com/fluidstackio/go-anta/pkg/device"
)

type fakeCatTest struct{ BaseTest }

func (f *fakeCatTest) Execute(_ context.Context, _ device.Device) (*TestResult, error) {
	return nil, nil
}
func (f *fakeCatTest) ValidateInput(_ any) error { return nil }

func newFakeCatTest(_ map[string]any) (Test, error) { return &fakeCatTest{}, nil }

func newCatalogTestRegistry(t *testing.T) *Registry {
	t.Helper()
	r := &Registry{tests: map[string]map[string]TestFactory{}}
	if err := r.Register("routing", "VerifyBGPPeers", newFakeCatTest); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := r.Register("hardware", "VerifyTemperature", newFakeCatTest); err != nil {
		t.Fatalf("register: %v", err)
	}
	return r
}

func TestCatalog_ValidateAgainst_HappyPath(t *testing.T) {
	r := newCatalogTestRegistry(t)
	c := &Catalog{Tests: []TestDefinition{
		{Name: "VerifyBGPPeers", Module: "routing"},
		{Name: "VerifyTemperature", Module: "hardware"},
	}}
	if err := c.ValidateAgainst(r); err != nil {
		t.Errorf("expected nil for known tests, got %v", err)
	}
}

func TestCatalog_ValidateAgainst_TyposCalledOutOnce(t *testing.T) {
	r := newCatalogTestRegistry(t)
	c := &Catalog{Tests: []TestDefinition{
		{Name: "VerifyBGPpeers", Module: "routing"},    // lowercase typo
		{Name: "VerifyTemperature", Module: "hardwre"}, // module typo
		{Name: "VerifyBGPPeers", Module: "routing"},    // valid
	}}
	err := c.ValidateAgainst(r)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "VerifyBGPpeers") {
		t.Errorf("error should name the lowercase typo, got: %v", err)
	}
	if !strings.Contains(msg, "unknown module") {
		t.Errorf("error should call out the unknown module, got: %v", err)
	}
}

func TestCatalog_FilterByName_ReturnsMissing(t *testing.T) {
	c := &Catalog{Tests: []TestDefinition{
		{Name: "A", Module: "x"},
		{Name: "B", Module: "x"},
	}}

	got, err := c.FilterByName([]string{"A", "C"})
	if err == nil {
		t.Fatal("expected error for missing name 'C'")
	}
	if !strings.Contains(err.Error(), "C") {
		t.Errorf("error should name 'C', got: %v", err)
	}
	if len(got.Tests) != 1 || got.Tests[0].Name != "A" {
		t.Errorf("expected single-match for 'A', got %+v", got.Tests)
	}
}

func TestCatalog_FilterByName_AllMatch(t *testing.T) {
	c := &Catalog{Tests: []TestDefinition{
		{Name: "A", Module: "x"},
		{Name: "B", Module: "x"},
	}}
	got, err := c.FilterByName([]string{"A", "B"})
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if len(got.Tests) != 2 {
		t.Errorf("expected 2 tests, got %d", len(got.Tests))
	}
}

func TestCatalog_FilterByName_EmptyInputPassesThrough(t *testing.T) {
	c := &Catalog{Tests: []TestDefinition{{Name: "A", Module: "x"}}}
	got, err := c.FilterByName(nil)
	if err != nil || got != c {
		t.Errorf("empty filter should return self with no error; got %v, %v", got, err)
	}
}
