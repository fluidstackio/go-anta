package inventory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fluidstackio/go-anta/pkg/device"
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

func TestNetboxSource_LegacyFormat(t *testing.T) {
	tmp := writeYAML(t, `
netbox:
  url: https://netbox.example.com
  token: secret
query:
  site: wdl1
`)
	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource (legacy): %v", err)
	}
	nb, ok := src.(*NetboxSource)
	if !ok {
		t.Fatalf("expected *NetboxSource, got %T", src)
	}
	if nb.config.URL != "https://netbox.example.com" {
		t.Errorf("legacy URL: got %q", nb.config.URL)
	}
	if nb.query.Site != "wdl1" {
		t.Errorf("legacy query: got %+v", nb.query)
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

// TestLoadFromNetbox_HonorsContextCancel asserts that the back-compat
// wrapper threads ctx all the way through to the HTTP layer (R6).
// Before the fix, LoadFromNetbox built its own context.Background()
// and a cancelled caller ctx had no effect.
func TestLoadFromNetbox_HonorsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { cancel() }()
	_, err := LoadFromNetbox(ctx, NetboxConfig{URL: srv.URL, Token: "x"}, NetboxQuery{}, nil)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got %v", err)
	}
}

func TestLoadNetboxInventory_HonorsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	tmp := writeYAML(t, `
netbox:
  url: `+srv.URL+`
  token: x
credentials:
  username: admin
  password: pw
`)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { cancel() }()
	_, err := LoadNetboxInventory(ctx, tmp)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Errorf("expected context error, got %v", err)
	}
}

func TestApplyDefaults_EnablePasswordAndTimeout(t *testing.T) {
	inv := &Inventory{
		Devices: []device.DeviceConfig{{Name: "r1", Host: "10.0.0.1"}},
	}
	out := inv.ApplyDefaults(DeviceDefaults{
		EnablePassword: "secret-enable",
		Timeout:        30 * time.Second,
	})
	if out.Devices[0].EnablePassword != "secret-enable" {
		t.Errorf("EnablePassword: got %q", out.Devices[0].EnablePassword)
	}
	if out.Devices[0].Timeout != 30*time.Second {
		t.Errorf("Timeout: got %s", out.Devices[0].Timeout)
	}
}

func TestLoadFromNetbox_PassesEnablePasswordAndTimeout(t *testing.T) {
	// Build a mock Netbox server that returns one device with a primary IP.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"name":"r1","primary_ip":{"address":"10.0.0.1/24"}}]}`))
	}))
	defer srv.Close()

	inv, err := LoadFromNetbox(
		context.Background(),
		NetboxConfig{URL: srv.URL, Token: "x"},
		NetboxQuery{},
		map[string]interface{}{
			"username":        "admin",
			"password":        "pw",
			"enable_password": "ena",
			"timeout":         "15s",
		},
	)
	if err != nil {
		t.Fatalf("LoadFromNetbox: %v", err)
	}
	if len(inv.Devices) != 1 {
		t.Fatalf("device count: got %d want 1", len(inv.Devices))
	}
	if inv.Devices[0].EnablePassword != "ena" {
		t.Errorf("EnablePassword: got %q want ena", inv.Devices[0].EnablePassword)
	}
	if inv.Devices[0].Timeout != 15*time.Second {
		t.Errorf("Timeout: got %s want 15s", inv.Devices[0].Timeout)
	}
}

func TestLoadFromNetbox_ErrorsOnEmptyInventory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	_, err := LoadFromNetbox(context.Background(), NetboxConfig{URL: srv.URL, Token: "x"}, NetboxQuery{}, nil)
	if err == nil || !strings.Contains(err.Error(), "no devices") {
		t.Errorf("expected no-devices error, got %v", err)
	}
}

func TestLoadNetboxInventory_HonorsEnvVars(t *testing.T) {
	// Build a YAML with a placeholder URL; env vars should override it.
	// The mock server returns a device; credentials in YAML satisfy Validate.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":[{"name":"r1","primary_ip":{"address":"10.0.0.1/24"}}]}`))
	}))
	defer srv.Close()

	tmp := writeYAML(t, `
netbox:
  url: https://wrong.example.com
  token: wrong-token
credentials:
  username: admin
  password: pw
`)

	t.Setenv("NETBOX_URL", srv.URL)
	t.Setenv("NETBOX_TOKEN", "right-token")

	inv, err := LoadNetboxInventory(context.Background(), tmp)
	if err != nil {
		t.Fatalf("LoadNetboxInventory: %v", err)
	}
	if len(inv.Devices) != 1 {
		t.Errorf("device count: got %d want 1", len(inv.Devices))
	}
}

func TestLoadNetboxInventory_RequiresURL(t *testing.T) {
	tmp := writeYAML(t, `
netbox:
  token: just-a-token
`)
	// No env vars set — URL is required and missing.
	t.Setenv("NETBOX_URL", "")
	t.Setenv("NETBOX_TOKEN", "")
	_, err := LoadNetboxInventory(context.Background(), tmp)
	if err == nil || !strings.Contains(err.Error(), "URL") {
		t.Errorf("expected URL-required error, got %v", err)
	}
}
