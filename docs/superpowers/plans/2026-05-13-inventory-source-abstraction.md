# Inventory Source Abstraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace go-anta's two hard-coded inventory paths (static YAML and Netbox) with a small `Source` interface that lets new backends be added by dropping a single Go file. First new source is **dcfab**, FluidStack's datacenter-fabric GraphQL API.

**Architecture:** A new `pkg/inventory.Source` interface with a global registry. Each implementation lives in its own `source_<kind>.go` file and registers itself in `init()`. A single `LoadSource(path) (Source, error)` entry point parses YAML, dispatches on the `kind:` field, and returns a constructed source. CLI commands collapse to one shared helper that calls `LoadSource → src.Load(ctx) → ApplyDefaults(...)`.

**Tech Stack:** Go, `gopkg.in/yaml.v3` (already used), `net/http` (for dcfab GraphQL GET), no new direct dependencies.

**Reference design:** `docs/superpowers/specs/2026-05-12-inventory-source-abstraction-design.md`

---

## File Structure

| Path | Action | Responsibility |
|---|---|---|
| `pkg/inventory/source.go` | Create | `Source` interface, `Factory` type, registry, `LoadSource` entry point |
| `pkg/inventory/source_test.go` | Create | Registry + LoadSource tests using a fake `Source` |
| `pkg/inventory/source_file.go` | Create | `FileSource` (static-YAML loader, the existing logic refactored) |
| `pkg/inventory/source_file_test.go` | Create | Loads sample static YAML fixtures |
| `pkg/inventory/netbox_client.go` | Rename from `netbox.go` | Low-level NetboxClient, types, GetDevices |
| `pkg/inventory/source_netbox.go` | Create | `NetboxSource` + factory + back-compat wrappers (`LoadFromNetbox`, `LoadNetboxInventory`) |
| `pkg/inventory/source_netbox_test.go` | Create | Factory decodes YAML; mocks the HTTP client |
| `pkg/inventory/source_dcfab.go` | Create | `DcfabSource` + GraphQL query builder + response parser + factory |
| `pkg/inventory/source_dcfab_test.go` | Create | Query construction (URL-encoding), device→DeviceConfig mapping, mock HTTP |
| `pkg/inventory/inventory.go` | Modify | `LoadInventory` becomes thin wrapper around `LoadSource`; add `DeviceDefaults` + `ApplyDefaults` |
| `pkg/inventory/inventory_test.go` | Create | `ApplyDefaults` overlay precedence tests |
| `internal/cli/commands/inventoryload.go` | Create | Shared CLI helper: parses common flags, calls `LoadSource`, applies `ApplyDefaults` |
| `internal/cli/commands/nrfu.go` | Modify | Use shared helper; add `--source`, `--region`, `--roles` flags |
| `internal/cli/commands/check.go` | Modify | Use shared helper; same flags |
| `internal/cli/commands/inventory.go` | Modify | Use shared helper; same flags |
| `examples/wdl101-dcfab.yaml` | Create | Sample dcfab inventory for the lab device |

### Why these boundaries

- **`source.go` is interface-only.** The contract for new sources, kept small so any future contributor reads ~50 lines to understand the extension point.
- **One file per source impl.** Each `source_<kind>.go` is self-contained: factory, config struct, `Load()`. Adding `source_consul.go` later requires zero edits to existing files.
- **Netbox split into client + source.** The HTTP/JSON client stays as-is (well-tested, no reason to disturb); the new `NetboxSource` is a thin Source-interface wrapper around it.
- **CLI helper extracted.** Today three commands copy-paste the same ~110 lines of Netbox query parsing. Pulling it into one file fixes `check.go`'s silent missing-keys bug from the prior review (deferred item R5) for free.

---

## Task 1: `Source` interface and global Registry

**Files:**
- Create: `pkg/inventory/source.go`
- Create: `pkg/inventory/source_test.go`

- [ ] **Step 1: Write failing tests in `pkg/inventory/source_test.go`**

```go
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

func (f *fakeSource) Kind() string                          { return f.kind }
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
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/...
```

Expected: build error (`undefined: newRegistry`, `undefined: Source`).

- [ ] **Step 3: Implement `source.go`**

```go
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
// RegisterSource(global, kind, factory) to make itself available.
var global = newRegistry()

// RegisterSource adds a source factory to the global registry. Called
// from each Source implementation's init() function.
func RegisterSource(kind string, f Factory) {
	global.register(kind, f)
}
```

- [ ] **Step 4: Run tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestRegistry -v
```

Expected: all 3 subtests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/source.go pkg/inventory/source_test.go && git commit -m "$(cat <<'EOF'
feat(inventory): add Source interface and registry

Defines the extension point for pluggable inventory backends: a small
Source interface (Kind + Load) and a thread-safe global registry that
maps kind strings to factories. Each backend impl will register itself
in init() and be selected at runtime by an inventory YAML's `kind:` field.

No backends registered yet; subsequent tasks add FileSource,
NetboxSource, and DcfabSource.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: `LoadSource` entry point and YAML dispatch

**Files:**
- Modify: `pkg/inventory/source.go`
- Modify: `pkg/inventory/source_test.go`

`LoadSource` reads a YAML file, peeks at the `kind:` field, and dispatches to the registered factory. Backward compat: a YAML with no `kind:` field is treated as `kind: file`; a YAML with a legacy top-level `netbox:` block is treated as `kind: netbox` (and re-parsed appropriately).

- [ ] **Step 1: Add tests to `source_test.go`**

```go
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

	src, err := loadSourceWithRegistry(tmp, r)
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

	if _, err := loadSourceWithRegistry(tmp, r); err != nil {
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

	if _, err := loadSourceWithRegistry(tmp, r); err != nil {
		t.Fatalf("loadSourceWithRegistry: %v", err)
	}
	if !called {
		t.Error("expected netbox factory to be called for legacy block")
	}
}

func TestLoadSource_FileNotFound(t *testing.T) {
	_, err := loadSourceWithRegistry("/does/not/exist.yaml", newRegistry())
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
```

Add `"os"` to the test file's imports.

- [ ] **Step 2: Run tests to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestLoadSource
```

Expected: build error (`undefined: loadSourceWithRegistry`).

- [ ] **Step 3: Implement `LoadSource` and `LoadSourceAs` in `source.go`**

Append to `pkg/inventory/source.go`:

```go
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
```

Add `"os"` to `source.go`'s imports.

- [ ] **Step 4: Run tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestLoadSource -v
```

Expected: all 4 subtests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/source.go pkg/inventory/source_test.go && git commit -m "$(cat <<'EOF'
feat(inventory): add LoadSource YAML dispatcher

Reads the inventory YAML, detects the source kind, and returns the
constructed Source. Backward compat: files without a `kind:` field
default to kind: file; files with a legacy top-level `netbox:` block
route to kind: netbox.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: `FileSource` — refactor static-YAML loader

**Files:**
- Create: `pkg/inventory/source_file.go`
- Create: `pkg/inventory/source_file_test.go`
- Modify: `pkg/inventory/inventory.go` (turn `LoadInventory` into a thin wrapper around `LoadSource`)

- [ ] **Step 1: Write failing tests in `source_file_test.go`**

```go
package inventory

import (
	"context"
	"os"
	"testing"
)

func TestFileSource_LoadsDevices(t *testing.T) {
	tmp := writeYAML(t, `
kind: file
devices:
  - name: r1
    host: 192.0.2.10
    username: admin
    password: pw
  - name: r2
    host: 192.0.2.11
    username: admin
    password: pw
`)

	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	if src.Kind() != "file" {
		t.Fatalf("Kind: got %q want file", src.Kind())
	}

	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(inv.Devices), 2; got != want {
		t.Errorf("device count: got %d want %d", got, want)
	}
	if inv.Devices[0].Name != "r1" {
		t.Errorf("first device name: got %q want r1", inv.Devices[0].Name)
	}
}

func TestFileSource_NoKindBackwardCompat(t *testing.T) {
	tmp := writeYAML(t, `
devices:
  - name: r1
    host: 192.0.2.10
    username: admin
    password: pw
`)

	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "r1" {
		t.Errorf("expected one device r1; got %v", inv.Devices)
	}
}

func TestFileSource_NetworksExpand(t *testing.T) {
	tmp := writeYAML(t, `
kind: file
networks:
  - network: 192.0.2.0/30
    username: admin
    password: pw
`)
	src, _ := LoadSource(tmp)
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// /30 has 2 host IPs (excluding network + broadcast).
	if len(inv.Devices) != 2 {
		t.Errorf("/30 should expand to 2 host devices, got %d", len(inv.Devices))
	}
	// Verify temp file removal works (sanity check on t.TempDir).
	if _, err := os.Stat(tmp); err != nil {
		t.Logf("temp file already gone (expected): %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestFileSource
```

Expected: tests fail because no factory is registered for `kind: file` yet.

- [ ] **Step 3: Create `source_file.go`**

```go
package inventory

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// FileSource produces an Inventory from a static YAML file: an explicit
// device list, plus optional `networks:` and `ranges:` blocks that get
// expanded into device entries at Load time.
type FileSource struct {
	devices  []deviceEntry
	networks []NetworkDefinition
	ranges   []RangeDefinition
}

// deviceEntry mirrors DeviceConfig but is used at YAML decode time so
// we can keep the unmarshaling localized to this file.
type deviceEntry struct {
	Name           string            `yaml:"name"`
	Host           string            `yaml:"host"`
	Port           int               `yaml:"port,omitempty"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password,omitempty"`
	Tags           []string          `yaml:"tags,omitempty"`
	Insecure       bool              `yaml:"insecure,omitempty"`
	Plaintext      bool              `yaml:"plaintext,omitempty"`
	Transport      string            `yaml:"transport,omitempty"`
	DisableCache   bool              `yaml:"disable_cache,omitempty"`
	Extra          map[string]string `yaml:"extra,omitempty"`
}

func (e deviceEntry) toConfig() DeviceConfig {
	return DeviceConfig{
		Name:           e.Name,
		Host:           e.Host,
		Port:           e.Port,
		Username:       e.Username,
		Password:       e.Password,
		EnablePassword: e.EnablePassword,
		Tags:           e.Tags,
		Insecure:       e.Insecure,
		Plaintext:      e.Plaintext,
		Transport:      e.Transport,
		DisableCache:   e.DisableCache,
		Extra:          e.Extra,
	}
}

func (s *FileSource) Kind() string { return "file" }

func (s *FileSource) Load(ctx context.Context) (*Inventory, error) {
	inv := &Inventory{
		Networks: s.networks,
		Ranges:   s.ranges,
	}
	for _, d := range s.devices {
		inv.Devices = append(inv.Devices, d.toConfig())
	}
	if err := inv.expandNetworks(); err != nil {
		return nil, fmt.Errorf("expand networks: %w", err)
	}
	if err := inv.expandRanges(); err != nil {
		return nil, fmt.Errorf("expand ranges: %w", err)
	}
	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("validate inventory: %w", err)
	}
	return inv, nil
}

func init() {
	RegisterSource("file", func(node *yaml.Node) (Source, error) {
		var cfg struct {
			Kind     string                `yaml:"kind"`
			Devices  []deviceEntry         `yaml:"devices"`
			Networks []NetworkDefinition   `yaml:"networks"`
			Ranges   []RangeDefinition     `yaml:"ranges"`
		}
		if err := node.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("file source: decode YAML: %w", err)
		}
		return &FileSource{
			devices:  cfg.Devices,
			networks: cfg.Networks,
			ranges:   cfg.Ranges,
		}, nil
	})
}
```

- [ ] **Step 4: Run the new tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestFileSource -v
```

Expected: all 3 subtests pass.

- [ ] **Step 5: Make `LoadInventory` wrap `LoadSource` in `pkg/inventory/inventory.go`**

Find the existing `LoadInventory` (around line 70) and replace its body with a thin delegation:

```go
// LoadInventory is a back-compat wrapper around LoadSource. New callers
// should use LoadSource directly so they can pass a context to Load.
func LoadInventory(path string) (*Inventory, error) {
	src, err := LoadSource(path)
	if err != nil {
		return nil, err
	}
	return src.Load(context.Background())
}
```

Add `"context"` to the file's imports if it isn't there.

Also remove the now-unused `isNetboxInventory` helper and its supporting code from `inventory.go` — `LoadSource`'s `detectKind` covers that path now. (If you find call sites of `isNetboxInventory`, delete them too.)

- [ ] **Step 6: Verify full inventory package still passes**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -v
```

Expected: all existing + new tests pass. If a test in `inventory_test.go` exists and previously called `isNetboxInventory`, update it accordingly.

- [ ] **Step 7: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/ && git commit -m "$(cat <<'EOF'
feat(inventory): add FileSource and route LoadInventory through LoadSource

FileSource implements the Source interface using the existing static-YAML
logic (devices + networks + ranges expansion). LoadInventory is now a
thin back-compat wrapper around LoadSource so existing callers (and
existing YAML files) keep working unchanged.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: `ApplyDefaults` credential overlay

**Files:**
- Modify: `pkg/inventory/inventory.go`
- Create: `pkg/inventory/inventory_test.go`

Sources that fetch from APIs (Netbox, dcfab) don't get credentials — those come from env/CLI flags. `ApplyDefaults` overlays defaults onto each device, but only for fields the device hasn't already set.

- [ ] **Step 1: Write failing tests in `inventory_test.go`**

```go
package inventory

import (
	"testing"

	"github.com/fluidstackio/go-anta/pkg/device"
)

func TestApplyDefaults_OnlyFillsEmpty(t *testing.T) {
	inv := &Inventory{
		Devices: []device.DeviceConfig{
			{Name: "r1", Host: "10.0.0.1"},                              // missing creds
			{Name: "r2", Host: "10.0.0.2", Username: "ops", Password: "p2"}, // pre-filled
		},
	}

	out := inv.ApplyDefaults(DeviceDefaults{
		Username:  "admin",
		Password:  "secret",
		Transport: "gnmi",
		Port:      6030,
		Plaintext: true,
	})

	// r1 should pick up everything from defaults.
	if out.Devices[0].Username != "admin" || out.Devices[0].Password != "secret" {
		t.Errorf("r1 creds: got %q/%q, want admin/secret",
			out.Devices[0].Username, out.Devices[0].Password)
	}
	if out.Devices[0].Transport != "gnmi" || out.Devices[0].Port != 6030 {
		t.Errorf("r1 transport/port: got %q/%d, want gnmi/6030",
			out.Devices[0].Transport, out.Devices[0].Port)
	}
	if !out.Devices[0].Plaintext {
		t.Error("r1 plaintext: expected true")
	}

	// r2 should keep its own creds, defaults must not override.
	if out.Devices[1].Username != "ops" || out.Devices[1].Password != "p2" {
		t.Errorf("r2 creds were overwritten: got %q/%q",
			out.Devices[1].Username, out.Devices[1].Password)
	}
	// Transport was empty on r2, so default should fill it.
	if out.Devices[1].Transport != "gnmi" {
		t.Errorf("r2 transport: got %q, want gnmi", out.Devices[1].Transport)
	}
}

func TestApplyDefaults_ReturnsNewInventoryNotMutated(t *testing.T) {
	inv := &Inventory{
		Devices: []device.DeviceConfig{{Name: "r1", Host: "10.0.0.1"}},
	}
	_ = inv.ApplyDefaults(DeviceDefaults{Username: "admin"})
	if inv.Devices[0].Username != "" {
		t.Errorf("input mutated: Username=%q", inv.Devices[0].Username)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestApplyDefaults
```

Expected: build error (`undefined: DeviceDefaults`, `ApplyDefaults`).

- [ ] **Step 3: Add `DeviceDefaults` and `ApplyDefaults` to `inventory.go`**

Append at the end of `pkg/inventory/inventory.go`:

```go
// DeviceDefaults holds connection settings that the CLI / env supplies
// at run time. Sources that fetch from APIs (Netbox, dcfab) leave these
// empty on the devices they return; the caller overlays them via
// Inventory.ApplyDefaults.
type DeviceDefaults struct {
	Username  string
	Password  string
	Transport string
	Insecure  bool
	Plaintext bool
	Port      int
}

// ApplyDefaults returns a new Inventory in which each device has any
// empty connection-config fields filled in from d. Per-device YAML
// values always win; defaults only fill blanks. The receiver is not
// mutated.
func (i *Inventory) ApplyDefaults(d DeviceDefaults) *Inventory {
	out := &Inventory{
		Networks: i.Networks,
		Ranges:   i.Ranges,
		Devices:  make([]device.DeviceConfig, len(i.Devices)),
	}
	for idx, dev := range i.Devices {
		if dev.Username == "" {
			dev.Username = d.Username
		}
		if dev.Password == "" {
			dev.Password = d.Password
		}
		if dev.Transport == "" {
			dev.Transport = d.Transport
		}
		if dev.Port == 0 {
			dev.Port = d.Port
		}
		// Booleans are tricky — only flip from false if defaults say so.
		if !dev.Insecure && d.Insecure {
			dev.Insecure = true
		}
		if !dev.Plaintext && d.Plaintext {
			dev.Plaintext = true
		}
		out.Devices[idx] = dev
	}
	return out
}
```

Add `"github.com/fluidstackio/go-anta/pkg/device"` to imports if it isn't already there.

- [ ] **Step 4: Run tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestApplyDefaults -v
```

Expected: both subtests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/ && git commit -m "$(cat <<'EOF'
feat(inventory): add DeviceDefaults and ApplyDefaults overlay

Lets the CLI supply username/password/transport/port at run time without
sources needing to know about them. ApplyDefaults returns a new Inventory
in which each device's empty fields are filled from the defaults;
per-device values always win.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Rename `netbox.go` → `netbox_client.go`

**Files:**
- Rename: `pkg/inventory/netbox.go` → `pkg/inventory/netbox_client.go`

A pure rename with no content changes. The file's contents (including `LoadFromNetbox` and `LoadNetboxInventory`) stay intact; Task 6 will move those functions to `source_netbox.go` as wrappers in the same commit it adds `NetboxSource`. This keeps every intermediate commit buildable.

- [ ] **Step 1: Rename the file**

```bash
cd /Users/gmckee/projects/go-anta && git mv pkg/inventory/netbox.go pkg/inventory/netbox_client.go
```

- [ ] **Step 2: Verify build remains clean (no content changed)**

```bash
cd /Users/gmckee/projects/go-anta && go build ./... && go vet ./...
```

Expected: clean.

- [ ] **Step 3: Commit the rename**

```bash
cd /Users/gmckee/projects/go-anta && git add -A && git commit -m "$(cat <<'EOF'
refactor(inventory): rename netbox.go to netbox_client.go

A pure rename in preparation for Task 6, which adds source_netbox.go and
moves LoadFromNetbox / LoadNetboxInventory there as wrappers around the
new NetboxSource. No content changes; this commit alone leaves the
codebase fully functional.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `NetboxSource` and back-compat wrappers

**Files:**
- Create: `pkg/inventory/source_netbox.go`
- Create: `pkg/inventory/source_netbox_test.go`
- Modify: `pkg/inventory/netbox_client.go` (remove `LoadFromNetbox`, `LoadNetboxInventory`, `NetboxInventoryConfig` — they move to `source_netbox.go`)

`NetboxSource` wraps the existing `NetboxClient` and implements the `Source` interface. The factory parses the YAML config into a `NetboxConfig` + `NetboxQuery`. The legacy `LoadFromNetbox` and `LoadNetboxInventory` functions are reimplemented in `source_netbox.go` as thin wrappers around the new types so existing CLI code keeps working. Both moves happen in the same commit so the codebase stays buildable.

- [ ] **Step 1: Write failing tests in `source_netbox_test.go`**

```go
package inventory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNetboxSource_FactoryParsesYAML(t *testing.T) {
	tmp := writeYAML(t, `
kind: netbox
url: https://netbox.example.com
token: secret
insecure: true
query:
  site: wdl1
  role: leaf
`)
	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	if src.Kind() != "netbox" {
		t.Errorf("Kind: got %q want netbox", src.Kind())
	}
	nb, ok := src.(*NetboxSource)
	if !ok {
		t.Fatalf("expected *NetboxSource, got %T", src)
	}
	if nb.config.URL != "https://netbox.example.com" {
		t.Errorf("URL: got %q", nb.config.URL)
	}
	if nb.config.Token != "secret" || !nb.config.Insecure {
		t.Errorf("config not parsed correctly: %+v", nb.config)
	}
	if nb.query.Site != "wdl1" || nb.query.Role != "leaf" {
		t.Errorf("query not parsed: %+v", nb.query)
	}
}

func TestNetboxSource_LoadHonorsContextCancel(t *testing.T) {
	// httptest server that hangs until ctx is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	nb := &NetboxSource{
		config: NetboxConfig{URL: srv.URL, Token: "x"},
		query:  NetboxQuery{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() { cancel() }()
	_, err := nb.Load(ctx)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestNetboxSource
```

Expected: build error (`undefined: NetboxSource`).

- [ ] **Step 3: Remove the legacy entry points from `netbox_client.go`**

Open `pkg/inventory/netbox_client.go` and delete (they'll be recreated in `source_netbox.go` in the next step):

- `func LoadFromNetbox(...)` and the entire function body.
- `type NetboxInventoryConfig struct { ... }`.
- `func LoadNetboxInventory(...)` and the entire function body.

Keep everything else: `NetboxConfig`, `NetboxClient`, `NewNetboxClient`, `NetboxQuery`, `NetboxDevice` and friends, `NetboxResponse`, `GetDevices`.

After this edit `go build ./...` will FAIL with `undefined: LoadFromNetbox` from CLI code. That's expected and intentional — the next step recreates those names in `source_netbox.go` and the build returns to clean before we commit.

- [ ] **Step 4: Create `pkg/inventory/source_netbox.go`**

```go
package inventory

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"
)

// NetboxSource implements Source by fetching devices from a Netbox
// instance via its REST API. Configured via the `kind: netbox` YAML
// schema.
type NetboxSource struct {
	config NetboxConfig
	query  NetboxQuery
	// credentials are populated by ApplyDefaults later; here we just
	// stash any per-source defaults the YAML included.
	defaults DeviceDefaults
}

func (s *NetboxSource) Kind() string { return "netbox" }

func (s *NetboxSource) Load(ctx context.Context) (*Inventory, error) {
	client := NewNetboxClient(s.config)
	devices, err := client.GetDevices(ctx, s.query)
	if err != nil {
		return nil, fmt.Errorf("netbox: fetch devices: %w", err)
	}

	inv := &Inventory{}
	for _, d := range devices {
		cfg := netboxDeviceToConfig(d, s.defaults)
		if cfg.Name == "" || cfg.Host == "" {
			continue
		}
		inv.Devices = append(inv.Devices, cfg)
	}
	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("netbox: validate inventory: %w", err)
	}
	return inv, nil
}

// netboxDeviceToConfig maps a Netbox API response into a DeviceConfig.
// Credentials come from defaults; the call site will typically also
// invoke Inventory.ApplyDefaults afterward to fill in env/CLI values.
func netboxDeviceToConfig(d NetboxDevice, defaults DeviceDefaults) DeviceConfig {
	host := ""
	if d.PrimaryIP4.Address != "" {
		host = stripCIDR(d.PrimaryIP4.Address)
	} else if d.PrimaryIP6.Address != "" {
		host = stripCIDR(d.PrimaryIP6.Address)
	}
	tags := make([]string, 0, len(d.Tags)+3)
	for _, tag := range d.Tags {
		tags = append(tags, tag.Slug)
	}
	if d.DeviceRole.Slug != "" {
		tags = append(tags, "role:"+d.DeviceRole.Slug)
	}
	if d.Platform.Slug != "" {
		tags = append(tags, "platform:"+d.Platform.Slug)
	}
	if d.Site.Slug != "" {
		tags = append(tags, "site:"+d.Site.Slug)
	}
	return DeviceConfig{
		Name:      d.Name,
		Host:      host,
		Tags:      tags,
		Username:  defaults.Username,
		Password:  defaults.Password,
		Transport: defaults.Transport,
		Port:      defaults.Port,
		Insecure:  defaults.Insecure,
		Plaintext: defaults.Plaintext,
	}
}

// stripCIDR removes a /NN suffix from an IP, returning just the address.
// Netbox primary_ip fields come back as "10.0.0.1/24" style strings.
func stripCIDR(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i]
		}
	}
	return s
}

// LoadFromNetbox is a back-compat wrapper around NetboxSource. New
// callers should use LoadSource with `kind: netbox` YAML.
func LoadFromNetbox(config NetboxConfig, query NetboxQuery, credentials map[string]interface{}) (*Inventory, error) {
	defaults := DeviceDefaults{}
	if v, ok := credentials["username"].(string); ok {
		defaults.Username = v
	}
	if v, ok := credentials["password"].(string); ok {
		defaults.Password = v
	}
	if v, ok := credentials["insecure"].(bool); ok {
		defaults.Insecure = v
	}
	src := &NetboxSource{config: config, query: query, defaults: defaults}
	return src.Load(context.Background())
}

// LoadNetboxInventory is a back-compat wrapper that reads a legacy
// inventory YAML (with a top-level netbox: block) and uses NetboxSource.
func LoadNetboxInventory(path string) (*Inventory, error) {
	src, err := LoadSource(path)
	if err != nil {
		return nil, err
	}
	return src.Load(context.Background())
}

func init() {
	RegisterSource("netbox", func(node *yaml.Node) (Source, error) {
		// Two flavors: new format (top-level url/token/query) and legacy
		// (top-level `netbox:` block wrapping the same fields).
		var newFormat struct {
			Kind     string       `yaml:"kind"`
			URL      string       `yaml:"url"`
			Token    string       `yaml:"token"`
			Insecure bool         `yaml:"insecure"`
			Query    NetboxQuery  `yaml:"query"`
		}
		if err := node.Decode(&newFormat); err == nil && newFormat.URL != "" {
			return &NetboxSource{
				config: NetboxConfig{URL: newFormat.URL, Token: newFormat.Token, Insecure: newFormat.Insecure},
				query:  newFormat.Query,
			}, nil
		}

		var legacy struct {
			Netbox struct {
				URL      string      `yaml:"url"`
				Token    string      `yaml:"token"`
				Insecure bool        `yaml:"insecure"`
				Query    NetboxQuery `yaml:"query"`
			} `yaml:"netbox"`
		}
		if err := node.Decode(&legacy); err != nil {
			return nil, fmt.Errorf("netbox source: decode YAML: %w", err)
		}
		if legacy.Netbox.URL == "" {
			return nil, fmt.Errorf("netbox source: url is required")
		}
		return &NetboxSource{
			config: NetboxConfig{URL: legacy.Netbox.URL, Token: legacy.Netbox.Token, Insecure: legacy.Netbox.Insecure},
			query:  legacy.Netbox.Query,
		}, nil
	})
}
```

Note on field names: if `NetboxDevice` doesn't have `PrimaryIP4` / `PrimaryIP6` / `Tags` / `DeviceRole` / `Platform` / `Site` exactly as written, look at the existing struct in `netbox_client.go` and adjust. The mapping intent is clear: pull the management IP and useful identifiers into the device's `Host` and `Tags`.

- [ ] **Step 5: Run tests**

```bash
cd /Users/gmckee/projects/go-anta && go build ./... && go test ./pkg/inventory/... -v
```

Expected: build clean, all tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/source_netbox.go pkg/inventory/source_netbox_test.go pkg/inventory/netbox_client.go && git commit -m "$(cat <<'EOF'
feat(inventory): add NetboxSource and move legacy entry points

Wraps the existing NetboxClient with the Source interface and registers
the netbox kind. Supports both the new format (top-level url/token/query)
and the legacy format (wrapped under a `netbox:` key). The old
LoadFromNetbox / LoadNetboxInventory entry points are preserved as thin
back-compat wrappers around NetboxSource so all existing CLI call sites
keep working — they just live in source_netbox.go now instead of
netbox_client.go.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `DcfabSource` — config + GraphQL query builder

**Files:**
- Create: `pkg/inventory/source_dcfab.go`
- Create: `pkg/inventory/source_dcfab_test.go`

Two-stage: first the config struct + query-builder logic (no network), then `Load()` proper. Splitting them lets us TDD the URL construction without mocking HTTP yet.

- [ ] **Step 1: Write failing tests for the query builder**

```go
package inventory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDcfabSource_FactoryParsesYAML(t *testing.T) {
	tmp := writeYAML(t, `
kind: dcfab
env: prod
region: wdl1
roles: [fm, ft]
platforms: [eos]
prefer_ip: ipv6
`)
	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	d, ok := src.(*DcfabSource)
	if !ok {
		t.Fatalf("expected *DcfabSource, got %T", src)
	}
	if d.cfg.Region != "wdl1" || d.cfg.Env != "prod" || d.cfg.PreferIP != "ipv6" {
		t.Errorf("config not parsed: %+v", d.cfg)
	}
	if len(d.cfg.Roles) != 2 || d.cfg.Roles[0] != "fm" {
		t.Errorf("roles not parsed: %v", d.cfg.Roles)
	}
}

func TestDcfabSource_QueryURLEncodesAllFilters(t *testing.T) {
	src := &DcfabSource{
		cfg: DcfabConfig{
			Region:    "wdl1",
			Roles:     []string{"fm", "ft"},
			Platforms: []string{"eos"},
		},
	}
	u := src.queryURL("https://dcfab.example.com")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if !strings.HasPrefix(u, "https://dcfab.example.com/v1alpha1/query?query=") {
		t.Errorf("wrong endpoint prefix: %s", u)
	}
	q := parsed.Query().Get("query")
	for _, want := range []string{`region(name: "wdl1")`, `roles: ["fm","ft"]`, `platforms: ["eos"]`, `implementation: ACTIVE`, `managementInterface`} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q\nfull query: %s", want, q)
		}
	}
}

func TestDcfabSource_EndpointDefaults(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", "https://dcfab.fluidstack.io"},
		{"prod", "https://dcfab.fluidstack.io"},
		{"dev", "https://dcfab.fluidstack.xyz"},
	}
	for _, tc := range cases {
		got := dcfabEndpoint(DcfabConfig{Env: tc.env})
		if got != tc.want {
			t.Errorf("env=%q: got %q want %q", tc.env, got, tc.want)
		}
	}
	// Explicit Endpoint wins.
	custom := dcfabEndpoint(DcfabConfig{Env: "prod", Endpoint: "https://custom.test"})
	if custom != "https://custom.test" {
		t.Errorf("explicit endpoint should win, got %q", custom)
	}
}

// (Used in Task 8; declared here so the file compiles when we add it.)
func mockDcfabServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// silence "imported and not used" when ctx is added in Task 8.
var _ context.Context = context.TODO()
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestDcfabSource
```

Expected: build error (`undefined: DcfabSource`, etc.).

- [ ] **Step 3: Create `pkg/inventory/source_dcfab.go`**

```go
package inventory

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// DcfabConfig is the YAML schema for `kind: dcfab` inventory files.
type DcfabConfig struct {
	Env       string   `yaml:"env"`         // prod | dev (default prod)
	Region    string   `yaml:"region"`      // required
	Roles     []string `yaml:"roles"`       // optional filter
	Platforms []string `yaml:"platforms"`   // optional filter
	Endpoint  string   `yaml:"endpoint"`    // optional override
	PreferIP  string   `yaml:"prefer_ip"`   // ipv4 | ipv6 (default ipv6)
}

// DcfabSource implements Source by querying the dcfab GraphQL API.
type DcfabSource struct {
	cfg      DcfabConfig
	defaults DeviceDefaults
}

func (s *DcfabSource) Kind() string { return "dcfab" }

// dcfabEndpoint resolves the HTTPS endpoint from the config. Explicit
// Endpoint wins; otherwise Env selects between prod/dev defaults.
func dcfabEndpoint(cfg DcfabConfig) string {
	if cfg.Endpoint != "" {
		return cfg.Endpoint
	}
	switch cfg.Env {
	case "dev":
		return "https://dcfab.fluidstack.xyz"
	default:
		return "https://dcfab.fluidstack.io"
	}
}

// queryURL builds the full GET URL for the dcfab GraphQL endpoint with
// the configured filters baked into the query string.
func (s *DcfabSource) queryURL(endpoint string) string {
	q := s.buildQuery()
	v := url.Values{}
	v.Set("query", q)
	return strings.TrimRight(endpoint, "/") + "/v1alpha1/query?" + v.Encode()
}

// buildQuery assembles the GraphQL query string. Format must match what
// the dcfab schema-reference describes: region(name:...).devices(...)
// returning ActiveDevice fields + managementInterface.addresses.
func (s *DcfabSource) buildQuery() string {
	args := []string{`implementation: ACTIVE`, `limit: 5000`}
	if len(s.cfg.Roles) > 0 {
		args = append(args, fmt.Sprintf(`roles: %s`, stringSliceLiteral(s.cfg.Roles)))
	}
	if len(s.cfg.Platforms) > 0 {
		args = append(args, fmt.Sprintf(`platforms: %s`, stringSliceLiteral(s.cfg.Platforms)))
	}
	return fmt.Sprintf(`{
  region(name: %q) {
    devices(%s) {
      ... on ActiveDevice {
        name
        role
        platform
        managementInterface {
          addresses {
            address
            version
          }
        }
      }
    }
  }
}`, s.cfg.Region, strings.Join(args, ", "))
}

// stringSliceLiteral formats ["a","b"] for embedding in a GraphQL
// argument list.
func stringSliceLiteral(in []string) string {
	parts := make([]string, len(in))
	for i, s := range in {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// Load is implemented in Task 8.
func (s *DcfabSource) Load(ctx context.Context) (*Inventory, error) {
	return nil, fmt.Errorf("DcfabSource.Load not yet implemented")
}

func init() {
	RegisterSource("dcfab", func(node *yaml.Node) (Source, error) {
		var cfg struct {
			Kind string `yaml:"kind"`
			DcfabConfig `yaml:",inline"`
		}
		if err := node.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("dcfab source: decode YAML: %w", err)
		}
		if cfg.Region == "" {
			return nil, fmt.Errorf("dcfab source: region is required")
		}
		return &DcfabSource{cfg: cfg.DcfabConfig}, nil
	})
}
```

- [ ] **Step 4: Run the new tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestDcfabSource -v
```

Expected: the 3 query-builder tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/source_dcfab.go pkg/inventory/source_dcfab_test.go && git commit -m "$(cat <<'EOF'
feat(inventory): add DcfabSource config and GraphQL query builder

Implements the YAML factory, config struct, endpoint-default logic, and
URL-encoded GraphQL query builder for the kind: dcfab inventory source.
Load() is stubbed and returns an error; Task 8 wires the HTTP layer.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: `DcfabSource.Load` — HTTP fetch + response parsing

**Files:**
- Modify: `pkg/inventory/source_dcfab.go`
- Modify: `pkg/inventory/source_dcfab_test.go`

- [ ] **Step 1: Add failing tests in `source_dcfab_test.go`**

Append:

```go
func TestDcfabSource_LoadMapsDevicesToConfigs(t *testing.T) {
	body := `{
	  "data": {
	    "region": {
	      "devices": [
	        {
	          "name": "wdl101-fis-fm1-r1",
	          "role": "fm",
	          "platform": "eos",
	          "managementInterface": {
	            "addresses": [
	              {"address": "fc00:800f:f01::8/64", "version": 6},
	              {"address": "10.0.0.8/24", "version": 4}
	            ]
	          }
	        },
	        {
	          "name": "wdl101-fis-fm1-r2",
	          "role": "fm",
	          "platform": "eos",
	          "managementInterface": {
	            "addresses": [
	              {"address": "10.0.0.9/24", "version": 4}
	            ]
	          }
	        }
	      ]
	    }
	  }
	}`
	srv := mockDcfabServer(t, body)

	src := &DcfabSource{
		cfg: DcfabConfig{
			Region:   "wdl1",
			Endpoint: srv.URL,
			PreferIP: "ipv6",
		},
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Devices) != 2 {
		t.Fatalf("device count: got %d want 2", len(inv.Devices))
	}
	// IPv6-preferring device picks the v6 address; the v4-only device
	// falls back to v4.
	if inv.Devices[0].Host != "fc00:800f:f01::8" {
		t.Errorf("device 0 host: got %q want fc00:800f:f01::8", inv.Devices[0].Host)
	}
	if inv.Devices[1].Host != "10.0.0.9" {
		t.Errorf("device 1 host: got %q want 10.0.0.9", inv.Devices[1].Host)
	}
	// Tags include role + platform.
	if !containsString(inv.Devices[0].Tags, "role:fm") {
		t.Errorf("tags missing role:fm: %v", inv.Devices[0].Tags)
	}
}

func TestDcfabSource_LoadErrorsAtPaginationCap(t *testing.T) {
	// Build a response with exactly 5000 devices to trigger the cap.
	var b strings.Builder
	b.WriteString(`{"data":{"region":{"devices":[`)
	for i := 0; i < 5000; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"name":"r%d","role":"x","platform":"y","managementInterface":{"addresses":[{"address":"10.0.0.%d/32","version":4}]}}`, i, i%256)
	}
	b.WriteString(`]}}}`)
	srv := mockDcfabServer(t, b.String())

	src := &DcfabSource{cfg: DcfabConfig{Region: "x", Endpoint: srv.URL}}
	_, err := src.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "5000") {
		t.Errorf("expected 5000-device cap error, got %v", err)
	}
}

func TestDcfabSource_LoadSkipsDevicesWithNoAddress(t *testing.T) {
	body := `{
	  "data": {
	    "region": {
	      "devices": [
	        {"name": "noaddr", "role": "fm", "platform": "eos", "managementInterface": null},
	        {"name": "ok", "role": "fm", "platform": "eos",
	         "managementInterface": {"addresses": [{"address": "10.0.0.1/24", "version": 4}]}}
	      ]
	    }
	  }
	}`
	srv := mockDcfabServer(t, body)
	src := &DcfabSource{cfg: DcfabConfig{Region: "x", Endpoint: srv.URL}}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "ok" {
		t.Errorf("expected only ok device, got %+v", inv.Devices)
	}
}

func containsString(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
```

Add `"fmt"` to the test file's imports if not already there.

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestDcfabSource_Load
```

Expected: stub error from current Load.

- [ ] **Step 3: Implement Load in `source_dcfab.go`**

Replace the stub `Load` with:

```go
// dcfabResponse is the JSON shape returned by the dcfab GraphQL endpoint
// for our region/devices query.
type dcfabResponse struct {
	Data struct {
		Region *struct {
			Devices []dcfabDevice `json:"devices"`
		} `json:"region"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

type dcfabDevice struct {
	Name                string                `json:"name"`
	Role                string                `json:"role"`
	Platform            string                `json:"platform"`
	ManagementInterface *dcfabMgmtInterface   `json:"managementInterface"`
}

type dcfabMgmtInterface struct {
	Addresses []dcfabAddress `json:"addresses"`
}

type dcfabAddress struct {
	Address string `json:"address"`
	Version int    `json:"version"`
}

// dcfabPaginationCap is the GraphQL complexity-limit-derived ceiling
// from the skill docs. Hitting exactly this number signals possible
// truncation.
const dcfabPaginationCap = 5000

func (s *DcfabSource) Load(ctx context.Context) (*Inventory, error) {
	endpoint := dcfabEndpoint(s.cfg)
	u := s.queryURL(endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("dcfab: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dcfab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("dcfab: status %d: %s", resp.StatusCode, body)
	}

	var parsed dcfabResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("dcfab: decode response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("dcfab: GraphQL error: %s", parsed.Errors[0].Message)
	}
	if parsed.Data.Region == nil {
		return nil, fmt.Errorf("dcfab: region %q not found", s.cfg.Region)
	}

	devices := parsed.Data.Region.Devices
	if len(devices) == dcfabPaginationCap {
		return nil, fmt.Errorf("dcfab: response hit %d-device pagination cap; narrow your roles/platforms filter or implement pagination", dcfabPaginationCap)
	}

	inv := &Inventory{}
	for _, d := range devices {
		host := pickMgmtAddress(d.ManagementInterface, s.cfg.PreferIP)
		if host == "" {
			continue
		}
		inv.Devices = append(inv.Devices, DeviceConfig{
			Name:      d.Name,
			Host:      host,
			Tags:      buildDcfabTags(d, s.cfg.Region),
			Username:  s.defaults.Username,
			Password:  s.defaults.Password,
			Transport: s.defaults.Transport,
			Port:      s.defaults.Port,
			Insecure:  s.defaults.Insecure,
			Plaintext: s.defaults.Plaintext,
		})
	}
	if len(inv.Devices) == 0 {
		return inv, nil
	}
	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("dcfab: validate inventory: %w", err)
	}
	return inv, nil
}

// pickMgmtAddress returns the management address per the prefer_ip
// policy. Falls back to the other family if the preferred isn't present.
func pickMgmtAddress(mi *dcfabMgmtInterface, prefer string) string {
	if mi == nil {
		return ""
	}
	if prefer == "" {
		prefer = "ipv6"
	}
	var preferred, fallback string
	for _, a := range mi.Addresses {
		stripped := stripCIDR(a.Address)
		switch a.Version {
		case 6:
			if prefer == "ipv6" && preferred == "" {
				preferred = stripped
			} else if fallback == "" {
				fallback = stripped
			}
		case 4:
			if prefer == "ipv4" && preferred == "" {
				preferred = stripped
			} else if fallback == "" {
				fallback = stripped
			}
		}
	}
	if preferred != "" {
		return preferred
	}
	return fallback
}

func buildDcfabTags(d dcfabDevice, region string) []string {
	tags := []string{}
	if d.Role != "" {
		tags = append(tags, "role:"+d.Role)
	}
	if d.Platform != "" {
		tags = append(tags, "platform:"+d.Platform)
	}
	if region != "" {
		tags = append(tags, "region:"+region)
	}
	return tags
}
```

Update imports at the top of `source_dcfab.go` to include `"encoding/json"`, `"io"`, `"net/http"`.

- [ ] **Step 4: Run all dcfab tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./pkg/inventory/... -run TestDcfabSource -v
```

Expected: all dcfab subtests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add pkg/inventory/source_dcfab.go pkg/inventory/source_dcfab_test.go && git commit -m "$(cat <<'EOF'
feat(inventory): implement DcfabSource.Load with HTTP + response parsing

Issues a single HTTP GET against the dcfab GraphQL endpoint, parses the
ActiveDevice list, and maps each entry to a DeviceConfig. The
managementInterface address picker honors prefer_ip (default ipv6) with
a graceful fallback to the other family. Devices with no management
address are skipped. Hitting exactly 5000 devices returns an explicit
"narrow your filters" error rather than silently truncating.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Shared CLI inventory loader helper

**Files:**
- Create: `internal/cli/commands/inventoryload.go`
- Create: `internal/cli/commands/inventoryload_test.go`

A single function used by all three commands (`nrfu`, `check`, `inventory`). Replaces ~110 lines of copy-pasted Netbox query parsing.

- [ ] **Step 1: Write failing tests in `inventoryload_test.go`**

```go
package commands

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestLoadInventoryForRun_StaticFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/inv.yaml"
	body := `
kind: file
devices:
  - name: r1
    host: 10.0.0.1
    username: admin
    password: pw
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	opts := InventoryLoadOptions{Path: path}
	inv, err := LoadInventoryForRun(context.Background(), opts)
	if err != nil {
		t.Fatalf("LoadInventoryForRun: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "r1" {
		t.Errorf("got %+v", inv.Devices)
	}
}

func TestLoadInventoryForRun_RejectsUnknownSourceFlag(t *testing.T) {
	opts := InventoryLoadOptions{Path: "/nope", SourceOverride: "smtp"}
	_, err := LoadInventoryForRun(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown-source error, got %v", err)
	}
}

func TestLoadInventoryForRun_RequiresOneOfPathOrNetboxURL(t *testing.T) {
	_, err := LoadInventoryForRun(context.Background(), InventoryLoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "inventory") {
		t.Errorf("expected missing-inventory error, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

```bash
cd /Users/gmckee/projects/go-anta && go test ./internal/cli/commands/... -run TestLoadInventoryForRun
```

Expected: build error (`undefined: LoadInventoryForRun`).

- [ ] **Step 3: Implement `inventoryload.go`**

```go
package commands

import (
	"context"
	"fmt"
	"os"

	"github.com/fluidstackio/go-anta/pkg/inventory"
)

// InventoryLoadOptions bundles every CLI flag/env value that affects how
// the inventory is loaded for a single command run.
type InventoryLoadOptions struct {
	// Source location.
	Path           string // --inventory; YAML file path
	SourceOverride string // --source; overrides YAML kind

	// Source-specific override knobs (any subset may be set).
	NetboxURL    string // --netbox-url; if set without Path, synthesize a netbox source
	NetboxToken  string // --netbox-token
	NetboxQuery  string // --netbox-query (raw key=value,key=value)
	Region       string // --region (dcfab override)
	Roles        string // --roles (dcfab override, comma-separated)

	// Credential / connection defaults applied after Load.
	Defaults inventory.DeviceDefaults
}

// LoadInventoryForRun resolves which inventory source to use, loads it,
// then overlays defaults. This is the single entry point used by nrfu,
// check, and inventory commands so source-specific parsing lives in one
// place.
func LoadInventoryForRun(ctx context.Context, opts InventoryLoadOptions) (*inventory.Inventory, error) {
	// Validate --source override up front so a bad value fails fast.
	switch opts.SourceOverride {
	case "", "file", "netbox", "dcfab":
	default:
		return nil, fmt.Errorf("unknown --source value %q (supported: file, netbox, dcfab)", opts.SourceOverride)
	}

	switch {
	case opts.Path != "":
		var src inventory.Source
		var err error
		if opts.SourceOverride != "" {
			src, err = inventory.LoadSourceAs(opts.Path, opts.SourceOverride)
		} else {
			src, err = inventory.LoadSource(opts.Path)
		}
		if err != nil {
			return nil, err
		}
		inv, err := src.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("inventory %s: %w", opts.Path, err)
		}
		return inv.ApplyDefaults(opts.Defaults), nil

	case opts.NetboxURL != "" || os.Getenv("NETBOX_URL") != "":
		// Synthesize a netbox source from CLI flags + env. Same shape as
		// the YAML-based path would produce; reusing the factory keeps a
		// single code path.
		return loadNetboxFromFlags(ctx, opts)

	default:
		return nil, fmt.Errorf("either --inventory or --netbox-url must be specified")
	}
}

func loadNetboxFromFlags(ctx context.Context, opts InventoryLoadOptions) (*inventory.Inventory, error) {
	url := opts.NetboxURL
	if url == "" {
		url = os.Getenv("NETBOX_URL")
	}
	token := opts.NetboxToken
	if token == "" {
		token = os.Getenv("NETBOX_TOKEN")
	}
	query := parseNetboxQueryString(opts.NetboxQuery)

	src := inventory.NewNetboxSource(inventory.NetboxConfig{URL: url, Token: token}, query)
	inv, err := src.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("netbox: %w", err)
	}
	return inv.ApplyDefaults(opts.Defaults), nil
}

// parseNetboxQueryString turns a "k1=v1,k2=v2" CLI string into a
// NetboxQuery. Unknown keys are silently ignored (matching the original
// CLI behavior); recognized keys are listed in the Netbox source.
func parseNetboxQueryString(s string) inventory.NetboxQuery {
	q := inventory.NetboxQuery{}
	if s == "" {
		return q
	}
	for _, pair := range splitCSV(s) {
		kv := splitOnce(pair, "=")
		if len(kv) != 2 {
			continue
		}
		key, value := trimSpace(kv[0]), trimSpace(kv[1])
		switch key {
		case "site":
			q.Site = value
		case "role":
			q.Role = value
		case "device_type":
			q.DeviceType = value
		case "manufacturer":
			q.Manufacturer = value
		case "platform":
			q.Platform = value
		case "status":
			q.Status = value
		case "tenant":
			q.Tenant = value
		case "region":
			q.Region = value
		case "name":
			q.Name = value
		case "name_contains":
			q.NameContains = value
		case "tag":
			q.Tags = append(q.Tags, value)
		case "site_id":
			q.SiteID = value
		case "role_id":
			q.RoleID = value
		case "device_type_id":
			q.DeviceTypeID = value
		}
	}
	return q
}

// trimSpace, splitCSV, splitOnce are tiny helpers to avoid pulling
// strings/strings.SplitN/strings.TrimSpace usage scattered through this
// file inline. Defined here for clarity.
func splitCSV(s string) []string {
	out := []string{}
	current := ""
	for _, r := range s {
		if r == ',' {
			out = append(out, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	out = append(out, current)
	return out
}

func splitOnce(s, sep string) []string {
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
```

Note: this code references `inventory.NewNetboxSource` and `inventory.NetboxQuery.SiteID`/`RoleID`/`DeviceTypeID` — fields that may already exist in `netbox_client.go` and a constructor that may not exist yet. If `NewNetboxSource(config, query)` doesn't exist, add a one-line constructor to `source_netbox.go`:

```go
func NewNetboxSource(config NetboxConfig, query NetboxQuery) *NetboxSource {
    return &NetboxSource{config: config, query: query}
}
```

And confirm `NetboxQuery` has the `SiteID`, `RoleID`, `DeviceTypeID` fields. If not, add them (this is item R5 from the prior review — `check.go` was silently missing these keys).

- [ ] **Step 4: Run the new tests**

```bash
cd /Users/gmckee/projects/go-anta && go test ./internal/cli/commands/... -run TestLoadInventoryForRun -v
```

Expected: all 3 subtests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add internal/cli/commands/inventoryload.go internal/cli/commands/inventoryload_test.go pkg/inventory/source_netbox.go pkg/inventory/netbox_client.go && git commit -m "$(cat <<'EOF'
feat(cli): shared inventory loader helper

LoadInventoryForRun is the single entry point used by nrfu, check, and
inventory commands. Resolves the source (YAML kind or synthesized from
CLI flags), calls src.Load(ctx), and overlays per-run DeviceDefaults.

The Netbox query string parser now recognizes site_id, role_id, and
device_type_id — keys the original check.go silently ignored. Tested
against the new central parser so all three commands behave identically.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: Switch `nrfu` to the shared loader

**Files:**
- Modify: `internal/cli/commands/nrfu.go`

Replaces the existing ~110 lines of inventory-load + per-device-config logic with a call to `LoadInventoryForRun`. The dry-run path keeps its existing display behavior.

- [ ] **Step 1: Add the new CLI flags**

In the `var ( ... )` block at the top of `nrfu.go`, add (alongside the existing `transport` field):

```go
	source string
	region string
	roles  string
```

In `init()`, after the existing `--transport` registration:

```go
	NrfuCmd.Flags().StringVar(&source, "source", "", "override the YAML inventory kind (file, netbox, dcfab)")
	NrfuCmd.Flags().StringVar(&region, "region", "", "dcfab region filter")
	NrfuCmd.Flags().StringVar(&roles, "roles", "", "dcfab roles filter (comma-separated)")
```

- [ ] **Step 2: Replace the inventory-load block in `runNrfu`**

Find the section that:
1. Reads `--inventory` or `--netbox-url`.
2. Calls `inventory.LoadInventory` or `loadNetboxInventory`.
3. Applies any per-device defaults.

(In the current code this spans roughly lines 95-160.)

Replace with:

```go
	inv, err := LoadInventoryForRun(ctx, InventoryLoadOptions{
		Path:           inventoryFile,
		SourceOverride: source,
		NetboxURL:      netboxURL,
		NetboxToken:    netboxToken,
		NetboxQuery:    netboxQuery,
		Region:         region,
		Roles:          roles,
		Defaults: inventory.DeviceDefaults{
			Username:  deviceUsername,
			Password:  devicePassword,
			Transport: transport,
			Insecure:  true, // existing default for lab use
		},
	})
	if err != nil {
		return err
	}
```

Delete the now-unused `loadNetboxInventory(ctx)` function from `nrfu.go` and the `os.Getenv("DEVICE_USERNAME")` / `os.Getenv("DEVICE_PASSWORD")` fallback logic that was inline. Move those env fallbacks into `LoadInventoryForRun` if they aren't there already — best place is in the construction of `opts.Defaults` (read env if the CLI flag is empty).

Update `LoadInventoryForRun` (in `inventoryload.go`) to apply the env-fallbacks for `Defaults.Username` / `Defaults.Password`:

```go
// Inside LoadInventoryForRun, before the switch:
	if opts.Defaults.Username == "" {
		opts.Defaults.Username = os.Getenv("DEVICE_USERNAME")
	}
	if opts.Defaults.Password == "" {
		opts.Defaults.Password = os.Getenv("DEVICE_PASSWORD")
	}
```

- [ ] **Step 3: Run a quick verification build**

```bash
cd /Users/gmckee/projects/go-anta && go build ./... && go vet ./...
```

Expected: clean. If anything is `undefined`, it's a name mismatch — fix.

- [ ] **Step 4: Verify --help shows the new flags**

```bash
cd /Users/gmckee/projects/go-anta && go build -o /tmp/anta-load ./cmd/go-anta && /tmp/anta-load nrfu --help | grep -E "^  --(source|region|roles)" ; rm -f /tmp/anta-load
```

Expected: three lines, one per new flag.

- [ ] **Step 5: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add internal/cli/commands/nrfu.go internal/cli/commands/inventoryload.go && git commit -m "$(cat <<'EOF'
feat(cli): nrfu uses shared inventory loader; adds --source/--region/--roles

Replaces the ~110 lines of inline inventory-load + Netbox-query parsing
with a single call to LoadInventoryForRun. DEVICE_USERNAME/PASSWORD env
fallback moves into the loader so all commands honor them uniformly.
Three new flags expose source-override (--source) and dcfab filters
(--region, --roles).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: Switch `check` and `inventory` commands to the shared loader

**Files:**
- Modify: `internal/cli/commands/check.go`
- Modify: `internal/cli/commands/inventory.go`

Mirror Task 10 in the other two commands. They use the same loader options struct, so the diff is mostly mechanical.

- [ ] **Step 1: Update `check.go`**

In the `var (...)` block add:

```go
	checkSource string
	checkRegion string
	checkRoles  string
```

In `init()`:

```go
	CheckCmd.Flags().StringVar(&checkSource, "source", "", "override the YAML inventory kind (file, netbox, dcfab)")
	CheckCmd.Flags().StringVar(&checkRegion, "region", "", "dcfab region filter")
	CheckCmd.Flags().StringVar(&checkRoles, "roles", "", "dcfab roles filter (comma-separated)")
```

Replace the inventory-load section in `runCheck` with:

```go
	inv, err := LoadInventoryForRun(ctx, InventoryLoadOptions{
		Path:           inventoryFile,
		SourceOverride: checkSource,
		NetboxURL:      checkNetboxURL,
		NetboxToken:    checkNetboxToken,
		NetboxQuery:    checkNetboxQuery,
		Region:         checkRegion,
		Roles:          checkRoles,
		Defaults: inventory.DeviceDefaults{
			Username:  checkDeviceUsername,
			Password:  checkDevicePassword,
			Transport: checkTransport,
			Insecure:  true,
		},
	})
	if err != nil {
		return err
	}
```

Delete the now-unused `loadCheckNetboxInventory(ctx)` function and the per-command Netbox-query parser that used to live inside it. The shared `parseNetboxQueryString` handles it.

- [ ] **Step 2: Update `inventory.go` (the CLI command, not the package)**

Same pattern. Add `invSource`, `invRegion`, `invRoles` to the var block; register the flags; replace the load logic with a call to `LoadInventoryForRun` using `invFile`/`invNetboxURL`/`invNetboxToken`/`invNetboxQuery`/`invSource`/`invRegion`/`invRoles`.

Delete the now-unused `loadInventoryFromNetbox(ctx)` function.

- [ ] **Step 3: Build and verify**

```bash
cd /Users/gmckee/projects/go-anta && go build ./... && go vet ./... && go test ./...
```

Expected: clean. All existing tests should still pass.

```bash
# Confirm both commands show the new flags.
go build -o /tmp/anta-task11 ./cmd/go-anta
/tmp/anta-task11 check --help | grep -E "^  --(source|region|roles)"
/tmp/anta-task11 inventory --help | grep -E "^  --(source|region|roles)"
rm -f /tmp/anta-task11
```

Expected: three lines each.

- [ ] **Step 4: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add internal/cli/commands/check.go internal/cli/commands/inventory.go && git commit -m "$(cat <<'EOF'
feat(cli): check and inventory commands use shared inventory loader

Mirrors nrfu's switch to LoadInventoryForRun. All three commands now
share one code path for resolving the inventory source, which fixes
check.go's previously silent missing query keys (site_id, role_id,
device_type_id).

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: Example dcfab inventory and end-to-end verification

**Files:**
- Create: `examples/wdl101-dcfab.yaml`

- [ ] **Step 1: Create the example**

```yaml
# examples/wdl101-dcfab.yaml
#
# dcfab-backed inventory for the wdl101 lab. Pulls every ACTIVE EOS
# device with role "fm" or "ft" from the wdl1 region. Credentials come
# from DEVICE_USERNAME / DEVICE_PASSWORD env vars or CLI flags.

kind: dcfab
env: prod
region: wdl1
roles: [fm, ft]
platforms: [eos]
prefer_ip: ipv6
```

- [ ] **Step 2: Smoke-test the build path**

```bash
cd /Users/gmckee/projects/go-anta && go build -o bin/go-anta ./cmd/go-anta
./bin/go-anta inventory -i examples/wdl101-dcfab.yaml --device-username admin --device-password admin --transport gnmi 2>&1 | head -30
```

Expected: the `inventory` command lists devices fetched from dcfab. Will only work from a Tailscale-connected machine; from elsewhere it errors with a clear connect failure.

- [ ] **Step 3: If you have Tailscale connectivity, run a real nrfu**

```bash
./bin/go-anta nrfu -i examples/wdl101-dcfab.yaml -C examples/wdl101-catalog.yaml \
    --device-username admin --device-password admin \
    --transport gnmi --progress=false 2>&1 | tail -25
```

Expected: tests run against the dcfab-loaded inventory; results look identical to the static-inventory version.

- [ ] **Step 4: Commit**

```bash
cd /Users/gmckee/projects/go-anta && git add examples/wdl101-dcfab.yaml && git commit -m "$(cat <<'EOF'
docs: add wdl101 dcfab inventory example

Demonstrates the kind: dcfab source against the wdl1 region with role
filters. Mirrors the existing static wdl101-inventory.yaml so the two
sources can be swapped without changing the test catalog.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

---

## Final verification

- [ ] **Step 1: Build, vet, test everything**

```bash
cd /Users/gmckee/projects/go-anta && go build ./... && go vet ./... && go test ./...
```

Expected: clean.

- [ ] **Step 2: Run a representative static-inventory nrfu (regression check)**

```bash
./bin/go-anta nrfu -i examples/wdl101-inventory.yaml -C examples/wdl101-catalog.yaml --progress=false 2>&1 | tail -10
```

Expected: identical results to before this PR.

- [ ] **Step 3: If Tailscale-connected, run the dcfab equivalent**

```bash
./bin/go-anta nrfu -i examples/wdl101-dcfab.yaml -C examples/wdl101-catalog.yaml \
    --device-username admin --device-password admin \
    --transport gnmi --progress=false 2>&1 | tail -10
```

Expected: results look comparable (same devices, possibly same tests passing).

---

## Out-of-scope reminders

These follow naturally from this design and are good follow-up issues:

- **dcfab pagination** (`offset` traversal beyond 5000 devices).
- **Source response caching** (write resolved inventory to disk with a TTL).
- **Multi-source merge** in one run.
- **dcfab authentication** when the API adds it.
