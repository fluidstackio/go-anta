# Code Review Backlog

This file tracks open issues from the 2026-05-11 deep code review. It is a working document — update as items are fixed or new findings emerge.

## How this was produced

Six parallel review agents covered the whole codebase (~23k LOC) by area:

1. Device/transport (`pkg/device/`)
2. Test framework (`pkg/test/`)
3. Config / inventory / Netbox (`pkg/config/`, `pkg/inventory/`, `pkg/platform/`, `internal/logger/`)
4. CLI / reporters / cmd (`internal/cli/`, `cmd/`, `pkg/reporter/`)
5. Test impls A (`tests/hardware/`, `tests/system/`, `tests/security/`, `tests/interfaces/`, `tests/software/`, `tests/services/`, `tests/logging/`, `tests/connectivity/`)
6. Test impls B (`tests/routing/`, `tests/vxlan/`, `tests/evpn/`, `tests/stp/`, `tests/vlan/`)

22 fixes have already shipped (commits below). The remaining items live in this doc.

## Fixes already shipped

| # | Commit | What it fixes |
|---|--------|---------------|
| 1 | `eb572d2` | Stop logging device passwords in plaintext (eAPI curl-cmd dump) |
| 2 | `2175ee6` | `VerifyInterfaceErrors` `check_all` mode never failed (shadowed `failures`) |
| 3 | `3db1fd4` | `expandRanges` infinite-loop guards (reverse / cross-family / oversize) |
| 4 | `ef19c88` | Netbox pagination silently truncated when `Limit` set |
| 5 | `b81d18e` | `nrfu` returned `ErrTestsFailed` sentinel; main.go maps exit codes (0/1/2) |
| 6 | `2ed4051` | Panic recovery in test workers (one bad test no longer truncates run) |
| 7 | `4d833ec` | Cache key collision — included params/version/revision/format |
| 8 | `db7620f` | `BaseDevice` accessor data races (moved mutex to BaseDevice) |
| 9 | `aa4d4bb` | `CommandCache` atomic counters (race on hits/misses under RLock) |
| 10 | `7550d16` | HTTP keep-alives + pool + `CloseIdleConnections` on Disconnect |
| 11 | `b9269c7` | 14 BGP tests' broken `Output.(string)` → `decodeOutput` helper |
| 12 | `a02baae` | `VerifyEVPNType2Route` was a no-op stub returning PASS → TestError "not implemented" |
| 13 | `58626c5` | Added `test.AsMap`/`test.AsSlice` helpers |
| 14 | `136edd7` | Silent-PASS: VerifyCoredump, VerifyAgentLogs, VerifyMaintenance |
| 15 | `586d1f7` | Silent-PASS: 6 hardware tests (drops, PCIe, linecards, env cooling/power, chassis health, inventory) |
| 16 | `0201b25` | `VerifyMlagStatus` distinguished parse-fail from disabled |
| 17 | `ef03c54` | SIGINT/SIGTERM handling in all three CLI commands |
| 18 | `aa38cc8` | Silent-PASS: 7 AAA tests + 4 ACL count tests |
| 19 | `86dc17d` | Silent-PASS: interfaces status/errors/utilization + flash/filesystem util + NTP/DNS |
| 20 | `7b41d51` | `VerifyBGPPeerCount` / `VerifyBGPSpecificPeers` vacuous PASS |
| 21 | `d396639` | `ExecuteBatch` Duration + nil-slot handling |
| 22 | `4a55715` | Secret redaction (`String`/`MarshalJSON`) for DeviceConfig, NetboxConfig, Network/RangeDefinition |

All 22 commits are independent and reviewable. Build + `go vet` clean throughout.

## Remaining work — prioritized

Each item below has a concrete file:line anchor and the rough shape of the fix. Severity reflects production impact, not implementation difficulty.

### CRITICAL — production correctness gaps

#### R1. BGP protocol-correctness bugs (5 sub-items)
False positives or always-fails because the test doesn't actually check what its docs claim.

- **AFI/SAFI ignored in `VerifyBGPPeerCount` and `VerifyBGPPeersHealth`** (`tests/routing/bgp.go:506-578` and `bgp.go:692-746`). The struct captures AFI/SAFI but the loop counts all peers in the VRF regardless of family. Multi-AF devices fail spuriously. Fix: filter by `peerInfo["addressFamilies"]` or query `show bgp <afi> <safi> summary` directly.
- **`VerifyBGPRouteECMP` hardcodes threshold `>=2`** (`bgp.go:3567-3577`). The YAML doc shows `expected_paths: 4` per route but `Routes []string` discards everything except the prefix. Make `Routes` a struct list `{prefix, expected_paths, vrf}` and compare `len(NextHops)` against the per-route expected value. Same issue in `VerifyBGPRoutePaths` (`bgp.go:3394-3476`).
- **`VerifyBGPRedistribution` inverted logic** (`bgp.go:3679-3690`). Queries `show ip route bgp` — every route returned has routeType "BGP", so the check "Route X appears to be learned via BGP, not redistributed" fires for every row. Use `show ip bgp` and check the `origin`/`type` per path, or query the source-protocol table.
- **`VerifyBGPPeerTtlMultiHops` conflates `ttl` and `max_hops`** (`bgp.go:3823-3828`). YAML docs say `expected_ttl: 255` and `max_hops: 5`. EOS returns `ebgpMultihop` as a hop count (~5). Currently compares against `expected_ttl` so always fails. Fix: read both fields; compare `EbgpMultihop` to `max_hops`.
- **`VerifyBGPPeersHealthRibd` wrong AF key format** (`bgp.go:3201`). Builds `fmt.Sprintf("%s-%s", AFI, SAFI)` → `"ipv4-unicast"`. EOS `show bgp summary ribd` returns camelCase keys (`ipv4Unicast`, `evpn`). Always reports "Address family not found". Fix: canonicalize before lookup.

#### R2. `VerifyEVPNType5Routes` doc/code schema mismatch
`tests/evpn/type5_routes.go`. Docs (line 48) tell users to write `prefixes: [- address: ..., vni:, routes: [...]]`. Code reads `prefixMap["prefix"]` (not `address`) at line 118, and reads `inputs["routes"]`/`inputs["paths"]` at top level (line 134, 160). Every documented YAML config produces empty parses and the test errors. Either change docs to match code or restructure the parser. Pick whichever matches the intended schema.

### HIGH — broad correctness gaps

#### R3. Silent-PASS sweep — remaining packages
The `test.AsMap` helper exists; the systemic fix is the same: prepend a guard before each unguarded top-level cast. Files with known sites (per the reviewer reports):

- `tests/routing/static.go:128-180` (deeply nested 6-level traversal)
- `tests/routing/path_selection.go` — most sites correctly error; spot-check
- `tests/vxlan/conn_settings.go:84-128` — if `interfaces` key absent → silent pass
- `tests/vxlan/interface.go`, `vxlan/config_sanity.go`, `vxlan/vni_binding.go`, `vxlan/vtep.go` — same pattern
- `tests/stp/stp_tests.go:80-103` (and ~5 other `VerifyStp*` tests)
- `tests/evpn/type5_routes.go` (after R2 is decided)
- `tests/vlan/internal_policy.go`, `vlan/dynamic_source.go` — same pattern (vlan/status.go was OK)
- `tests/system/configuration.go` — check `VerifyRunningConfigDiffs` and `VerifyZeroTouch` if present
- `tests/system/system_tests.go:967, 667` — VerifyCPUUtilization and VerifyReload already have fallthroughs but verify
- `tests/logging/logging_tests.go` (many tests; reviewer flagged them as misclassifying parse-fail as TestFailure rather than TestError)
- `tests/services/services_tests.go` — VerifyHostname is correct; others should be checked

Recipe: for each `if XxxData, ok := cmdResult.Output.(map[string]any); ok {` with no else, prepend
```go
if _, err := test.AsMap(cmdResult.Output); err != nil {
    result.Status = test.TestError
    result.Message = fmt.Sprintf("Unexpected output: %v", err)
    return result, nil
}
```

#### R4. `ValidateInput` is mostly dead code
Across all `tests/`, `ValidateInput` either returns nil (stub) or validates the post-constructor struct rather than the raw `input` map. A user typo like `peer_addres:` produces an empty parsed value and either a vacuous PASS or "no peers found" — the real cause (key typo) is never surfaced.

Fix is best done as a convention change: make `NewVerify*` constructors return an error when an `inputs[key]` is present but the type assertion fails, e.g.

```go
if raw, ok := inputs["max_drops"]; ok {
    switch v := raw.(type) {
    case float64: t.MaxDrops = int64(v)
    case int:     t.MaxDrops = int64(v)
    default:      return nil, fmt.Errorf("max_drops: expected number, got %T", raw)
    }
}
```

Then `ValidateInput` can be deprecated or reduced to range-checks only.

This is dozens of edits — consider tackling per package.

#### R5. Three commands copy-paste Netbox query parsing; `check.go` is silently missing keys
`internal/cli/commands/nrfu.go:246-374`, `check.go:129-221`, `inventory.go:241-350` have the same ~110-line block. `check.go` is silently missing `site_id`, `role_id`, `device_type_id`, etc. that nrfu/inventory support, so `--netbox-query site_id=14` works for nrfu/inventory but drops silently in `check`.

Extract one shared `loadNetboxInventoryShared(ctx, opts NetboxOpts) (*inventory.Inventory, error)` (probably in a new `internal/cli/commands/netboxutil.go`) and have all three commands call it.

#### R6. `LoadFromNetbox` ignores caller's context
`pkg/inventory/netbox.go:296-298` uses `context.Background()` and discards the passed-in ctx. After fix #17 (signal handling), this is the last layer that doesn't honor cancellation. Change the signature to accept ctx and thread it through. Breaking API change — coordinate with the v1 surface.

#### R7. Catalog validation doesn't check registry membership
`pkg/test/catalog.go`. A typo like `VerifyBGPpeers` (lowercase 'p') is not caught at parse time; instead every device gets a `TestError "Test not found"`, producing 50× duplicate errors instead of one upfront failure. Add `Catalog.ValidateAgainst(registry *Registry) error` and call it from `runNrfu` before `runner.Run`.

#### R8. Filter functions silently return empty on no match
`pkg/inventory/inventory.go` and `pkg/test/catalog.go`. `--limit nosuchhost`, `--test nosuchname`, `FilterByTags(["typo"])` all return an empty result with no error. User sees "no tests matched, all good." Make them return `(result, error)` or at least log a warning.

#### R9. `VerifyBGPExchangedRoutes` is N+1 over the device
`tests/routing/bgp.go:1156-1219`. Two device calls per peer × routes. With 50 peers that's 100 RTTs. Same issue in `VerifyBGPPeerMPCaps`, `VerifyBGPAdvCommunities`, `VerifyBGPPeerDropStats`, `VerifyBGPPeerUpdateErrors`. The `device.Device` interface has `ExecuteBatch` (`pkg/device/device.go:15`) — use it. Test framework should expose a batched-Execute hook so all tests benefit.

### MEDIUM — design / correctness, less urgent

- **R10. `tests/system/system_tests.go:152` `parseVersion`** uses `fmt.Sscanf` with discarded error; `4.28.1F-rc.2` and `4.28.1F-rc.1` parse identically. Use a real version parser.
- **R11. NTP/LLDP bidirectional `strings.Contains`** (`tests/system/system_tests.go:382-421`, `tests/connectivity/lldp.go:156`). `10.1.1.1` matches `10.1.1.10`; `spine1` matches `spine100`. Switch to exact match or anchored regex.
- **R12. STP mode validation lists `rapidPvst`** (`tests/stp/stp_tests.go:117`). EOS returns `rapid-pvst`. Anyone passing the validator's accepted value always fails the device comparison. Normalize like `normalizeOSPFState`.
- **R13. `hardware/inventory.go:113,122` likely off by 1024×** in memory/flash MB calc. EOS reports `memTotal` in KB, code divides by 1024 twice. Verify against actual EOS output and adjust.
- **R14. `bgp.go` is 3843 lines / 24 tests** — split per concern (session, capabilities, health, routes, config). Extract the repeated VRF→peer lookup helper.
- **R15. Two near-duplicate runners** (`pkg/test/runner.go` + `progress_runner.go`). Merge so concurrency lives in one place with an injected progress hook. After this you can also implement R9's batching once instead of twice.
- **R16. UDP port range typo** in `tests/vxlan/conn_settings.go:143` — `65335` should be `65535`.
- **R17. `VerifyBGPPeerDropStats` / `VerifyBGPPeerUpdateErrors` accept input as `map`** but the YAML docs show a list. List-shaped inputs are silently ignored. Either accept both or update docs.
- **R18. `VerifyStpTopologyChanges` double-counts** (`tests/stp/stp_tests.go:551-565`). Per-interface check appends + total sum compared, semantics ambiguous.
- **R19. Test-result model unclear contract.** `TestResult.Message` is a `%v` blob; `Details` is declared but never written; `CustomField` is dead. Define a structured `Details` shape and remove `CustomField`.
- **R20. Cobra flag conflict detection.** Use `MarkFlagsMutuallyExclusive("inventory", "netbox-url")` and `MarkFlagsOneRequired(...)`. Currently `-i foo.yaml --netbox-url ...` silently picks Netbox.
- **R21. Logger writes log file with mode `0666`** (`internal/logger/logger.go:69`). Should be `0600` — debug logs may contain payloads.
- **R22. TLS posture** (`pkg/device/eos.go:35-50`). `MinVersion: TLS 1.0` + explicit ciphers that exclude modern AEAD on TLS 1.2. Default to TLS 1.2 minimum, drop the explicit list, make legacy opt-in.
- **R23. Catalog field name mismatches.** Various tests' YAML docs claim `domain_names:` vs code reads `fqdn:` etc. Cross-walk docs vs code (the reviewer found several).
- **R24. Netbox `device_role` vs `role` schema drift** (`pkg/inventory/netbox.go:93`). Netbox 4.x renamed the field. Add both names or version-detect.
- **R25. `cmd/test-catalog/` is an empty directory** — delete it.
- **R26. `cmd/debug` and `cmd/debug-json` are separate binaries** that hand-roll eAPI payloads — drift risk. Port to subcommands or delete.

### LOW

A long tail of style and ergonomics from the reviewer notes. Lower priority but documented:
- `repeat()` instead of `strings.Repeat` (progress runner)
- `time.Sleep` as synchronization in progress UI
- Map-iteration nondeterminism in `inventory` summary output
- `nrfu`/`quiet`/`silent`/`progress` flag overlap UX
- `tabwriter.Flush()` error ignored
- Stale TODOs and dead code in `tests/hardware/transceivers.go`
- Hardcoded `maxRoutes := 1000000` in `tests/hardware/chassis.go:417`

## Verifying the work so far

```bash
go build ./...
go vet ./...
# Once a regression test exists, add:
# go test -race ./...
```

No automated tests exist for the engine yet — would be worth adding race-detector tests for the runner and device packages before further concurrency changes.

## Suggested order to resume

1. **R1 BGP protocol bugs** — small files, high false-positive/negative risk.
2. **R3 silent-PASS sweep** in `tests/routing/static.go`, `tests/vxlan/`, `tests/stp/`, `tests/vlan/`, `tests/logging/` — mechanical, follows the pattern in commits 14/15/18/19.
3. **R5 Netbox parsing dedup** — catches `check.go`'s missing-keys bug.
4. **R6 LoadFromNetbox ctx** — finishes the cancellation plumbing from #17.
5. **R2 EVPN Type-5 schema** — decide doc vs code, then implement.
6. **R4 ValidateInput** — biggest behavioral improvement but most invasive.

If picking this up cold, read the device-layer (`pkg/device/eos.go`) and the test runner (`pkg/test/runner.go`) first to understand the framework's contract before touching test impls.
