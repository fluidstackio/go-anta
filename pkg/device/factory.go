package device

import (
	"errors"
	"fmt"
)

// New constructs a Device of the configured transport. Use this as the
// entry point from CLI/inventory code so a single switch governs which
// transport handles each device. The concrete constructors
// (NewEOSDevice, NewGNMIDevice) own their own port and timeout defaults.
func New(cfg DeviceConfig) (Device, error) {
	switch cfg.Transport {
	case "", "eapi":
		return NewEOSDevice(cfg), nil
	case "gnmi":
		return nil, errors.New("gnmi transport not yet implemented")
	default:
		return nil, fmt.Errorf("unknown transport %q (supported: eapi, gnmi)", cfg.Transport)
	}
}
