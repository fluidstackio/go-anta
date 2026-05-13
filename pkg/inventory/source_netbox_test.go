package inventory

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
