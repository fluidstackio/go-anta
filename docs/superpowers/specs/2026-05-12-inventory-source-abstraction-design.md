# Design: Pluggable inventory sources for go-anta

**Status:** Approved
**Date:** 2026-05-12
**Author:** gavin@fluidstack.io (with Claude)

## Goal

Replace go-anta's two hard-coded inventory paths (static YAML and Netbox) with a small `Source` interface that lets new inventory backends be added by dropping a single Go file. First new source is **dcfab**, FluidStack's datacenter-fabric GraphQL API.

## Non-goals

- Multi-source merging in one run (one inventory file → one source).
- dcfab pagination beyond a single `limit: 5000` page. Larger fleets need filter narrowing or a follow-up.
- Caching source responses to disk between runs.
- Dynamic re-fetch / inventory hot-reload during a run.
- Changing how device credentials are supplied (env + CLI flag stays).

## Architecture

```
pkg/inventory/
├── inventory.go          Inventory type + Validate + Filters (mostly unchanged)
├── source.go             NEW: Source interface + global Registry + LoadSource
├── source_file.go        NEW: FileSource (the existing static-YAML logic)
├── source_netbox.go      RENAMED from netbox.go: NetboxSource wraps existing client
├── source_dcfab.go       NEW: DcfabSource — HTTP GET against dcfab GraphQL
└── netbox_client.go      SPLIT from netbox.go: low-level HTTP/JSON client
```

The `Inventory` result type, the `FilterByTags`/`FilterByNames`/`FilterByLimit` methods, and the `expandNetworks`/`expandRanges` helpers are reused as-is. The `Source` interface only governs **how an Inventory is produced**.

### Source interface

```go
// Source loads an Inventory from a configured backend. Implementations are
// registered at init() time via RegisterSource and selected by the `kind:`
// field in an inventory YAML file.
type Source interface {
    // Kind returns the registered name (for diagnostics: "file", "netbox", "dcfab").
    Kind() string
    // Load fetches devices and returns an Inventory. The caller's ctx is honored
    // for cancellation. Credentials are NOT populated here — see ApplyDefaults.
    Load(ctx context.Context) (*Inventory, error)
}
```

### Registration

Each implementation registers in `init()`:

```go
inventory.RegisterSource("dcfab", func(raw map[string]any) (Source, error) {
    var cfg DcfabConfig
    if err := mapstructure.Decode(raw, &cfg); err != nil { ... }
    return &DcfabSource{cfg: cfg}, nil
})
```

The registry is a `sync.RWMutex`-protected `map[string]Factory`, identical in shape to `pkg/test.Registry`. A factory takes the parsed YAML (`map[string]any`) and returns a constructed `Source`.

### Top-level loader

```go
// LoadSource reads a YAML inventory file, picks the source kind, and returns
// a constructed Source. Callers then call src.Load(ctx).
//
// A YAML file with no `kind:` field is treated as kind: file for backward
// compatibility with existing static inventories.
func LoadSource(path string) (Source, error)
```

CLI commands collapse from ~110 lines of per-command Netbox parsing to:

```go
src, err := inventory.LoadSource(path)
if err != nil { return err }
inv, err := src.Load(ctx)
if err != nil { return err }
inv = inv.ApplyDefaults(defaults)
```

### Credential overlay

Sources return `*Inventory` with `Username`/`Password` empty (Netbox and dcfab don't expose credentials; only static YAML does). A post-load step overlays per-run credentials onto every device that doesn't already have them:

```go
type DeviceDefaults struct {
    Username  string
    Password  string
    Transport string  // "eapi" or "gnmi"
    Insecure  bool
    Plaintext bool
    Port      int
}

// ApplyDefaults overlays defaults onto each device, but only for fields that
// are currently empty / zero. Per-device YAML values always win.
func (i *Inventory) ApplyDefaults(d DeviceDefaults) *Inventory
```

This is the single overlay point for both the existing `DEVICE_USERNAME/PASSWORD` env vars and the `--transport`/`--device-username`/`--device-password` CLI flags.

## File source

```yaml
# Backward compat: no `kind:` field defaults to file.
kind: file    # optional

devices:
  - name: leaf1
    host: 192.0.2.10
    transport: gnmi
    port: 6030

networks:
  - network: 192.0.2.0/24
    ...

ranges:
  - start: 192.0.2.10
    end: 192.0.2.20
    ...
```

Implementation is the existing `LoadInventory` logic, refactored to be the `FileSource.Load` method.

## Netbox source

```yaml
kind: netbox
url: https://netbox.example.com   # or env NETBOX_URL
token: secret                     # or env NETBOX_TOKEN (preferred)
insecure: false
query:
  site: wdl101
  role: leaf
  platform: eos
  tags: [production]
```

Implementation is the existing `LoadFromNetbox` logic refactored into `NetboxSource.Load`. The 110-line per-command query parser in `nrfu.go`/`check.go`/`inventory.go` is replaced by a single struct decoding from the YAML — `check.go`'s silently missing keys (`site_id`, `role_id`, `device_type_id`) are fixed in passing because all three commands now share the same code path.

## dcfab source

```yaml
kind: dcfab
env: prod                    # optional; default prod
region: wdl1                 # required
roles: [fm, ft]              # optional filter; empty = all roles
platforms: [eos]             # optional filter; empty = all platforms
endpoint: https://...        # optional override (default derived from env)
prefer_ip: ipv6              # optional; default ipv6
```

### GraphQL query (constructed at Load time)

```graphql
{
  region(name: "wdl1") {
    devices(implementation: ACTIVE, roles: ["fm","ft"], platforms: ["eos"], limit: 5000) {
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
}
```

Issued as a single HTTP GET to `{endpoint}/v1alpha1/query?query={url-encoded}` via `net/http`. No authentication (Tailscale-gated). Default endpoint:

- `env: prod` → `https://dcfab.fluidstack.io`
- `env: dev`  → `https://dcfab.fluidstack.xyz`

### Device → DeviceConfig mapping

| dcfab field | DeviceConfig field | Notes |
|---|---|---|
| `name` | `Name` | |
| `managementInterface.addresses[]` | `Host` | First address matching `prefer_ip` (default IPv6); fall back to the other family. If none, the device is skipped with a logger.Warn. |
| `role`, `platform`, `region` | `Tags` | Joined as `["role:fm", "platform:eos", "region:wdl1"]` so existing `--tags` filters work. |
| (none) | `Port`, `Username`, `Password`, `Transport`, `Insecure`, `Plaintext` | Populated by `ApplyDefaults` after Load. |

### Pagination

Initial implementation uses `limit: 5000` matching the skill's complexity-limit guidance. If exactly 5000 devices come back, return an error suggesting filter narrowing (or paginate in a follow-up).

## CLI changes

Existing flags continue to work; their behavior is now expressed as overrides on the YAML-declared source.

| Flag | Behavior |
|---|---|
| `--inventory path.yaml` | Load that YAML; `kind:` field selects the source. |
| `--source dcfab\|netbox\|file` | Override the YAML `kind:` field. The rest of the YAML body is still passed to the factory, so any required fields for the overriding kind must be present — otherwise the factory errors with a clear message. |
| `--netbox-url URL` | If no `--inventory`, synthesize a NetboxSource from CLI flags + env (current behavior). |
| `--netbox-query "..."` | Apply to NetboxSource (works whether source comes from YAML or flags). |
| `--region wdl1` | NEW: applies to DcfabSource. |
| `--roles fm,ft` | NEW: applies to DcfabSource. |
| `--device-username` / `--device-password` | Unchanged; fed into `ApplyDefaults`. |
| `--transport` | Unchanged; fed into `ApplyDefaults`. |

The Netbox-from-flags-only path stays for backward compatibility but is implemented as "synthesize a YAML map from flags, call the registry, return the resulting Source."

## Migration order

Each step is one commit and the codebase stays green throughout.

1. Add `Source` interface, Registry, `LoadSource`, and `FileSource`. `LoadInventory(path)` becomes a thin wrapper that calls `LoadSource` → `FileSource.Load`. Existing static YAMLs work unchanged. Add a small unit test.
2. Refactor Netbox into `NetboxSource`. `LoadFromNetbox` and `LoadNetboxInventory` stay as thin back-compat wrappers. Move HTTP/JSON details into `netbox_client.go`.
3. Add `DcfabSource` with full unit tests for query construction and device→DeviceConfig mapping.
4. Replace per-command Netbox parsing in `nrfu.go`/`check.go`/`inventory.go` with a single shared loader helper. Verify `check.go`'s missing-keys bug is fixed by adding a test asserting the YAML round-trips.
5. Add `--source`, `--region`, `--roles` CLI flags. Update `--help` text.
6. Add a `examples/wdl101-dcfab.yaml` to mirror the existing `examples/wdl101-inventory.yaml`; verify nrfu works end-to-end via dcfab.

## Tests

- `pkg/inventory/source_test.go` — Registry + LoadSource + back-compat for kind-less YAML.
- `pkg/inventory/source_file_test.go` — existing inventory tests retargeted at `FileSource`.
- `pkg/inventory/source_netbox_test.go` — query construction + mock HTTP responses.
- `pkg/inventory/source_dcfab_test.go` — GraphQL query construction (golden-file the URL-encoded string), device→DeviceConfig mapping (table-driven on management-address shapes), pagination-cap error.
- Integration smoke: `examples/wdl101-dcfab.yaml` runs against the live API when run from a Tailscale-connected machine.

## Risks

- **dcfab schema drift.** dcfab is an internal API; the schema is captured in the local skill resources. If `ActiveDevice.managementInterface` is renamed or restructured, the unit tests that parse the response will fail and we'll need to update the typed struct. Mitigation: keep the response struct narrow (only the fields we need), so unrelated additions don't break us.
- **Pagination cap.** Hitting exactly 5000 devices is the only red flag; we error rather than silently truncate.
- **IPv6 address preference might surprise eAPI users.** dcfab returns both IPv4 and IPv6 management addresses for a device; preferring IPv6 matches gNMI lab use but might be wrong for eAPI deployments on IPv4. Configurable via `prefer_ip:` — default subject to user override.
- **Backward compat of YAML files without `kind:`.** Any existing inventory that doesn't have a `kind:` field is treated as `file`. We rely on no existing static inventory having a top-level `kind:` key that means something else. Verified against `examples/*.yaml`.

## Out-of-scope follow-ups

These naturally fall out of this design and are good follow-up issues:

1. **dcfab pagination** — when a region has >5000 devices, paginate via `offset`.
2. **Source caching** — write the resolved Inventory to a local file with a TTL so repeated runs against the same source don't hit the API each time.
3. **Inventory merge** — combine multiple Sources in one run (e.g., dcfab + a small static override file).
4. **Source-specific authentication** — when dcfab adds auth, plumb tokens through the same env-fallback pattern Netbox uses.
