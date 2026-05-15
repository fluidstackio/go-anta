package inventory

import (
	"testing"

	"github.com/fluidstackio/go-anta/pkg/device"
)

func TestApplyDefaults_OnlyFillsEmpty(t *testing.T) {
	inv := &Inventory{
		Devices: []device.DeviceConfig{
			{Name: "r1", Host: "10.0.0.1"},                                  // missing creds
			{Name: "r2", Host: "10.0.0.2", Username: "ops", Password: "p2"}, // pre-filled
		},
	}

	out := inv.ApplyDefaults(DeviceDefaults{
		Username:  "admin",
		Password:  "secret",
		Transport: "gnmi",
		Port:      6030,
		Plaintext: true,
	})

	// r1 should pick up everything from defaults.
	if out.Devices[0].Username != "admin" || out.Devices[0].Password != "secret" {
		t.Errorf("r1 creds: got %q/%q, want admin/secret",
			out.Devices[0].Username, out.Devices[0].Password)
	}
	if out.Devices[0].Transport != "gnmi" || out.Devices[0].Port != 6030 {
		t.Errorf("r1 transport/port: got %q/%d, want gnmi/6030",
			out.Devices[0].Transport, out.Devices[0].Port)
	}
	if !out.Devices[0].Plaintext {
		t.Error("r1 plaintext: expected true")
	}

	// r2 should keep its own creds, defaults must not override.
	if out.Devices[1].Username != "ops" || out.Devices[1].Password != "p2" {
		t.Errorf("r2 creds were overwritten: got %q/%q",
			out.Devices[1].Username, out.Devices[1].Password)
	}
	// Transport was empty on r2, so default should fill it.
	if out.Devices[1].Transport != "gnmi" {
		t.Errorf("r2 transport: got %q, want gnmi", out.Devices[1].Transport)
	}
}

func TestApplyDefaults_ReturnsNewInventoryNotMutated(t *testing.T) {
	inv := &Inventory{
		Devices: []device.DeviceConfig{{Name: "r1", Host: "10.0.0.1"}},
	}
	_ = inv.ApplyDefaults(DeviceDefaults{Username: "admin"})
	if inv.Devices[0].Username != "" {
		t.Errorf("input mutated: Username=%q", inv.Devices[0].Username)
	}
}

func TestApplyDefaults_PreservesNetworksAndRanges(t *testing.T) {
	inv := &Inventory{
		Devices:  []device.DeviceConfig{{Name: "r1", Host: "10.0.0.1"}},
		Networks: []NetworkDefinition{{Network: "10.0.0.0/24", Username: "x"}},
		Ranges:   []RangeDefinition{{Start: "10.0.1.1", End: "10.0.1.5"}},
	}
	out := inv.ApplyDefaults(DeviceDefaults{})
	if len(out.Networks) != 1 || len(out.Ranges) != 1 {
		t.Errorf("networks/ranges dropped: %+v / %+v", out.Networks, out.Ranges)
	}
}
