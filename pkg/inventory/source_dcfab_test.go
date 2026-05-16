package inventory

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestDcfabSource_FactoryParsesYAML(t *testing.T) {
	tmp := writeYAML(t, `
kind: dcfab
env: prod
region: wdl1
filter: |
  implementation: ACTIVE,
  roles: ["fm0","fm1"],
  platforms: ["eos"]
prefer_ip: ipv6
`)
	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	d, ok := src.(*DcfabSource)
	if !ok {
		t.Fatalf("expected *DcfabSource, got %T", src)
	}
	if d.cfg.Region != "wdl1" || d.cfg.Env != "prod" || d.cfg.PreferIP != "ipv6" {
		t.Errorf("config not parsed: %+v", d.cfg)
	}
	if !strings.Contains(d.cfg.Filter, `roles: ["fm0","fm1"]`) {
		t.Errorf("filter not parsed: %q", d.cfg.Filter)
	}
}

func TestDcfabSource_FactoryRejectsMissingRegion(t *testing.T) {
	tmp := writeYAML(t, `
kind: dcfab
env: prod
filter: |
  implementation: ACTIVE
`)
	_, err := LoadSource(tmp)
	if err == nil || !strings.Contains(err.Error(), "region") {
		t.Errorf("expected region-required error, got %v", err)
	}
}

func TestDcfabSource_FactoryRejectsMissingFilter(t *testing.T) {
	tmp := writeYAML(t, `
kind: dcfab
region: wdl1
`)
	_, err := LoadSource(tmp)
	if err == nil || !strings.Contains(err.Error(), "filter") {
		t.Errorf("expected filter-required error, got %v", err)
	}
}

func TestDcfabSource_QueryURLSplicesFilterVerbatim(t *testing.T) {
	src := &DcfabSource{
		cfg: DcfabConfig{
			Region: "wdl1",
			Filter: `implementation: ACTIVE, roles: ["fm0"], platforms: ["eos"]`,
		},
	}
	u := src.queryURL("https://dcfab.example.com")
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if !strings.HasPrefix(u, "https://dcfab.example.com/v1alpha1/query?query=") {
		t.Errorf("wrong endpoint prefix: %s", u)
	}
	q := parsed.Query().Get("query")
	for _, want := range []string{`region(name: "wdl1")`, `roles: ["fm0"]`, `platforms: ["eos"]`, `implementation: ACTIVE`, `limit: 5000`, `managementInterface`} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q\nfull query: %s", want, q)
		}
	}
}

func TestDcfabSource_QueryURLRespectsUserLimit(t *testing.T) {
	src := &DcfabSource{
		cfg: DcfabConfig{
			Region: "wdl1",
			Filter: `implementation: ACTIVE, limit: 100`,
		},
	}
	u := src.queryURL("https://dcfab.example.com")
	parsed, _ := url.Parse(u)
	q := parsed.Query().Get("query")
	if !strings.Contains(q, "limit: 100") {
		t.Errorf("user limit not preserved: %s", q)
	}
	if strings.Contains(q, "limit: 5000") {
		t.Errorf("auto-limit should not be appended when user set one: %s", q)
	}
}

func TestDcfabSource_EndpointDefaults(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", "https://dcfab.fluidstack.io"},
		{"prod", "https://dcfab.fluidstack.io"},
		{"dev", "https://dcfab.fluidstack.xyz"},
	}
	for _, tc := range cases {
		got := DcfabEndpoint(DcfabConfig{Env: tc.env})
		if got != tc.want {
			t.Errorf("env=%q: got %q want %q", tc.env, got, tc.want)
		}
	}
	// Explicit Endpoint wins.
	custom := DcfabEndpoint(DcfabConfig{Env: "prod", Endpoint: "https://custom.test"})
	if custom != "https://custom.test" {
		t.Errorf("explicit endpoint should win, got %q", custom)
	}
}

// mockDcfabServer is used in Task 8; declared here so the file compiles
// when we add HTTP tests next task.
func mockDcfabServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDcfabSource_LoadMapsDevicesToConfigs(t *testing.T) {
	body := `{
	  "data": {
	    "region": {
	      "devices": [
	        {
	          "name": "wdl101-fis-fm1-r1",
	          "role": "fm",
	          "platform": "eos",
	          "managementInterface": {
	            "addresses": [
	              {"address": "fc00:800f:f01::8/64", "version": 6},
	              {"address": "10.0.0.8/24", "version": 4}
	            ]
	          }
	        },
	        {
	          "name": "wdl101-fis-fm1-r2",
	          "role": "fm",
	          "platform": "eos",
	          "managementInterface": {
	            "addresses": [
	              {"address": "10.0.0.9/24", "version": 4}
	            ]
	          }
	        }
	      ]
	    }
	  }
	}`
	srv := mockDcfabServer(t, body)

	src := &DcfabSource{
		cfg: DcfabConfig{
			Region:   "wdl1",
			Endpoint: srv.URL,
			PreferIP: "ipv6",
		},
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Devices) != 2 {
		t.Fatalf("device count: got %d want 2", len(inv.Devices))
	}
	// IPv6-preferring device picks the v6 address; the v4-only device
	// falls back to v4.
	if inv.Devices[0].Host != "fc00:800f:f01::8" {
		t.Errorf("device 0 host: got %q want fc00:800f:f01::8", inv.Devices[0].Host)
	}
	if inv.Devices[1].Host != "10.0.0.9" {
		t.Errorf("device 1 host: got %q want 10.0.0.9", inv.Devices[1].Host)
	}
	// Tags include role + platform + region.
	if !containsString(inv.Devices[0].Tags, "role:fm") {
		t.Errorf("tags missing role:fm: %v", inv.Devices[0].Tags)
	}
	if !containsString(inv.Devices[0].Tags, "platform:eos") {
		t.Errorf("tags missing platform:eos: %v", inv.Devices[0].Tags)
	}
	if !containsString(inv.Devices[0].Tags, "region:wdl1") {
		t.Errorf("tags missing region:wdl1: %v", inv.Devices[0].Tags)
	}
}

func TestDcfabSource_LoadErrorsAtPaginationCap(t *testing.T) {
	// Build a response with exactly 5000 devices to trigger the cap.
	var b strings.Builder
	b.WriteString(`{"data":{"region":{"devices":[`)
	for i := 0; i < 5000; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"name":"r%d","role":"x","platform":"y","managementInterface":{"addresses":[{"address":"10.0.0.%d/32","version":4}]}}`, i, i%256)
	}
	b.WriteString(`]}}}`)
	srv := mockDcfabServer(t, b.String())

	src := &DcfabSource{cfg: DcfabConfig{Region: "x", Endpoint: srv.URL}}
	_, err := src.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "5000") {
		t.Errorf("expected 5000-device cap error, got %v", err)
	}
}

func TestDcfabSource_LoadSkipsDevicesWithNoAddress(t *testing.T) {
	body := `{
	  "data": {
	    "region": {
	      "devices": [
	        {"name": "noaddr", "role": "fm", "platform": "eos", "managementInterface": null},
	        {"name": "ok", "role": "fm", "platform": "eos",
	         "managementInterface": {"addresses": [{"address": "10.0.0.1/24", "version": 4}]}}
	      ]
	    }
	  }
	}`
	srv := mockDcfabServer(t, body)
	src := &DcfabSource{cfg: DcfabConfig{Region: "x", Endpoint: srv.URL}}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "ok" {
		t.Errorf("expected only ok device, got %+v", inv.Devices)
	}
}

func TestDcfabSource_LoadHandlesGraphQLError(t *testing.T) {
	body := `{"errors": [{"message": "region not found"}]}`
	srv := mockDcfabServer(t, body)
	src := &DcfabSource{cfg: DcfabConfig{Region: "nope", Endpoint: srv.URL}}
	_, err := src.Load(context.Background())
	if err == nil || !strings.Contains(err.Error(), "region not found") {
		t.Errorf("expected GraphQL error to be surfaced, got %v", err)
	}
}

func containsString(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
