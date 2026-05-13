package inventory

import (
	"context"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// fakeSource is a Source impl used to exercise the registry without
// touching network or filesystem.
type fakeSource struct {
	kind   string
	loaded *Inventory
	err    error
}

func (f *fakeSource) Kind() string                               { return f.kind }
func (f *fakeSource) Load(_ context.Context) (*Inventory, error) { return f.loaded, f.err }

func TestRegistry_RegisterAndGet(t *testing.T) {
	// Use a private registry to keep test isolated from real init() calls.
	r := newRegistry()

	r.register("fake", func(_ *yaml.Node) (Source, error) {
		return &fakeSource{kind: "fake"}, nil
	})

	factory, err := r.factory("fake")
	if err != nil {
		t.Fatalf("factory(fake): %v", err)
	}
	src, err := factory(nil)
	if err != nil {
		t.Fatalf("factory invoke: %v", err)
	}
	if src.Kind() != "fake" {
		t.Errorf("Kind: got %q, want fake", src.Kind())
	}
}

func TestRegistry_UnknownKindErrors(t *testing.T) {
	r := newRegistry()
	_, err := r.factory("nope")
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown-kind error, got %v", err)
	}
}

func TestRegistry_ConcurrentSafe(t *testing.T) {
	r := newRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.register("k", func(_ *yaml.Node) (Source, error) { return &fakeSource{kind: "k"}, nil })
			_, _ = r.factory("k")
		}(i)
	}
	wg.Wait()
}
