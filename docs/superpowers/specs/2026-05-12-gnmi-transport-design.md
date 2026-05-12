# Design: gNMI transport for go-anta

**Status:** Approved
**Date:** 2026-05-12
**Author:** gavin@fluidstack.io (with Claude)

## Goal

Add a gNMI (gRPC Network Management Interface) transport as an alternative to the existing HTTPS eAPI transport. Users should be able to send the same CLI commands via gNMI's `origin=cli` Get path and have the results flow through the existing test framework unchanged.

Reference invocation that motivates this:

```
gnmi -addr 192.0.2.214:6030 -username admin -password ***** \
    get origin=cli "show version"
```

## Non-goals

- gNMI Subscribe (streaming telemetry). go-anta is request/response.
- gNMI Set (configuration push). go-anta is read-only.
- mTLS (deferred; gnmic supports it, can be added later).
- Mixed-transport `ExecuteBatch` calls. A single device uses a single transport.

## Architecture

```
pkg/device/
├── device.go         interface + BaseDevice + DeviceConfig (gets new Transport field)
├── factory.go        NEW: device.New(cfg) dispatches on cfg.Transport
├── eos.go            existing eAPI client — unchanged behavior
├── gnmi.go           NEW: GNMIDevice implements Device interface
└── cache.go          existing — shared between transports
```

The existing `Device` interface is the abstraction boundary. Both `EOSDevice` and `GNMIDevice` implement it; everything above the device layer (test runner, tests themselves, reporters, CLI) is transport-agnostic.

## Transport selection

`DeviceConfig` gets a new field:

```go
Transport string `yaml:"transport,omitempty" json:"transport,omitempty"`
```

Values: `"eapi"` (default when empty) or `"gnmi"`. Per-device in inventory:

```yaml
devices:
  - name: leaf1
    host: 192.0.2.214
    transport: gnmi
    port: 6030       # optional; defaults below if omitted
    username: admin
    password: ******
  - name: spine1
    host: 192.0.2.10
    # transport omitted = eapi
    username: admin
    password: ******
```

A new CLI flag on `nrfu` and `check`:

```
--transport {eapi|gnmi}
```

When set, the flag overrides each device's YAML `transport`. When unset, YAML wins. This supports both mixed-fleet inventories (per-device YAML) and quick experiments (CLI override for a whole run).

**Default ports:**
- `eapi` → 443 (unchanged)
- `gnmi` → 6030 (Arista default)

A single entry point dispatches:

```go
// device.New constructs a Device for the configured transport. The CLI
// and inventory layers call this exclusively; concrete constructors
// (NewEOSDevice, NewGNMIDevice) become package-internal.
func New(cfg DeviceConfig) (Device, error)
```

## gNMI client — request/response

**Library:** `github.com/openconfig/gnmic/pkg/api` for the higher-level client (target/path helpers, auth wiring); `github.com/openconfig/gnmi/proto/gnmi` for typed messages.

**Connect:**
1. Build a gnmic Target with address, username, password, TLS config (InsecureSkipVerify gated by `cfg.Insecure`, mirroring eAPI).
2. Dial gRPC.
3. Issue a probe `show version` Get to verify reachability and populate `BaseDevice.Model` from `modelName`.
4. Transition state to `ConnectionStateEstablished`.

The probe mirrors `EOSDevice.Connect` so semantics around `IsEstablished()` stay identical for callers.

**Execute:** build a single-path Get request:
- `Path[0].Origin = "cli"`
- `Path[0].Elem = [{Name: <expanded command string>}]`
- `Encoding` chosen from `cmd.Format`:
  - `""` or `"json"` → `JSON_IETF`
  - `"text"` → `ASCII`

The response `Notification.Update[].Val` is a `TypedValue`. For `JSON_IETF` it carries JSON bytes; we `json.Unmarshal` into `map[string]interface{}` and assign to `CommandResult.Output`. For `ASCII` we assign the raw string. Either way, the existing `Output interface{}` contract is preserved — tests that do `cmdResult.Output.(map[string]any)` continue to work.

`cmd.Params` template-expansion is handled by the existing `expandTemplate` helper. `cmd.Version` and `cmd.Revision` are not meaningful on the gNMI CLI path; ignored with no error.

**ExecuteBatch:** one gNMI Get with N paths (one per command). gNMI returns one Notification per path; results are assembled in order matching the input slice. Mirrors the eAPI batch contract including:
- `Duration` set on each result (per-command share of the batch wall time).
- Short responses (fewer Notifications than paths) populate the corresponding result with `Error` set, so callers don't see nil slots.

**Disconnect:** close the gRPC `ClientConn`; clear the cache; transition state.

**Cache:** reuses `CommandCache` and `cacheKey(cmd)` exactly as the eAPI path. The cache key already includes params/version/revision/format, so the only constraint is that two transports don't share a cache (they don't — each device owns its own).

**Mutex / concurrency / panic safety:** identical patterns to `EOSDevice`. Embeds `BaseDevice` and uses its `mu` for state transitions. All `Execute`/`ExecuteBatch` paths take `RLock` to validate state and release before the network call.

## Auth and TLS

- Username/password via gRPC per-RPC metadata (`Authorization`-style; gnmic handles this).
- TLS is on by default. `cfg.Insecure=true` flips `InsecureSkipVerify`.
- mTLS (`ClientCert`/`ClientKey`) is **out of scope** for this iteration. Add later as additional `DeviceConfig` fields.

## Tests and validation

- **No test impl changes.** The `tests/...` packages call `dev.Execute(ctx, cmd)` and parse `cmdResult.Output` as a map. The gNMI transport returns the same shape, so every existing test runs unchanged on a gNMI device.
- **New unit test:** `pkg/device/factory_test.go` — table-driven test that `device.New` dispatches each `Transport` value to the right concrete type with the right default port.
- **New integration smoke (optional):** `pkg/device/gnmi_integration_test.go` — gated by `GO_ANTA_GNMI_HOST`/`USER`/`PASS` env vars; `t.Skip` if unset. Probes `show version` and asserts non-empty Model.
- **Debug tool:** extend `cmd/debug` to accept `--transport gnmi` so connectivity can be sanity-checked outside the full nrfu flow.

## Out-of-scope errors and follow-ups

The following are intentionally deferred. Each is small enough to be its own follow-up PR.

1. **mTLS support** — add `ClientCert`/`ClientKey` to `DeviceConfig`; thread into gnmic.
2. **gNMI capabilities probe** — call `Capabilities()` on connect to confirm the device supports `origin=cli`, with a clear error if it doesn't.
3. **Capability advertisement in test results** — some tests assume eAPI-specific JSON shapes; if Arista's `origin=cli` JSON ever diverges from eAPI JSON, surface a per-test transport requirement. Out of scope until a divergence appears.
4. **Per-transport timeouts** — currently `DeviceConfig.Timeout` covers the whole request. gRPC has its own timeout semantics; defer until we see real-world latency profiles.

## Risks

- **Arista's `origin=cli` JSON encoding behavior is the assumption that makes this work cheaply.** If Arista returns different JSON for some commands via gNMI than via eAPI, tests will mis-parse. Mitigation: integration-test a handful of representative commands before claiming the transport is production-ready.
- **gnmic dependency tree is large** (the user picked the higher-level wrapper over the bare openconfig/gnmi client). Adds a few hundred KB of transitive deps. Acceptable trade-off for the cleaner API; revisit if go.mod gets unwieldy.
- **Connection pooling differences.** gRPC uses a single multiplexed HTTP/2 connection per `ClientConn`, naturally pooling requests. Different model from the per-device `http.Transport` pool of the eAPI client, but functionally similar from the caller's view.

## Implementation order

1. `DeviceConfig.Transport` field + `device.New` factory + table-driven test (no behavior change).
2. `GNMIDevice` skeleton with `Connect`/`Disconnect` and a single-command `Execute` (JSON encoding only).
3. `ExecuteBatch`.
4. ASCII (text) encoding path.
5. CLI `--transport` flag on `nrfu` and `check`.
6. `cmd/debug --transport gnmi` toggle.

Each step is a separate commit, build-clean at every step.
