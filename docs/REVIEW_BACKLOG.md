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

### Second-pass PRs (post-merge)

| PR | What it closes / adds |
|---|---|
| #16 `feat/gnoi-ping-traceroute` | Replaces CLI-parsed `ping` with gNOI System.Ping; adds `VerifyTraceroute`. `Device.Ping`/`Device.Traceroute` interface methods; `GNMIDevice` implementation; `EOSDevice` returns `ErrDiagUnsupported`. **Supersedes the LLDP/NTP half of R11 for ping specifically** (the regex parser is gone) and supplies the missing traceroute capability the original review did not call out. |
| #17 `fix/backlog-cleanup` | Ships R3 (silent-PASS sweep across routing/static, vxlan/*, stp/*, vlan/*, system/configuration, services, logging), R11 (NTP/LLDP substring match → exact match w/ FQDN tolerance), R12 (`VerifySTPMode` rewrite against real EOS shape — broader than the original "normalize rapidPvst" framing), R13 (memory units; flash via `show file systems`), R16 (UDP port typo). |
| #18 `fix/bgp-r1-afi-safi` | Ships R1 (the 5 BGP sub-items above). |

## Remaining work — prioritized

Each item below has a concrete file:line anchor and the rough shape of the fix. Severity reflects production impact, not implementation difficulty.

### CRITICAL — production correctness gaps

#### ~~R1. BGP protocol-correctness bugs~~ — shipped on `fix/bgp-r1-afi-safi`

All five sub-items addressed. Each is an independent commit; lab-verified
against EOS 4.34.4M (wdl101) except where noted. Real-data inspection
revealed that several of the original diagnoses were *less severe* than
what was actually broken — the file had wrong response decoders, wrong
field names, and YAML inputs that were never parsed.

| Sub-item | Commit | What landed |
|---|---|---|
| R1.1 AFI/SAFI in `VerifyBGPPeerCount` / `VerifyBGPPeersHealth` | `bb8e198` | Per-AF query via new `bgpSummaryCommand` helper (`show bgp ipv4 unicast summary` / `show bgp evpn summary` / …). Old code read peer-level `peerState` regardless of AF. |
| R1.5 `VerifyBGPPeersHealthRibd` AF key | `b59d466` | New `bgpRibdAFKey` helper produces camelCase keys (`ipv4Unicast`, `vpnIpv4`, `ipv4MplsLabels`, `evpn`). Note: `show bgp summary ribd` is invalid on multi-agent EOS; fix is for ribd-mode devices. |
| R1.4 `VerifyBGPPeerTtlMultiHops` | `5f445c1` | Found three layered bugs: YAML `expected_ttl`/`max_hops` were never parsed; response decoded as `vrfs.<vrf>.neighbors` (map) but EOS returns `vrfs.<vrf>.peerList` (array); field was `ebgpMultihop` (nonexistent) — actual EOS fields are `ttl` and `maxTtlHops`. Now queries `show bgp neighbors vrf all` and compares both fields. Real `ValidateInput`. |
| R1.2 `VerifyBGPRoutePaths` / `VerifyBGPRouteECMP` | `d67685c` | `Routes []string` replaced by `BgpRoute{Prefix, ExpectedPaths, VRF}` plus shared `parseBgpRoutes`. RoutePaths now uses `show ip bgp vrf all` counting `bgpRoutePaths`; ECMP groups by VRF and issues `show ip route vrf <vrf> bgp detail` counting `vias` (the old `nextHops` field never existed — EOS uses `vias`). Fallback: ECMP requires ≥2 next-hops when `expected_paths` is unset; RoutePaths requires ≥1 path. |
| R1.3 `VerifyBGPRedistribution` | `98203bf` | Was silently passing: it queried `show ip route bgp` and compared `routeType == "BGP"`, but EOS returns "eBGP"/"iBGP" so the failure branch was vacuously unreachable. Rewritten around `show bgp instance.vrfs[vrf].afiSafiConfig[af].redistributedRoutes[].proto` for the config check; optional `expected_count` enforced as an upper-bound sanity check via `show ip route vrf <vrf> <proto>`. New `BgpRedistribution` type; real `ValidateInput`. |

Real-device outputs used for verification are in `/tmp/bgp-out/` (regenerable
with `cmd/debug` against `examples/wdl101-inventory.yaml`).

#### R2. `VerifyEVPNType5Routes` doc/code schema mismatch
`tests/evpn/type5_routes.go`. Docs (line 48) tell users to write `prefixes: [- address: ..., vni:, routes: [...]]`. Code reads `prefixMap["prefix"]` (not `address`) at line 118, and reads `inputs["routes"]`/`inputs["paths"]` at top level (line 134, 160). Every documented YAML config produces empty parses and the test errors. Either change docs to match code or restructure the parser. Pick whichever matches the intended schema.

### HIGH — broad correctness gaps

#### ~~R3. Silent-PASS sweep — remaining packages~~ — shipped in PR #17

Guards landed across `tests/routing/static.go`, `tests/vxlan/*`, `tests/stp/*`,
`tests/vlan/{internal_policy,dynamic_source,status}.go`,
`tests/system/configuration.go`, `tests/services/services_tests.go`, and
`tests/logging/logging_tests.go`. STP work surfaced *additional* schema
bugs beyond R3's recipe — six STP tests were reading non-existent fields
(`instances` / `protocolMode` / `spanningTreeInstances` as slice) so they
got real rewrites against EOS's actual `spanningTreeInstances` map. EVPN
Type-5 deliberately deferred until R2 is decided.

#### ~~R4. `ValidateInput` is mostly dead code~~ — partly shipped in `fix/r4-input-validation`

Two parts:

1. **Typo detection (shipped, all tests covered)** — `pkg/test.ValidateInputKeys`
   now runs inside `Registry.GetTestWithInputs` after the factory
   returns. It reflects on the test struct's `yaml:""` tags (skipping
   embedded `BaseTest`'s metadata fields) and rejects any top-level
   input key not in that set. Tests with non-yaml-tagged input fields
   can implement `CustomInputKeys` to opt-in extra keys. Tests that
   declare no input tags at all are skipped — the framework has
   nothing to compare against and they're read ad-hoc. Result: typos
   like `peer_addres:` now fail at construction time with
   `connectivity/VerifyReachability: unknown input key(s) [peer_addres]; valid keys are: [hosts repeat size]`.

2. **Type-mismatch detection at the field level (helpers shipped, retrofit pending)** —
   `pkg/test.GetInt/GetString/GetBool/GetStringSlice` are available
   for use in new and edited constructors. They return typed errors
   for present-but-wrong-type values (`max_drops: "five"` →
   `max_drops: expected number, got string`). Existing 125
   constructors still read `inputs[key]` ad-hoc — they continue to
   work, but they silently coerce-to-default on type mismatch. Whoever
   edits a NewVerify\* next should migrate it to the helpers; new
   tests should use them from the start.

#### R5. Three commands copy-paste Netbox query parsing; `check.go` is silently missing keys
`internal/cli/commands/nrfu.go:246-374`, `check.go:129-221`, `inventory.go:241-350` have the same ~110-line block. `check.go` is silently missing `site_id`, `role_id`, `device_type_id`, etc. that nrfu/inventory support, so `--netbox-query site_id=14` works for nrfu/inventory but drops silently in `check`.

Extract one shared `loadNetboxInventoryShared(ctx, opts NetboxOpts) (*inventory.Inventory, error)` (probably in a new `internal/cli/commands/netboxutil.go`) and have all three commands call it.

#### R6. `LoadFromNetbox` ignores caller's context
`pkg/inventory/netbox.go:296-298` uses `context.Background()` and discards the passed-in ctx. After fix #17 (signal handling), this is the last layer that doesn't honor cancellation. Change the signature to accept ctx and thread it through. Breaking API change — coordinate with the v1 surface.

#### ~~R7. Catalog validation doesn't check registry membership~~ — shipped in `fix/r7-r8-typo-detection`

`Catalog.ValidateAgainst(*Registry)` walks every catalog entry and
sorts unknown `(Module, Name)` tuples into a single error. Called from
`runNrfu` after filtering. A typo like `VerifyBGPpeers` now surfaces
once at start with `catalog references unknown test(s): [routing/VerifyBGPpeers]`
instead of N copies of "Test not found".

#### ~~R8. Filter functions silently return empty on no match~~ — shipped in `fix/r7-r8-typo-detection`

All six filter functions (`Catalog.FilterByName/Module/Tags` and
`Inventory.FilterByNames/Tags/Limit`) now return `(*Filtered, error)`.
Error names the specific unmatched values (`no matches for device name(s): [leaf99]`
or `--limit "nosuchhost" matched no devices`). `nrfu` and `check`
surface filter errors as hard failures; `inventory` (an exploratory
command) prints them as `warning:` to stderr instead. `nrfu` also now
fails fast if devices or tests reach zero after filtering — the old
"no tests matched, all good" silent path is gone.

#### R9. `VerifyBGPExchangedRoutes` is N+1 over the device
`tests/routing/bgp.go:1156-1219`. Two device calls per peer × routes. With 50 peers that's 100 RTTs. Same issue in `VerifyBGPPeerMPCaps`, `VerifyBGPAdvCommunities`, `VerifyBGPPeerDropStats`, `VerifyBGPPeerUpdateErrors`. The `device.Device` interface has `ExecuteBatch` (`pkg/device/device.go:15`) — use it. Test framework should expose a batched-Execute hook so all tests benefit.

### MEDIUM — design / correctness, less urgent

- **R10. `tests/system/system_tests.go:152` `parseVersion`** uses `fmt.Sscanf` with discarded error; `4.28.1F-rc.2` and `4.28.1F-rc.1` parse identically. Use a real version parser.
- ~~**R11. NTP/LLDP bidirectional `strings.Contains`**~~ — shipped in PR #17. Exact match (case-insensitive, with FQDN tolerance for LLDP `systemName`).
- ~~**R12. STP mode validation lists `rapidPvst`**~~ — shipped in PR #17. The real fix was bigger: the test was reading non-existent fields entirely; rewritten against `spanningTreeInstances.<key>.protocol` with `normalizeStpMode` lowercase/hyphen-strip.
- ~~**R13. `hardware/inventory.go` memory off by 1024×**~~ — shipped in PR #17. `memTotal` treated as KB (one divide); flash size source switched from non-existent `flashSize` in `show version` to `show file systems` lookup of the `flash:` filesystem.
- **R14. `bgp.go` is 3843 lines / 24 tests** — split per concern (session, capabilities, health, routes, config). Extract the repeated VRF→peer lookup helper.
- **R15. Two near-duplicate runners** (`pkg/test/runner.go` + `progress_runner.go`). Merge so concurrency lives in one place with an injected progress hook. After this you can also implement R9's batching once instead of twice.
- ~~**R16. UDP port range typo**~~ — shipped in PR #17.
- **R17. `VerifyBGPPeerDropStats` / `VerifyBGPPeerUpdateErrors` accept input as `map`** but the YAML docs show a list. List-shaped inputs are silently ignored. Either accept both or update docs.
- **R18. `VerifyStpTopologyChanges` double-counts** (`tests/stp/stp_tests.go:551-565`). Per-interface check appends + total sum compared, semantics ambiguous.
- **R19. Test-result model unclear contract.** `TestResult.Message` is a `%v` blob; `Details` is declared but never written; `CustomField` is dead. Define a structured `Details` shape and remove `CustomField`.
- **R20. Cobra flag conflict detection.** Use `MarkFlagsMutuallyExclusive("inventory", "netbox-url")` and `MarkFlagsOneRequired(...)`. Currently `-i foo.yaml --netbox-url ...` silently picks Netbox.
- **R21. Logger writes log file with mode `0666`** (`internal/logger/logger.go:69`). Should be `0600` — debug logs may contain payloads.
- **R22. TLS posture** (`pkg/device/eos.go:35-50`). `MinVersion: TLS 1.0` + explicit ciphers that exclude modern AEAD on TLS 1.2. Default to TLS 1.2 minimum, drop the explicit list, make legacy opt-in.
- **R23. Catalog field name mismatches.** Various tests' YAML docs claim `domain_names:` vs code reads `fqdn:` etc. Cross-walk docs vs code (the reviewer found several).
- **R24. Netbox `device_role` vs `role` schema drift** (`pkg/inventory/netbox.go:93`). Netbox 4.x renamed the field. Add both names or version-detect.
- ~~**R25. `cmd/test-catalog/` is an empty directory**~~ — deleted in the cleanup pass after PR #18, along with `web/`, `internal/web/`, `internal/catalog/`, `internal/api/` (all empty).
- ~~**R26. `cmd/debug-json`** — deleted~~. It only printed a hardcoded `show version` JSON payload plus a curl example with placeholder credentials; no device interaction. `cmd/debug` stays — it's used by `examples/wdl101-inventory.yaml` smoke-testing and was the workhorse for the gNOI verification in PR #16.

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
