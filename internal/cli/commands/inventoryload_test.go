package commands

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/fluidstackio/go-anta/pkg/inventory"
)

func TestLoadInventoryForRun_StaticFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/inv.yaml"
	body := `
kind: file
devices:
  - name: r1
    host: 10.0.0.1
    username: admin
    password: pw
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	opts := InventoryLoadOptions{Path: path}
	inv, err := LoadInventoryForRun(context.Background(), opts)
	if err != nil {
		t.Fatalf("LoadInventoryForRun: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "r1" {
		t.Errorf("got %+v", inv.Devices)
	}
}

func TestLoadInventoryForRun_RejectsUnknownSourceFlag(t *testing.T) {
	opts := InventoryLoadOptions{Path: "/nope", SourceOverride: "smtp"}
	_, err := LoadInventoryForRun(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("expected unknown-source error, got %v", err)
	}
}

func TestLoadInventoryForRun_RequiresOneOfPathOrNetboxURL(t *testing.T) {
	// Clear NETBOX_URL env to ensure the test isn't affected by ambient state.
	t.Setenv("NETBOX_URL", "")
	_, err := LoadInventoryForRun(context.Background(), InventoryLoadOptions{})
	if err == nil || !strings.Contains(err.Error(), "inventory") {
		t.Errorf("expected missing-inventory error, got %v", err)
	}
}

func TestLoadInventoryForRun_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/inv.yaml"
	body := `
kind: file
devices:
  - name: r1
    host: 10.0.0.1
    username: admin
    password: pw
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	opts := InventoryLoadOptions{
		Path:     path,
		Defaults: inventory.DeviceDefaults{Transport: "gnmi"},
	}
	inv, err := LoadInventoryForRun(context.Background(), opts)
	if err != nil {
		t.Fatalf("LoadInventoryForRun: %v", err)
	}
	if inv.Devices[0].Transport != "gnmi" {
		t.Errorf("Transport: got %q want gnmi", inv.Devices[0].Transport)
	}
}

func TestParseNetboxQueryString_BasicFields(t *testing.T) {
	q := parseNetboxQueryString("site=dc1,role=leaf,name=r1")
	if q.Site != "dc1" {
		t.Errorf("Site: got %q want dc1", q.Site)
	}
	if q.Role != "leaf" {
		t.Errorf("Role: got %q want leaf", q.Role)
	}
	if q.Name != "r1" {
		t.Errorf("Name: got %q want r1", q.Name)
	}
}

func TestParseNetboxQueryString_IDFields(t *testing.T) {
	q := parseNetboxQueryString("site_id=42,role_id=7,device_type_id=99")
	if q.SiteID != 42 {
		t.Errorf("SiteID: got %d want 42", q.SiteID)
	}
	if q.RoleID != 7 {
		t.Errorf("RoleID: got %d want 7", q.RoleID)
	}
	if q.DeviceTypeID != 99 {
		t.Errorf("DeviceTypeID: got %d want 99", q.DeviceTypeID)
	}
}

func TestParseNetboxQueryString_Tags(t *testing.T) {
	q := parseNetboxQueryString("tag=alpha,tag=beta")
	if len(q.Tags) != 2 || q.Tags[0] != "alpha" || q.Tags[1] != "beta" {
		t.Errorf("Tags: got %v", q.Tags)
	}
}

func TestParseNetboxQueryString_Empty(t *testing.T) {
	q := parseNetboxQueryString("")
	if q.Site != "" || q.Role != "" || len(q.Tags) != 0 {
		t.Errorf("expected zero query, got %+v", q)
	}
}
