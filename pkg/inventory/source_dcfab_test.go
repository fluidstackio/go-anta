package inventory

import (
	"context"
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
roles: [fm, ft]
platforms: [eos]
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
	if len(d.cfg.Roles) != 2 || d.cfg.Roles[0] != "fm" {
		t.Errorf("roles not parsed: %v", d.cfg.Roles)
	}
}

func TestDcfabSource_FactoryRejectsMissingRegion(t *testing.T) {
	tmp := writeYAML(t, `
kind: dcfab
env: prod
`)
	_, err := LoadSource(tmp)
	if err == nil || !strings.Contains(err.Error(), "region") {
		t.Errorf("expected region-required error, got %v", err)
	}
}

func TestDcfabSource_QueryURLEncodesAllFilters(t *testing.T) {
	src := &DcfabSource{
		cfg: DcfabConfig{
			Region:    "wdl1",
			Roles:     []string{"fm", "ft"},
			Platforms: []string{"eos"},
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
	for _, want := range []string{`region(name: "wdl1")`, `roles: ["fm","ft"]`, `platforms: ["eos"]`, `implementation: ACTIVE`, `managementInterface`} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q\nfull query: %s", want, q)
		}
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
		got := dcfabEndpoint(DcfabConfig{Env: tc.env})
		if got != tc.want {
			t.Errorf("env=%q: got %q want %q", tc.env, got, tc.want)
		}
	}
	// Explicit Endpoint wins.
	custom := dcfabEndpoint(DcfabConfig{Env: "prod", Endpoint: "https://custom.test"})
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

// silence "imported and not used" before Task 8 adds Load tests.
var _ context.Context = context.TODO()
