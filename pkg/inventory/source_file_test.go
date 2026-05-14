package inventory

import (
	"context"
	"testing"
	"time"
)

func TestFileSource_LoadsDevices(t *testing.T) {
	tmp := writeYAML(t, `
kind: file
devices:
  - name: r1
    host: 192.0.2.10
    username: admin
    password: pw
  - name: r2
    host: 192.0.2.11
    username: admin
    password: pw
`)

	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	if src.Kind() != "file" {
		t.Fatalf("Kind: got %q want file", src.Kind())
	}

	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(inv.Devices), 2; got != want {
		t.Errorf("device count: got %d want %d", got, want)
	}
	if inv.Devices[0].Name != "r1" {
		t.Errorf("first device name: got %q want r1", inv.Devices[0].Name)
	}
}

func TestFileSource_NoKindBackwardCompat(t *testing.T) {
	tmp := writeYAML(t, `
devices:
  - name: r1
    host: 192.0.2.10
    username: admin
    password: pw
`)

	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "r1" {
		t.Errorf("expected one device r1; got %v", inv.Devices)
	}
}

func TestFileSource_NetworksExpand(t *testing.T) {
	tmp := writeYAML(t, `
kind: file
networks:
  - network: 192.0.2.0/30
    username: admin
    password: pw
`)
	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// /30 has 2 host IPs (excluding network + broadcast).
	if len(inv.Devices) != 2 {
		t.Errorf("/30 should expand to 2 host devices, got %d", len(inv.Devices))
	}
}

func TestFileSource_TimeoutPropagates(t *testing.T) {
	tmp := writeYAML(t, `
kind: file
devices:
  - name: r1
    host: 192.0.2.10
    username: admin
    password: pw
    timeout: 7s
`)
	src, err := LoadSource(tmp)
	if err != nil {
		t.Fatalf("LoadSource: %v", err)
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := inv.Devices[0].Timeout; got != 7*time.Second {
		t.Errorf("Timeout: got %s want 7s", got)
	}
}

func TestFileSource_LoadInventoryStillWorks(t *testing.T) {
	tmp := writeYAML(t, `
devices:
  - name: r1
    host: 192.0.2.10
    username: admin
    password: pw
`)
	inv, err := LoadInventory(tmp)
	if err != nil {
		t.Fatalf("LoadInventory: %v", err)
	}
	if len(inv.Devices) != 1 || inv.Devices[0].Name != "r1" {
		t.Errorf("LoadInventory backward compat broken: %v", inv.Devices)
	}
}
