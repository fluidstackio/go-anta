# gNMI Transport Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a gNMI/gRPC transport alongside the existing HTTPS eAPI transport for go-anta, so the same network tests can run against devices via either protocol.

**Architecture:** A new `GNMIDevice` type implements the existing `device.Device` interface. A new `device.New(cfg)` factory dispatches between `EOSDevice` and `GNMIDevice` based on `DeviceConfig.Transport`. Test impls in `tests/...` are unchanged because both transports normalize their `Output` to the same `map[string]interface{}` shape that EOS eAPI JSON produces.

**Tech Stack:** Go, openconfig/gnmic (`github.com/openconfig/gnmic/pkg/api`), openconfig/gnmi protobuf types (`github.com/openconfig/gnmi/proto/gnmi`).

**Reference design:** `docs/superpowers/specs/2026-05-12-gnmi-transport-design.md`

---

## File Structure

| Path | Action | Responsibility |
|---|---|---|
| `pkg/device/device.go` | Modify | Add `Transport` field to `DeviceConfig`; nothing else |
| `pkg/device/factory.go` | Create | `device.New(cfg)` dispatcher with default-port logic |
| `pkg/device/factory_test.go` | Create | Table-driven unit tests for dispatch |
| `pkg/device/gnmi.go` | Create | `GNMIDevice` implementing the `Device` interface |
| `pkg/device/gnmi_unwrap.go` | Create | Pure helper that strips the `{"<cmd>": {...}}` wrapper from gNMI JSON_IETF responses |
| `pkg/device/gnmi_unwrap_test.go` | Create | Unit tests for the unwrap helper |
| `pkg/device/gnmi_integration_test.go` | Create | Env-var-gated smoke test against a real device |
| `internal/cli/commands/nrfu.go` | Modify | Add `--transport` flag; call `device.New` |
| `internal/cli/commands/check.go` | Modify | Same |
| `cmd/debug/main.go` | Modify | Add `--transport gnmi` toggle |
| `go.mod` / `go.sum` | Modify | Add gnmic + gnmi proto deps |

### Why these boundaries

- The factory lives in its own file so the dispatch logic is easy to find and test in isolation from the transport implementations.
- `gnmi_unwrap.go` is pulled out as a pure function so it can be exhaustively tested without touching gRPC or a real device. This is the most error-prone part of the response handling.
- `gnmi.go` itself handles the gRPC lifecycle and uses `gnmi_unwrap.go`. Keeping the wrapper-stripping in a separate file keeps each file focused.

---

## Task 1: Add `Transport` field and `device.New` factory

**Files:**
- Modify: `pkg/device/device.go`
- Create: `pkg/device/factory.go`
- Create: `pkg/device/factory_test.go`

This task lays down the dispatch skeleton with the eAPI path only. `Transport == "gnmi"` returns an error here; Task 3 wires up gNMI itself.

- [ ] **Step 1: Write failing tests in `pkg/device/factory_test.go`**

```go
package device

import (
	"strings"
	"testing"
)

func TestNew_DispatchAndDefaults(t *testing.T) {
	tests := []struct {
		name        string
		cfg         DeviceConfig
		wantPort    int
		wantErrSub  string
		wantConcrete string
	}{
		{
			name:         "default transport is eapi",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1"},
			wantPort:     443,
			wantConcrete: "*device.EOSDevice",
		},
		{
			name:         "explicit eapi transport",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "eapi"},
			wantPort:     443,
			wantConcrete: "*device.EOSDevice",
		},
		{
			name:         "explicit port wins over default",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Port: 8443, Transport: "eapi"},
			wantPort:     8443,
			wantConcrete: "*device.EOSDevice",
		},
		{
			name:       "unknown transport errors",
			cfg:        DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "smtp"},
			wantErrSub: "unknown transport",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dev, err := New(tc.cfg)
			if tc.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotType := stringifyType(dev)
			if gotType != tc.wantConcrete {
				t.Errorf("concrete type: got %s, want %s", gotType, tc.wantConcrete)
			}
			// Verify the port was applied via the resulting device.
			if eos, ok := dev.(*EOSDevice); ok {
				if eos.Config.Port != tc.wantPort {
					t.Errorf("port: got %d, want %d", eos.Config.Port, tc.wantPort)
				}
			}
		})
	}
}

func stringifyType(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return "*device." + typeName(v)
}

func typeName(v interface{}) string {
	// Concrete type name without the package prefix; helper kept simple.
	switch v.(type) {
	case *EOSDevice:
		return "EOSDevice"
	default:
		return "Unknown"
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```
go test ./pkg/device/...
```

Expected: build failure (`undefined: New`) or test failure.

- [ ] **Step 3: Add `Transport` to `DeviceConfig`**

In `pkg/device/device.go`, add the field inside `DeviceConfig`:

```go
Transport      string            `yaml:"transport,omitempty" json:"transport,omitempty"`
```

Place it adjacent to `Insecure` for visibility.

- [ ] **Step 4: Implement the factory**

Create `pkg/device/factory.go`:

```go
package device

import "fmt"

// New constructs a Device of the configured transport. This is the
// recommended entry point for the CLI and inventory layers; concrete
// constructors (NewEOSDevice, NewGNMIDevice) are exposed for tests but
// callers should prefer New so a single switch governs default ports
// and validation.
func New(cfg DeviceConfig) (Device, error) {
	switch cfg.Transport {
	case "", "eapi":
		if cfg.Port == 0 {
			cfg.Port = 443
		}
		return NewEOSDevice(cfg), nil
	case "gnmi":
		return nil, fmt.Errorf("gnmi transport not yet implemented")
	default:
		return nil, fmt.Errorf("unknown transport %q (supported: eapi, gnmi)", cfg.Transport)
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

```
go test ./pkg/device/...
go vet ./...
```

Expected: PASS, no vet errors.

- [ ] **Step 6: Commit**

```
git add pkg/device/device.go pkg/device/factory.go pkg/device/factory_test.go
git commit -m "feat(device): add Transport field and device.New factory

DeviceConfig grows a Transport string. device.New(cfg) dispatches on it
(eapi default, gnmi placeholder). EOSDevice gets a port-443 default
applied centrally instead of in NewEOSDevice's constructor.

The CLI/inventory layers should call device.New rather than NewEOSDevice
directly; subsequent tasks will switch them over.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Add gnmic dependency

**Files:**
- Modify: `go.mod`, `go.sum`

No tests or implementation in this task — it isolates the dep-bump commit so later commits are reviewable without dependency noise.

- [ ] **Step 1: Add the dependency**

```
go get github.com/openconfig/gnmic@latest
go get github.com/openconfig/gnmi@latest
go mod tidy
```

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Commit**

```
git add go.mod go.sum
git commit -m "build: add openconfig/gnmic and openconfig/gnmi deps

For the upcoming gNMI transport. No code uses these yet; isolating
the dep bump as its own commit keeps the implementation diff focused.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Implement and test the JSON unwrap helper

**Files:**
- Create: `pkg/device/gnmi_unwrap.go`
- Create: `pkg/device/gnmi_unwrap_test.go`

The wrapper-stripping logic is the most error-prone part of response handling and is pure (bytes in, map out). We test it in isolation before touching gRPC.

- [ ] **Step 1: Write failing tests in `pkg/device/gnmi_unwrap_test.go`**

```go
package device

import (
	"reflect"
	"testing"
)

func TestUnwrapCLIResponse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		command string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "strips matching command-name wrapper",
			raw:     `{"show version":{"modelName":"DCS-7280","version":"4.34.4M"}}`,
			command: "show version",
			want:    map[string]interface{}{"modelName": "DCS-7280", "version": "4.34.4M"},
		},
		{
			name:    "leaves unwrapped JSON alone",
			raw:     `{"modelName":"DCS-7280","version":"4.34.4M"}`,
			command: "show version",
			want:    map[string]interface{}{"modelName": "DCS-7280", "version": "4.34.4M"},
		},
		{
			name:    "wrapper key that does not match command is preserved",
			raw:     `{"unexpected":{"x":1}}`,
			command: "show version",
			want:    map[string]interface{}{"unexpected": map[string]interface{}{"x": float64(1)}},
		},
		{
			name:    "multi-key top level is not unwrapped",
			raw:     `{"show version":{"x":1},"extra":2}`,
			command: "show version",
			want:    map[string]interface{}{"show version": map[string]interface{}{"x": float64(1)}, "extra": float64(2)},
		},
		{
			name:    "wrapper value that is not an object is preserved",
			raw:     `{"show version":"raw text"}`,
			command: "show version",
			want:    map[string]interface{}{"show version": "raw text"},
		},
		{
			name:    "invalid JSON returns error",
			raw:     `not json`,
			command: "show version",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unwrapCLIResponse([]byte(tc.raw), tc.command)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; result=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

```
go test ./pkg/device/ -run TestUnwrapCLIResponse -v
```

Expected: build error (`undefined: unwrapCLIResponse`).

- [ ] **Step 3: Implement the helper**

Create `pkg/device/gnmi_unwrap.go`:

```go
package device

import (
	"encoding/json"
	"fmt"
)

// unwrapCLIResponse decodes JSON_IETF bytes returned by an Arista gNMI
// origin=cli Get and strips the single-key command-name wrapper if
// present, so the resulting map matches the eAPI JSON shape exactly.
//
// Arista wraps CLI responses like:
//
//	{"show version": {"modelName": "...", ...}}
//
// while eAPI returns the inner object directly inside result[0]. Both
// transports should produce identical Output values so test impls can
// remain transport-agnostic.
//
// If the JSON has a single top-level key matching the expanded command,
// the inner object is returned. Otherwise the parsed object is returned
// as-is.
func unwrapCLIResponse(raw []byte, expandedCommand string) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse gNMI JSON_IETF response: %w", err)
	}
	if len(parsed) != 1 {
		return parsed, nil
	}
	inner, ok := parsed[expandedCommand]
	if !ok {
		return parsed, nil
	}
	innerMap, ok := inner.(map[string]interface{})
	if !ok {
		return parsed, nil
	}
	return innerMap, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

```
go test ./pkg/device/ -run TestUnwrapCLIResponse -v
go vet ./...
```

Expected: PASS, no vet errors.

- [ ] **Step 5: Commit**

```
git add pkg/device/gnmi_unwrap.go pkg/device/gnmi_unwrap_test.go
git commit -m "feat(device): add gNMI CLI response unwrap helper

Arista's gNMI origin=cli Get wraps the JSON response in a single-key
object: {\"show version\": {...actual data...}}. eAPI returns the inner
object directly. unwrapCLIResponse strips this wrapper when present so
both transports produce identical Output shapes for tests.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Skeleton `GNMIDevice` + Connect/Disconnect

**Files:**
- Create: `pkg/device/gnmi.go`
- Modify: `pkg/device/factory.go` (replace error stub with NewGNMIDevice call)
- Modify: `pkg/device/factory_test.go` (add gNMI dispatch cases)

This task implements `Connect` (probe with `show version`) and `Disconnect`. `Execute`/`ExecuteBatch` come in Task 5.

- [ ] **Step 1: Add gNMI dispatch cases to `pkg/device/factory_test.go`**

Append these cases to the `tests` slice in `TestNew_DispatchAndDefaults`:

```go
		{
			name:         "gnmi transport with default port",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "gnmi"},
			wantPort:     6030,
			wantConcrete: "*device.GNMIDevice",
		},
		{
			name:         "gnmi transport with explicit port",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "gnmi", Port: 9339},
			wantPort:     9339,
			wantConcrete: "*device.GNMIDevice",
		},
```

Extend `typeName` to recognize `*GNMIDevice`:

```go
func typeName(v interface{}) string {
	switch v.(type) {
	case *EOSDevice:
		return "EOSDevice"
	case *GNMIDevice:
		return "GNMIDevice"
	default:
		return "Unknown"
	}
}
```

Extend the port verification block in the test body so it inspects `*GNMIDevice`'s embedded `BaseDevice.Config.Port` too:

```go
			switch d := dev.(type) {
			case *EOSDevice:
				if d.Config.Port != tc.wantPort {
					t.Errorf("port: got %d, want %d", d.Config.Port, tc.wantPort)
				}
			case *GNMIDevice:
				if d.Config.Port != tc.wantPort {
					t.Errorf("port: got %d, want %d", d.Config.Port, tc.wantPort)
				}
			}
```

Replace the existing single-case `if eos, ok := dev.(*EOSDevice); ok { ... }` block with the switch above.

- [ ] **Step 2: Run tests to verify failure**

```
go test ./pkg/device/ -run TestNew_DispatchAndDefaults -v
```

Expected: build error (`undefined: GNMIDevice`).

- [ ] **Step 3: Create `pkg/device/gnmi.go` with the skeleton**

```go
package device

import (
	"context"
	"fmt"
	"time"

	gnmiapi "github.com/openconfig/gnmic/pkg/api"
	gnmitarget "github.com/openconfig/gnmic/pkg/api/target"

	"github.com/fluidstackio/go-anta/internal/logger"
)

// GNMIDevice implements the Device interface against an Arista EOS
// device using gNMI gRPC with origin=cli Get requests. The JSON shape
// inside the response matches eAPI exactly (after wrapper-stripping),
// so tests written against EOSDevice work unchanged against GNMIDevice.
type GNMIDevice struct {
	BaseDevice
	target *gnmitarget.Target
	cache  *CommandCache
}

// NewGNMIDevice constructs a gNMI-backed device. Callers should prefer
// device.New(cfg) which handles default ports and dispatch.
func NewGNMIDevice(config DeviceConfig) *GNMIDevice {
	if config.Port == 0 {
		config.Port = 6030
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	dev := &GNMIDevice{
		BaseDevice: BaseDevice{
			Config: config,
			State:  ConnectionStateClosed,
		},
	}
	if !config.DisableCache {
		dev.cache = NewCommandCache(128, 60*time.Second)
	}
	return dev
}

func (d *GNMIDevice) Connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.State == ConnectionStateConnected || d.State == ConnectionStateEstablished {
		return nil
	}

	d.State = ConnectionStateConnecting

	addr := fmt.Sprintf("%s:%d", d.Config.Host, d.Config.Port)
	opts := []gnmiapi.TargetOption{
		gnmiapi.Address(addr),
		gnmiapi.Username(d.Config.Username),
		gnmiapi.Password(d.Config.Password),
		gnmiapi.Timeout(d.Config.Timeout),
	}
	if d.Config.Insecure {
		opts = append(opts, gnmiapi.SkipVerify(true))
	}

	target, err := gnmiapi.NewTarget(opts...)
	if err != nil {
		d.State = ConnectionStateError
		return fmt.Errorf("build gNMI target for %s: %w", d.Config.Name, err)
	}
	if err := target.CreateGNMIClient(ctx); err != nil {
		d.State = ConnectionStateError
		return fmt.Errorf("dial gNMI for %s: %w", d.Config.Name, err)
	}
	d.target = target
	d.State = ConnectionStateConnected
	d.ConnectionTime = time.Now()

	// Probe with show version (matches EOSDevice.Connect behavior) so
	// IsEstablished() / HardwareModel() have the expected post-Connect
	// invariants. Execute itself does not yet exist; for now we just
	// transition state without populating Model.
	d.State = ConnectionStateEstablished
	logger.Infof("Successfully connected to %s via gNMI", d.Config.Name)
	return nil
}

func (d *GNMIDevice) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.State = ConnectionStateClosed
	if d.cache != nil {
		d.cache.Clear()
	}
	if d.target != nil {
		_ = d.target.Close()
		d.target = nil
	}
	return nil
}

// Execute and ExecuteBatch are added in subsequent tasks. Returning an
// explicit error here makes accidental use during development obvious.

func (d *GNMIDevice) Execute(ctx context.Context, cmd Command) (*CommandResult, error) {
	return nil, fmt.Errorf("GNMIDevice.Execute not yet implemented")
}

func (d *GNMIDevice) ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error) {
	return nil, fmt.Errorf("GNMIDevice.ExecuteBatch not yet implemented")
}

func (d *GNMIDevice) Refresh(ctx context.Context) error {
	return fmt.Errorf("GNMIDevice.Refresh not yet implemented")
}
```

- [ ] **Step 4: Wire the factory to use `NewGNMIDevice`**

In `pkg/device/factory.go`, replace the `"gnmi"` case body:

```go
	case "gnmi":
		if cfg.Port == 0 {
			cfg.Port = 6030
		}
		return NewGNMIDevice(cfg), nil
```

- [ ] **Step 5: Run tests to verify they pass**

```
go test ./pkg/device/ -run TestNew_DispatchAndDefaults -v
go build ./...
go vet ./...
```

Expected: PASS, clean build, no vet errors.

- [ ] **Step 6: Commit**

```
git add pkg/device/gnmi.go pkg/device/factory.go pkg/device/factory_test.go
git commit -m "feat(device): GNMIDevice skeleton with Connect/Disconnect

Creates the gRPC target via gnmic, marks state Established on successful
dial. Execute/ExecuteBatch/Refresh are stubs returning explicit errors
so accidental use during further development is obvious.

The factory dispatcher now wires gnmi -> NewGNMIDevice with port 6030
as the default.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Implement `GNMIDevice.Execute` (single command, JSON_IETF)

**Files:**
- Modify: `pkg/device/gnmi.go`

- [ ] **Step 1: Replace the `Execute` stub with the real implementation**

Imports to add at the top of `gnmi.go`:

```go
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
```

Replace the existing `Execute` method body:

```go
func (d *GNMIDevice) Execute(ctx context.Context, cmd Command) (*CommandResult, error) {
	d.mu.RLock()
	if d.State != ConnectionStateEstablished {
		d.mu.RUnlock()
		return nil, fmt.Errorf("device %s is not connected", d.Config.Name)
	}
	target := d.target
	d.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("device %s has no active gNMI target", d.Config.Name)
	}

	if cmd.UseCache && d.cache != nil {
		if cached := d.cache.Get(d.cacheKey(cmd)); cached != nil {
			cached.Cached = true
			return cached, nil
		}
	}

	start := time.Now()
	expanded := d.expandTemplate(cmd)

	encoding := "json_ietf"
	if cmd.Format == "text" {
		encoding = "ascii"
	}

	req, err := gnmiapi.NewGetRequest(
		gnmiapi.Path(fmt.Sprintf("cli:/%s", expanded)),
		gnmiapi.Encoding(encoding),
	)
	if err != nil {
		return nil, fmt.Errorf("build gNMI Get for %q: %w", expanded, err)
	}

	resp, err := target.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gNMI Get for %q: %w", expanded, err)
	}

	output, err := extractCLIOutput(resp, expanded, encoding)
	if err != nil {
		return nil, err
	}

	result := &CommandResult{
		Command:   cmd,
		Output:    output,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}

	if cmd.UseCache && d.cache != nil {
		d.cache.Set(d.cacheKey(cmd), result)
	}
	return result, nil
}

// extractCLIOutput pulls the first Update value out of a gNMI GetResponse
// and converts it to a shape compatible with EOSDevice.Execute's Output:
//
//   - JSON_IETF -> map[string]interface{} with the command-name wrapper
//     stripped (see unwrapCLIResponse).
//   - ASCII   -> string of the raw CLI output.
func extractCLIOutput(resp *gnmipb.GetResponse, expanded, encoding string) (interface{}, error) {
	if resp == nil || len(resp.Notification) == 0 || len(resp.Notification[0].Update) == 0 {
		return nil, fmt.Errorf("gNMI Get for %q returned no notifications", expanded)
	}
	val := resp.Notification[0].Update[0].Val
	if val == nil {
		return nil, fmt.Errorf("gNMI Get for %q returned nil TypedValue", expanded)
	}

	switch encoding {
	case "ascii":
		if s := val.GetAsciiVal(); s != "" {
			return s, nil
		}
		return "", nil
	case "json_ietf":
		raw := val.GetJsonIetfVal()
		if len(raw) == 0 {
			// Some commands without a JSON form may still return ASCII;
			// surface that gracefully rather than failing.
			if s := val.GetAsciiVal(); s != "" {
				return s, nil
			}
			return nil, fmt.Errorf("gNMI Get for %q returned empty JSON_IETF value", expanded)
		}
		return unwrapCLIResponse(raw, expanded)
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

func (d *GNMIDevice) expandTemplate(cmd Command) string {
	// Identical to EOSDevice.expandTemplate, copied here so the two
	// transports stay independent. If a third transport ever lands,
	// hoist this into BaseDevice.
	cmdStr := cmd.Template
	for key, value := range cmd.Params {
		placeholder := fmt.Sprintf("{%s}", key)
		cmdStr = strings.ReplaceAll(cmdStr, placeholder, fmt.Sprint(value))
	}
	return cmdStr
}

func (d *GNMIDevice) cacheKey(cmd Command) string {
	return fmt.Sprintf("%s|v=%s|r=%d|f=%s", d.expandTemplate(cmd), cmd.Version, cmd.Revision, cmd.Format)
}
```

Add `"strings"` to the import block.

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Run existing unit tests**

```
go test ./pkg/device/... -v
```

Expected: PASS (factory + unwrap tests still pass; `Execute` itself has no unit test yet — the integration smoke comes in Task 7).

- [ ] **Step 4: Commit**

```
git add pkg/device/gnmi.go
git commit -m "feat(device): implement GNMIDevice.Execute for single command

Builds a gNMI Get with origin=cli, encoding mapped from cmd.Format,
honors the device cache via the same cacheKey helper as the eAPI path,
and converts the response TypedValue to Output:

  - JSON_IETF -> map[string]interface{} via unwrapCLIResponse
  - ASCII     -> raw string (graceful fallback for JSON-less commands)

Tests written against EOSDevice now also pass on GNMIDevice provided
the device is reachable via gNMI.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Implement `GNMIDevice.ExecuteBatch`

**Files:**
- Modify: `pkg/device/gnmi.go`

Mirrors the eAPI ExecuteBatch contract (commit `d396639`): per-command Duration share, explicit Error on short responses, cache used for both reads and writes.

- [ ] **Step 1: Replace the `ExecuteBatch` stub**

```go
func (d *GNMIDevice) ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error) {
	d.mu.RLock()
	if d.State != ConnectionStateEstablished {
		d.mu.RUnlock()
		return nil, fmt.Errorf("device %s is not connected", d.Config.Name)
	}
	target := d.target
	d.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("device %s has no active gNMI target", d.Config.Name)
	}

	results := make([]*CommandResult, len(cmds))

	// Build the list of commands that actually need to hit the wire.
	type pending struct {
		index    int
		cmd      Command
		expanded string
		encoding string
	}
	var pendings []pending

	for i, cmd := range cmds {
		if cmd.UseCache && d.cache != nil {
			if cached := d.cache.Get(d.cacheKey(cmd)); cached != nil {
				cached.Cached = true
				results[i] = cached
				continue
			}
		}
		expanded := d.expandTemplate(cmd)
		encoding := "json_ietf"
		if cmd.Format == "text" {
			encoding = "ascii"
		}
		pendings = append(pendings, pending{
			index:    i,
			cmd:      cmd,
			expanded: expanded,
			encoding: encoding,
		})
	}

	if len(pendings) == 0 {
		return results, nil
	}

	// gNMI Get can carry multiple paths in a single request, but each
	// request takes one encoding. If callers mix formats inside a batch
	// we split into one request per encoding.
	byEncoding := map[string][]pending{}
	for _, p := range pendings {
		byEncoding[p.encoding] = append(byEncoding[p.encoding], p)
	}

	for encoding, group := range byEncoding {
		opts := []gnmiapi.GetRequestOption{gnmiapi.Encoding(encoding)}
		for _, p := range group {
			opts = append(opts, gnmiapi.Path(fmt.Sprintf("cli:/%s", p.expanded)))
		}
		req, err := gnmiapi.NewGetRequest(opts...)
		if err != nil {
			return nil, fmt.Errorf("build gNMI batch Get (%s): %w", encoding, err)
		}

		start := time.Now()
		resp, err := target.Get(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("gNMI batch Get (%s): %w", encoding, err)
		}
		perCmd := time.Since(start) / time.Duration(len(group))

		// Map response notifications back to the input order. We assume
		// the server returns notifications in the same order as the
		// requested paths, which is what Arista does in practice.
		for i, p := range group {
			result := &CommandResult{
				Command:   p.cmd,
				Duration:  perCmd,
				Timestamp: time.Now(),
			}

			if i >= len(resp.Notification) || len(resp.Notification[i].Update) == 0 {
				result.Error = fmt.Errorf("gNMI batch returned no response for %q", p.expanded)
				results[p.index] = result
				continue
			}

			val := resp.Notification[i].Update[0].Val
			if val == nil {
				result.Error = fmt.Errorf("gNMI batch returned nil TypedValue for %q", p.expanded)
				results[p.index] = result
				continue
			}

			output, err := extractTypedValue(val, p.expanded, p.encoding)
			if err != nil {
				result.Error = err
				results[p.index] = result
				continue
			}
			result.Output = output
			results[p.index] = result

			if p.cmd.UseCache && d.cache != nil {
				d.cache.Set(d.cacheKey(p.cmd), result)
			}
		}
	}

	return results, nil
}

// extractTypedValue pulls the value out of a single TypedValue using
// the same conventions as extractCLIOutput, but reused for batch paths.
func extractTypedValue(val *gnmipb.TypedValue, expanded, encoding string) (interface{}, error) {
	switch encoding {
	case "ascii":
		if s := val.GetAsciiVal(); s != "" {
			return s, nil
		}
		return "", nil
	case "json_ietf":
		raw := val.GetJsonIetfVal()
		if len(raw) == 0 {
			if s := val.GetAsciiVal(); s != "" {
				return s, nil
			}
			return nil, fmt.Errorf("gNMI returned empty JSON_IETF value for %q", expanded)
		}
		return unwrapCLIResponse(raw, expanded)
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}
```

Refactor `extractCLIOutput` (Task 5) to delegate to `extractTypedValue`:

```go
func extractCLIOutput(resp *gnmipb.GetResponse, expanded, encoding string) (interface{}, error) {
	if resp == nil || len(resp.Notification) == 0 || len(resp.Notification[0].Update) == 0 {
		return nil, fmt.Errorf("gNMI Get for %q returned no notifications", expanded)
	}
	val := resp.Notification[0].Update[0].Val
	if val == nil {
		return nil, fmt.Errorf("gNMI Get for %q returned nil TypedValue", expanded)
	}
	return extractTypedValue(val, expanded, encoding)
}
```

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 3: Run unit tests**

```
go test ./pkg/device/... -v
```

Expected: existing tests still PASS.

- [ ] **Step 4: Commit**

```
git add pkg/device/gnmi.go
git commit -m "feat(device): implement GNMIDevice.ExecuteBatch

One gNMI Get per encoding (callers can mix json/text in a batch and
we split). Response notifications are mapped back to input order by
index. Mirrors the eAPI batch contract: each result carries a per-
command share of wall time, short responses produce results with
Error set so callers never see nil slots.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Env-var-gated integration smoke test

**Files:**
- Create: `pkg/device/gnmi_integration_test.go`

This test runs only when `GO_ANTA_GNMI_HOST`, `GO_ANTA_GNMI_USER`, and `GO_ANTA_GNMI_PASS` are set. CI skips it; engineers with a lab device can run it locally.

- [ ] **Step 1: Create the integration test**

```go
package device

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestGNMIDevice_Integration(t *testing.T) {
	host := os.Getenv("GO_ANTA_GNMI_HOST")
	user := os.Getenv("GO_ANTA_GNMI_USER")
	pass := os.Getenv("GO_ANTA_GNMI_PASS")
	if host == "" || user == "" || pass == "" {
		t.Skip("set GO_ANTA_GNMI_HOST/USER/PASS to run gNMI integration smoke test")
	}

	dev, err := New(DeviceConfig{
		Name:      "smoke",
		Host:      host,
		Username:  user,
		Password:  pass,
		Transport: "gnmi",
		Insecure:  true,
		Timeout:   10 * time.Second,
	})
	if err != nil {
		t.Fatalf("device.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := dev.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer dev.Disconnect()

	if !dev.IsEstablished() {
		t.Fatal("expected IsEstablished after Connect")
	}

	result, err := dev.Execute(ctx, Command{Template: "show version"})
	if err != nil {
		t.Fatalf("Execute show version: %v", err)
	}
	m, ok := result.Output.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map output, got %T", result.Output)
	}
	if _, ok := m["modelName"]; !ok {
		t.Errorf("expected modelName in output, got keys: %v", keysOf(m))
	}
}

func keysOf(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```

- [ ] **Step 2: Run the test in skip mode (no env vars)**

```
go test ./pkg/device/ -run TestGNMIDevice_Integration -v
```

Expected: `--- SKIP: TestGNMIDevice_Integration` (env vars not set).

- [ ] **Step 3: Document how to run with a real device**

Append to the doc-comment at the top of `gnmi_integration_test.go`:

```go
// Run against a real device with:
//
//   GO_ANTA_GNMI_HOST=fc00:800f:f01::8 \
//   GO_ANTA_GNMI_USER=admin \
//   GO_ANTA_GNMI_PASS=admin \
//   go test ./pkg/device/ -run TestGNMIDevice_Integration -v
```

- [ ] **Step 4: Commit**

```
git add pkg/device/gnmi_integration_test.go
git commit -m "test(device): gNMI integration smoke test

Gated by GO_ANTA_GNMI_HOST/USER/PASS env vars so CI skips it. Engineers
with a lab device can run it locally to verify Connect + Execute end
to end and to surface any new shape divergences from eAPI early.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: `--transport` flag and device.New in `nrfu`

**Files:**
- Modify: `internal/cli/commands/nrfu.go`

- [ ] **Step 1: Add the flag**

In the `var ( ... )` block at the top of the file, add:

```go
	transport      string
```

In the `init()` function, register the flag:

```go
	NrfuCmd.Flags().StringVar(&transport, "transport", "", "transport for device connections (eapi, gnmi). Overrides per-device YAML transport when set.")
```

- [ ] **Step 2: Replace the `device.NewEOSDevice` call with `device.New`**

Find the device-connect loop (currently around line 158-169 in `nrfu.go`):

```go
	for _, devConfig := range inv.Devices {
		dev := device.NewEOSDevice(devConfig)
		...
	}
```

Replace with:

```go
	for _, devConfig := range inv.Devices {
		if transport != "" {
			devConfig.Transport = transport
		}
		dev, err := device.New(devConfig)
		if err != nil {
			logger.Errorf("Failed to construct device %s: %v", devConfig.Name, err)
			continue
		}
		...
	}
```

(Keep the rest of the loop body — `dev.Connect`, `deviceList = append(...)`, `defer dev.Disconnect()` — unchanged.)

- [ ] **Step 3: Verify build**

```
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Smoke-test with an eapi inventory (no behavior change expected)**

```
./bin/go-anta nrfu -i path/to/some/inventory.yaml -C path/to/catalog.yaml --dry-run
```

(Or whatever's representative.) Expected: identical output to before.

- [ ] **Step 5: Commit**

```
git add internal/cli/commands/nrfu.go
git commit -m "feat(cli): --transport flag on nrfu

Adds the per-run override. Switches the device-connect loop to
device.New so transport dispatch happens in one place. Per-device
YAML transport still wins when the flag is empty.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: `--transport` flag and device.New in `check`

**Files:**
- Modify: `internal/cli/commands/check.go`

- [ ] **Step 1: Add the flag**

In the `var ( ... )` block:

```go
	checkTransport string
```

In `init()`:

```go
	CheckCmd.Flags().StringVar(&checkTransport, "transport", "", "transport for device connections (eapi, gnmi). Overrides per-device YAML transport when set.")
```

- [ ] **Step 2: Replace `device.NewEOSDevice` with `device.New`**

Find the device construction in `check.go` (in the connect loop). Replace `device.NewEOSDevice(devConfig)` with:

```go
		if checkTransport != "" {
			devConfig.Transport = checkTransport
		}
		dev, err := device.New(devConfig)
		if err != nil {
			fmt.Printf("  %s: failed to construct device: %v\n", devConfig.Name, err)
			continue
		}
```

- [ ] **Step 3: Verify build**

```
go build ./...
go vet ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```
git add internal/cli/commands/check.go
git commit -m "feat(cli): --transport flag on check

Mirrors nrfu's transport override. Switches to device.New for
consistency.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: `--transport` toggle in `cmd/debug`

**Files:**
- Modify: `cmd/debug/main.go`

- [ ] **Step 1: Read the current debug command**

```
cat cmd/debug/main.go
```

Note the structure (hardcoded IP, eAPI client, etc.). The goal is to switch to `device.New` and accept a `--transport` flag.

- [ ] **Step 2: Refactor to use `device.New`**

Replace the contents of `cmd/debug/main.go` with a version that uses `flag` (or cobra-style, matching the existing style of the file) to accept `--host`, `--user`, `--pass`, `--transport`, and `--port`, then calls `device.New` to construct the device and runs a `show version`.

A minimal sketch (adapt to match the file's existing style):

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fluidstackio/go-anta/pkg/device"
)

func main() {
	host := flag.String("host", "", "device host (IP or hostname)")
	user := flag.String("user", "admin", "username")
	pass := flag.String("pass", "", "password (or set DEVICE_PASSWORD env var)")
	port := flag.Int("port", 0, "device port (default 443 for eapi, 6030 for gnmi)")
	transport := flag.String("transport", "eapi", "transport: eapi or gnmi")
	insecure := flag.Bool("insecure", true, "skip TLS verification")
	flag.Parse()

	if *host == "" {
		fmt.Fprintln(os.Stderr, "--host is required")
		os.Exit(2)
	}
	if *pass == "" {
		*pass = os.Getenv("DEVICE_PASSWORD")
	}

	dev, err := device.New(device.DeviceConfig{
		Name:      "debug",
		Host:      *host,
		Port:      *port,
		Username:  *user,
		Password:  *pass,
		Transport: *transport,
		Insecure:  *insecure,
		Timeout:   10 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "device.New: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := dev.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Connect: %v\n", err)
		os.Exit(1)
	}
	defer dev.Disconnect()

	result, err := dev.Execute(ctx, device.Command{Template: "show version"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execute: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Output: %+v\n", result.Output)
}
```

Discard whatever curl-leaking debug code was there before (the previous review flagged that file as a candidate for deletion or refactor).

- [ ] **Step 3: Verify build**

```
go build ./cmd/debug
```

Expected: produces a working binary.

- [ ] **Step 4: Smoke-test (manual, with a reachable device)**

```
./debug --host fc00:800f:f01::8 --user admin --pass admin --transport gnmi --port 6030
./debug --host <eapi-host> --user admin --pass <pw>
```

Expected: both print the parsed `show version` output.

- [ ] **Step 5: Commit**

```
git add cmd/debug/main.go
git commit -m "feat(debug): support --transport gnmi via device.New

Replaces the previous hand-rolled eAPI debug code (which printed
credentials in curl-friendly form) with a thin wrapper around
device.New so the same binary can probe either transport. Useful for
verifying gNMI connectivity outside the full nrfu flow.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Final verification

- [ ] **Step 1: Build and vet everything**

```
go build ./...
go vet ./...
go test ./pkg/device/...
```

Expected: clean build, no vet errors, unit tests pass (integration test skips).

- [ ] **Step 2: Run a representative nrfu against an eapi-configured inventory**

```
./bin/go-anta nrfu -i <existing-inventory.yaml> -C <existing-catalog.yaml> --dry-run
```

Expected: identical output to before the feature.

- [ ] **Step 3: If a gNMI-reachable device is available, run nrfu against it**

```
./bin/go-anta nrfu -i <inventory-with-gnmi-device.yaml> -C <catalog.yaml> --transport gnmi
```

Expected: tests run successfully via gNMI; results should look the same as via eAPI for any commands that exist in both forms.

---

## Out of scope reminder

These are intentionally not implemented in this plan and remain follow-ups:

- mTLS support (would add `ClientCert`/`ClientKey` to `DeviceConfig`).
- gNMI Capabilities probe on Connect (the existing dial-then-Get already validates reachability).
- Large-ASN string-vs-number normalization (open question from the spike — re-test only if an actual test fails).
- gNMI Subscribe / Set.

If a follow-up surfaces during implementation, append a note to the design doc rather than expanding this plan.
