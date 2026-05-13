package device

import (
	"fmt"
	"strings"
	"testing"
)

func TestNew_DispatchAndDefaults(t *testing.T) {
	tests := []struct {
		name         string
		cfg          DeviceConfig
		wantPort     int
		wantErrSub   string
		wantConcrete string
	}{
		{
			name:         "default transport is eapi",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1"},
			wantPort:     443,
			wantConcrete: "*device.EOSDevice",
		},
		{
			name:         "explicit eapi transport",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "eapi"},
			wantPort:     443,
			wantConcrete: "*device.EOSDevice",
		},
		{
			name:         "explicit port wins over default",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Port: 8443, Transport: "eapi"},
			wantPort:     8443,
			wantConcrete: "*device.EOSDevice",
		},
		{
			name:       "unknown transport errors",
			cfg:        DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "smtp"},
			wantErrSub: "unknown transport",
		},
		{
			name:         "gnmi transport with default port",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "gnmi"},
			wantPort:     6030,
			wantConcrete: "*device.GNMIDevice",
		},
		{
			name:         "gnmi transport with explicit port",
			cfg:          DeviceConfig{Name: "d1", Host: "10.0.0.1", Transport: "gnmi", Port: 9339},
			wantPort:     9339,
			wantConcrete: "*device.GNMIDevice",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dev, err := New(tc.cfg)
			if tc.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSub, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			gotType := fmt.Sprintf("%T", dev)
			if gotType != tc.wantConcrete {
				t.Errorf("concrete type: got %s, want %s", gotType, tc.wantConcrete)
			}
			switch d := dev.(type) {
			case *EOSDevice:
				if d.Config.Port != tc.wantPort {
					t.Errorf("port: got %d, want %d", d.Config.Port, tc.wantPort)
				}
			case *GNMIDevice:
				if d.Config.Port != tc.wantPort {
					t.Errorf("port: got %d, want %d", d.Config.Port, tc.wantPort)
				}
			}
		})
	}
}
