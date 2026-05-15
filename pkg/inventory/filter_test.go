package inventory

import (
	"strings"
	"testing"

	"github.com/fluidstackio/go-anta/pkg/device"
)

func mkInv(devs ...device.DeviceConfig) *Inventory {
	return &Inventory{Devices: devs}
}

func TestFilterByNames_ReportsMissing(t *testing.T) {
	inv := mkInv(
		device.DeviceConfig{Name: "leaf1", Host: "10.0.0.1"},
		device.DeviceConfig{Name: "leaf2", Host: "10.0.0.2"},
	)
	got, err := inv.FilterByNames([]string{"leaf1", "leaf3"})
	if err == nil {
		t.Fatal("expected error for missing 'leaf3'")
	}
	if !strings.Contains(err.Error(), "leaf3") {
		t.Errorf("error should name 'leaf3', got: %v", err)
	}
	if len(got.Devices) != 1 || got.Devices[0].Name != "leaf1" {
		t.Errorf("expected single match leaf1, got %+v", got.Devices)
	}
}

func TestFilterByTags_ReportsMissing(t *testing.T) {
	inv := mkInv(
		device.DeviceConfig{Name: "a", Tags: []string{"lab"}},
		device.DeviceConfig{Name: "b", Tags: []string{"prod"}},
	)
	_, err := inv.FilterByTags([]string{"lab", "qa"})
	if err == nil {
		t.Fatal("expected error for tag 'qa' with zero matches")
	}
	if !strings.Contains(err.Error(), "qa") {
		t.Errorf("error should name 'qa', got: %v", err)
	}
}

func TestFilterByLimit_EmptyResultIsError(t *testing.T) {
	inv := mkInv(
		device.DeviceConfig{Name: "leaf1"},
		device.DeviceConfig{Name: "leaf2"},
	)
	_, err := inv.FilterByLimit("nosuchhost")
	if err == nil {
		t.Fatal("expected error for unmatched limit pattern")
	}
	if !strings.Contains(err.Error(), "nosuchhost") {
		t.Errorf("error should name the pattern, got: %v", err)
	}
}

func TestFilterByLimit_WildcardMatch(t *testing.T) {
	inv := mkInv(
		device.DeviceConfig{Name: "leaf1"},
		device.DeviceConfig{Name: "leaf2"},
		device.DeviceConfig{Name: "spine1"},
	)
	got, err := inv.FilterByLimit("leaf*")
	if err != nil {
		t.Errorf("expected nil err for matching pattern, got %v", err)
	}
	if len(got.Devices) != 2 {
		t.Errorf("expected 2 matches for leaf*, got %d", len(got.Devices))
	}
}

func TestFilterByNames_EmptyPassesThrough(t *testing.T) {
	inv := mkInv(device.DeviceConfig{Name: "a"})
	got, err := inv.FilterByNames(nil)
	if err != nil || got != inv {
		t.Errorf("empty filter should pass through; got %v, %v", got, err)
	}
}
