package device

import (
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
			gotType := stringifyType(dev)
			if gotType != tc.wantConcrete {
				t.Errorf("concrete type: got %s, want %s", gotType, tc.wantConcrete)
			}
			if eos, ok := dev.(*EOSDevice); ok {
				if eos.Config.Port != tc.wantPort {
					t.Errorf("port: got %d, want %d", eos.Config.Port, tc.wantPort)
				}
			}
		})
	}
}

func stringifyType(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return "*device." + typeName(v)
}

func typeName(v interface{}) string {
	switch v.(type) {
	case *EOSDevice:
		return "EOSDevice"
	default:
		return "Unknown"
	}
}
