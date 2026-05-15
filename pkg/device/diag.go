package device

import (
	"errors"
	"time"
)

// ErrDiagUnsupported is returned by Ping and Traceroute on transports
// that don't support the operation. eAPI returns it because the gNMI
// gNOI System service is the only reliable, structured ping/traceroute
// path on Arista — `runCmds ping ...` over eAPI works but is brittle
// (free-form output text), and `cli` over gNMI Get is restricted to
// `show` commands by the server.
var ErrDiagUnsupported = errors.New("ping/traceroute not supported on this transport")

// L3Protocol selects the address family for Ping/Traceroute. Auto lets
// the device decide based on the destination's literal form.
type L3Protocol int

const (
	L3ProtocolAuto L3Protocol = iota
	L3ProtocolIPv4
	L3ProtocolIPv6
)

// PingOpts configures a Ping. Source must be a literal IP address (not
// an interface name) — Arista's gNOI implementation rejects interface
// names with "ping: unknown iface: <name>". Operators who want to ping
// out of a specific interface should pass that interface's IP.
type PingOpts struct {
	Destination   string        // required
	Source        string        // literal IP, optional
	VRF           string        // network instance, "" → default
	Count         int           // 0 → device default
	Size          int           // bytes, 0 → device default
	Interval      time.Duration // between echoes
	Wait          time.Duration // per-echo timeout
	DoNotFragment bool
	DoNotResolve  bool
	L3Protocol    L3Protocol
}

// PingResult aggregates a Ping streaming response.
type PingResult struct {
	Destination string
	Source      string       // source IP as reported by the device
	Echoes      []PingEcho   // one per received reply
	Stats       PingStats    // summary statistics
}

type PingEcho struct {
	Sequence int
	Bytes    int
	TTL      int
	RTT      time.Duration
}

type PingStats struct {
	Sent     int
	Received int
	Loss     float64       // 0.0 - 1.0
	MinRTT   time.Duration
	AvgRTT   time.Duration
	MaxRTT   time.Duration
	StdDev   time.Duration
	Duration time.Duration // total elapsed time of the ping run
}

// TracerouteOpts configures a Traceroute. Same Source caveat as
// PingOpts — must be an IP, not an interface name.
type TracerouteOpts struct {
	Destination   string
	Source        string
	VRF           string
	InitialTTL    int           // 0 → device default
	MaxTTL        int           // 0 → device default
	Wait          time.Duration // per-probe timeout
	Probes        int           // probes per hop, 0 → device default (3)
	DoNotFragment bool
	DoNotResolve  bool
	L3Protocol    L3Protocol
}

// TracerouteResult is the aggregated structured response. Hops is
// ordered by hop number; each hop has 1..Probes entries (the gNOI
// stream emits one message per probe, which we coalesce here).
type TracerouteResult struct {
	Destination     string
	DestinationName string
	PacketSize      int
	Hops            []TracerouteHop
}

type TracerouteHop struct {
	Number int
	Probes []TracerouteProbe
}

type TracerouteProbe struct {
	Address string
	Name    string
	RTT     time.Duration
	State   TracerouteState
}

// TracerouteState mirrors the gNOI TracerouteResponse.State enum.
type TracerouteState int

const (
	TracerouteStateUnknown        TracerouteState = iota
	TracerouteStateNone                            // default — ICMP echo-reply, no special state
	TracerouteStateICMP                            // unreachable ICMP returned
	TracerouteStateHostUnreachable
	TracerouteStateNetworkUnreachable
	TracerouteStateProtocolUnreachable
	TracerouteStateSourceRouteFailed
	TracerouteStateFragmentationNeeded
	TracerouteStateProhibited
	TracerouteStatePrecedenceViolation
	TracerouteStatePrecedenceCutoff
)
