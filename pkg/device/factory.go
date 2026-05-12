package device

import "fmt"

// New constructs a Device of the configured transport. This is the
// recommended entry point for the CLI and inventory layers; concrete
// constructors (NewEOSDevice, NewGNMIDevice) are exposed for tests but
// callers should prefer New so a single switch governs default ports
// and validation.
func New(cfg DeviceConfig) (Device, error) {
	switch cfg.Transport {
	case "", "eapi":
		if cfg.Port == 0 {
			cfg.Port = 443
		}
		return NewEOSDevice(cfg), nil
	case "gnmi":
		return nil, fmt.Errorf("gnmi transport not yet implemented")
	default:
		return nil, fmt.Errorf("unknown transport %q (supported: eapi, gnmi)", cfg.Transport)
	}
}
