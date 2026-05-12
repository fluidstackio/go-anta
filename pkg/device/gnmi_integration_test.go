package device

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestGNMIDevice_Integration exercises Connect + Execute against a real
// gNMI-enabled device. It is gated by env vars so CI skips it. To run
// locally:
//
//	GO_ANTA_GNMI_HOST=fc00:800f:f01::8 \
//	GO_ANTA_GNMI_USER=admin \
//	GO_ANTA_GNMI_PASS=admin \
//	go test ./pkg/device/ -run TestGNMIDevice_Integration -v
//
// The host may be an IPv4 or IPv6 address (bracket notation is not
// required — Go's net.Dial handles the format when passed as a literal
// without brackets, and the transport joins host:port internally).
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
