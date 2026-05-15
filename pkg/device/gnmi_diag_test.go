package device

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestGNMIDevice_Ping_Integration exercises gNOI System.Ping against a
// real gNMI-enabled device. Env vars:
//
//	GO_ANTA_GNMI_HOST       — device host (IPv4/IPv6)
//	GO_ANTA_GNMI_USER       — username
//	GO_ANTA_GNMI_PASS       — password
//	GO_ANTA_GNMI_PORT       — port (default 6030)
//	GO_ANTA_GNMI_PLAINTEXT  — "1" to use plaintext gRPC
//	GO_ANTA_PING_DEST       — ping destination (default 127.0.0.1)
//	GO_ANTA_PING_VRF        — optional VRF
func TestGNMIDevice_Ping_Integration(t *testing.T) {
	dev := setupDiagDevice(t)
	defer dev.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dest := os.Getenv("GO_ANTA_PING_DEST")
	if dest == "" {
		dest = "127.0.0.1"
	}

	res, err := dev.Ping(ctx, PingOpts{
		Destination: dest,
		Count:       3,
		VRF:         os.Getenv("GO_ANTA_PING_VRF"),
	})
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if res.Stats.Sent != 3 {
		t.Errorf("expected Stats.Sent=3, got %d", res.Stats.Sent)
	}
	if res.Stats.Received == 0 {
		t.Errorf("expected at least one echo, got Received=0; echoes=%d", len(res.Echoes))
	}
	if len(res.Echoes) != res.Stats.Received {
		t.Errorf("expected len(Echoes)==Stats.Received, got %d vs %d", len(res.Echoes), res.Stats.Received)
	}
	for i, e := range res.Echoes {
		if e.Sequence == 0 {
			t.Errorf("echo[%d] missing Sequence", i)
		}
		if e.RTT == 0 {
			t.Errorf("echo[%d] missing RTT", i)
		}
	}
	t.Logf("Ping %s: sent=%d received=%d avg=%v loss=%.2f", dest, res.Stats.Sent, res.Stats.Received, res.Stats.AvgRTT, res.Stats.Loss)
}

func TestGNMIDevice_Traceroute_Integration(t *testing.T) {
	dev := setupDiagDevice(t)
	defer dev.Disconnect()

	// Traceroute can be slow on the device side; gNOI streams may take
	// well over a minute before the first message even on a one-hop
	// path. Generous timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dest := os.Getenv("GO_ANTA_TRACEROUTE_DEST")
	if dest == "" {
		// 127.0.0.1 is guaranteed reachable from any VRF in one hop on
		// every Linux-based NOS, including EOS. Override with
		// GO_ANTA_TRACEROUTE_DEST for richer paths.
		dest = "127.0.0.1"
	}

	res, err := dev.Traceroute(ctx, TracerouteOpts{
		Destination: dest,
		MaxTTL:      5,
		Wait:        2 * time.Second,
		VRF:         os.Getenv("GO_ANTA_TRACEROUTE_VRF"),
	})
	if err != nil {
		t.Fatalf("Traceroute: %v", err)
	}
	if len(res.Hops) == 0 {
		t.Errorf("expected at least one hop, got none")
	}
	for i, h := range res.Hops {
		if h.Number != i+1 {
			t.Errorf("expected hop %d, got %d", i+1, h.Number)
		}
		if len(h.Probes) == 0 {
			t.Errorf("hop %d has no probes", h.Number)
		}
	}
	t.Logf("Traceroute %s: %d hops", dest, len(res.Hops))
}

func TestEOSDevice_Ping_Unsupported(t *testing.T) {
	dev := &EOSDevice{}
	_, err := dev.Ping(context.Background(), PingOpts{Destination: "1.1.1.1"})
	if !errors.Is(err, ErrDiagUnsupported) {
		t.Errorf("expected ErrDiagUnsupported, got %v", err)
	}
	_, err = dev.Traceroute(context.Background(), TracerouteOpts{Destination: "1.1.1.1"})
	if !errors.Is(err, ErrDiagUnsupported) {
		t.Errorf("expected ErrDiagUnsupported from Traceroute, got %v", err)
	}
}

func setupDiagDevice(t *testing.T) Device {
	t.Helper()
	host := os.Getenv("GO_ANTA_GNMI_HOST")
	user := os.Getenv("GO_ANTA_GNMI_USER")
	pass := os.Getenv("GO_ANTA_GNMI_PASS")
	if host == "" || user == "" || pass == "" {
		t.Skip("set GO_ANTA_GNMI_HOST/USER/PASS to run diag integration test")
	}
	cfg := DeviceConfig{
		Name:      "diag-smoke",
		Host:      host,
		Username:  user,
		Password:  pass,
		Transport: "gnmi",
		Insecure:  true,
		Plaintext: os.Getenv("GO_ANTA_GNMI_PLAINTEXT") == "1",
		Timeout:   15 * time.Second,
	}
	if p := os.Getenv("GO_ANTA_GNMI_PORT"); p != "" {
		var port int
		if _, err := timeFmtAtoi(p, &port); err != nil {
			t.Fatalf("GO_ANTA_GNMI_PORT not an integer: %v", err)
		}
		cfg.Port = port
	}
	dev, err := New(cfg)
	if err != nil {
		t.Fatalf("device.New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := dev.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return dev
}

// timeFmtAtoi is a tiny shim so the test file doesn't need strconv just
// for one parse; we already import time everywhere.
func timeFmtAtoi(s string, out *int) (int, error) {
	var n int
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errors.New("not a digit: " + string(c))
		}
		n = n*10 + int(c-'0')
	}
	*out = n
	return n, nil
}
