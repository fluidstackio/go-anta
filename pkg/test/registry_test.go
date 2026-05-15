package test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fluidstackio/go-anta/pkg/device"
)

// fakeRegTest is a yaml-tagged test used to exercise the registry's
// typo-detection path end-to-end.
type fakeRegTest struct {
	BaseTest
	Hosts []string `yaml:"hosts"`
}

func (t *fakeRegTest) Execute(_ context.Context, _ device.Device) (*TestResult, error) {
	return &TestResult{Status: TestSuccess}, nil
}
func (t *fakeRegTest) ValidateInput(_ any) error { return nil }

func newFakeRegTest(inputs map[string]any) (Test, error) {
	t := &fakeRegTest{}
	if hosts, ok := inputs["hosts"].([]any); ok {
		for _, h := range hosts {
			if s, ok := h.(string); ok {
				t.Hosts = append(t.Hosts, s)
			}
		}
	}
	return t, nil
}

// TestRegistry_GetTestWithInputs_TypoRejected verifies that a typo in
// a top-level input key is rejected at GetTestWithInputs time. Before
// this check, the typo was silently dropped and the user saw a
// downstream "no hosts found" failure that didn't point at the cause.
func TestRegistry_GetTestWithInputs_TypoRejected(t *testing.T) {
	// Use a fresh registry to avoid colliding with the global one.
	r := &Registry{tests: map[string]map[string]TestFactory{}}
	if err := r.Register("fake", "FakeTest", newFakeRegTest); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := r.GetTestWithInputs("fake", "FakeTest", map[string]any{"hostz": []any{"a"}})
	if err == nil {
		t.Fatal("expected typo to be rejected, got nil")
	}
	if !strings.Contains(err.Error(), "fake/FakeTest") {
		t.Errorf("error should identify module/test, got: %v", err)
	}
	if !strings.Contains(err.Error(), "hostz") {
		t.Errorf("error should name the typo, got: %v", err)
	}
}

func TestRegistry_GetTestWithInputs_ValidPasses(t *testing.T) {
	r := &Registry{tests: map[string]map[string]TestFactory{}}
	if err := r.Register("fake", "FakeTest", newFakeRegTest); err != nil {
		t.Fatalf("register: %v", err)
	}

	got, err := r.GetTestWithInputs("fake", "FakeTest", map[string]any{"hosts": []any{"1.1.1.1"}})
	if err != nil {
		t.Fatalf("valid inputs rejected: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil test")
	}
}

func TestRegistry_GetTestWithInputs_FactoryErrorPreserved(t *testing.T) {
	// If the factory itself returns an error (e.g. malformed nested
	// shape) we should pass it through, not wrap or replace with the
	// key-validation message.
	r := &Registry{tests: map[string]map[string]TestFactory{}}
	sentinel := errors.New("boom from factory")
	if err := r.Register("fake", "Boom", func(_ map[string]any) (Test, error) {
		return nil, sentinel
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	_, err := r.GetTestWithInputs("fake", "Boom", map[string]any{"hosts": []any{}})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected factory error to be preserved, got: %v", err)
	}
}
