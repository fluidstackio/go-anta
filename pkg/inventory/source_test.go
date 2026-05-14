package inventory

import (
	"context"
	"os"
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

func TestLoadSource_RoutesByKind(t *testing.T) {
	tmp := writeYAML(t, `
kind: fake
greeting: hello
`)

	r := newRegistry()
	r.register("fake", func(node *yaml.Node) (Source, error) {
		var cfg struct {
			Kind     string `yaml:"kind"`
			Greeting string `yaml:"greeting"`
		}
		if err := node.Decode(&cfg); err != nil {
			return nil, err
		}
		if cfg.Greeting != "hello" {
			t.Errorf("greeting passed through: got %q", cfg.Greeting)
		}
		return &fakeSource{kind: "fake"}, nil
	})

	src, err := loadSourceWithRegistry(tmp, "", r)
	if err != nil {
		t.Fatalf("loadSourceWithRegistry: %v", err)
	}
	if src.Kind() != "fake" {
		t.Errorf("Kind: got %q want fake", src.Kind())
	}
}

func TestLoadSource_NoKindDefaultsToFile(t *testing.T) {
	tmp := writeYAML(t, `
devices:
  - name: x
    host: 1.2.3.4
    username: admin
    password: pw
`)

	r := newRegistry()
	called := false
	r.register("file", func(node *yaml.Node) (Source, error) {
		called = true
		return &fakeSource{kind: "file"}, nil
	})

	if _, err := loadSourceWithRegistry(tmp, "", r); err != nil {
		t.Fatalf("loadSourceWithRegistry: %v", err)
	}
	if !called {
		t.Error("expected file factory to be called")
	}
}

func TestLoadSource_LegacyNetboxBlock(t *testing.T) {
	tmp := writeYAML(t, `
netbox:
  url: https://netbox.example.com
  token: secret
`)

	r := newRegistry()
	called := false
	r.register("netbox", func(node *yaml.Node) (Source, error) {
		called = true
		return &fakeSource{kind: "netbox"}, nil
	})

	if _, err := loadSourceWithRegistry(tmp, "", r); err != nil {
		t.Fatalf("loadSourceWithRegistry: %v", err)
	}
	if !called {
		t.Error("expected netbox factory to be called for legacy block")
	}
}

func TestLoadSource_OverrideKindWins(t *testing.T) {
	// YAML says kind: file but the override forces netbox.
	tmp := writeYAML(t, `
kind: file
devices:
  - name: x
    host: 1.2.3.4
`)
	r := newRegistry()
	called := ""
	r.register("file", func(node *yaml.Node) (Source, error) {
		called = "file"
		return &fakeSource{kind: "file"}, nil
	})
	r.register("netbox", func(node *yaml.Node) (Source, error) {
		called = "netbox"
		return &fakeSource{kind: "netbox"}, nil
	})

	if _, err := loadSourceWithRegistry(tmp, "netbox", r); err != nil {
		t.Fatalf("loadSourceWithRegistry: %v", err)
	}
	if called != "netbox" {
		t.Errorf("expected netbox factory called, got %q", called)
	}
}

func TestLoadSource_FileNotFound(t *testing.T) {
	_, err := loadSourceWithRegistry("/does/not/exist.yaml", "", newRegistry())
	if err == nil {
		t.Error("expected error for missing file")
	}
}

// writeYAML helper: dump content to a temp file and return the path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/inv.yaml"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeYAML: %v", err)
	}
	return path
}
