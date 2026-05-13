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

The response `Notification.Update[].Val` is a `TypedValue`. For `JSON_IETF` it carries JSON bytes; we `json.Unmarshal` into `map[string]interface{}` and assign to `CommandResult.Output`. For `ASCII` we assign the raw string.

**Wrapper-stripping (validated by spike on 2026-05-12):** Arista's gNMI server returns JSON_IETF bytes wrapped under a single key matching the CLI command string, e.g. `{"show version": {"modelName": "...", ...}}`. The eAPI client already extracts the equivalent inner object (`result[0]`) before assigning to `Output`. To keep the two transports' `Output` shapes identical, `GNMIDevice.Execute` strips this one extra level:

```go
var raw map[string]interface{}
_ = json.Unmarshal(jsonIetfVal, &raw)
if len(raw) == 1 {
    if inner, ok := raw[d.expandTemplate(cmd)].(map[string]interface{}); ok {
        raw = inner
    }
}
result.Output = raw
```

After this strip, every test impl in `tests/...` that walks `cmdResult.Output.(map[string]any)["vrfs"][...]` etc. works without modification.

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

- **JSON shape parity (largely retired by 2026-05-12 spike).** A live test against an EOS 4.34.4M device showed that `origin=cli` with `JSON_IETF` encoding returns the same EOS-native JSON shape as eAPI (inside a single-key wrapper which the transport strips — see Execute above). Field names, nesting, and value types matched for `show version` and `show ip bgp summary`. Two open questions remain to fully retire this risk:
  1. **Large ASN encoding.** The spike showed `"asn": "4204100000"` as a JSON **string** in gNMI output. eAPI may quote this differently (TBD; run the equivalent eAPI curl to compare). If they differ, tests that read `peerInfo["asn"].(float64)` would fail on the gNMI transport. Mitigation if observed: a small normalizer that re-types numeric-looking strings, applied per-command or globally.
  2. **Commands without JSON representation.** A handful of EOS commands have no `| json` form; those return ASCII even when JSON is requested. Currently no in-scope test depends on such commands, but the transport should gracefully degrade (return the ASCII string in `Output`) rather than error.
- **IPv6 link-local zone-index limitation.** gnmic does not correctly handle IPv6 link-local addresses with a zone identifier (`fe80::1%eth0`) due to URL re-encoding; tracked upstream as [openconfig/gnmic#796](https://github.com/openconfig/gnmic/issues/796). Standard global/site-local IPv6 (e.g. `fc00:800f:f01::8`) works fine. Datacenter fabric use is unaffected.
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
