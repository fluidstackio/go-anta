package device

import (
	"context"
	"fmt"
	"time"

	gnmiapi "github.com/openconfig/gnmic/pkg/api"
	gnmitarget "github.com/openconfig/gnmic/pkg/api/target"

	"github.com/fluidstackio/go-anta/internal/logger"
)

// GNMIDevice implements the Device interface against an Arista EOS
// device using gNMI gRPC with origin=cli Get requests. The JSON shape
// inside the response matches eAPI exactly (after wrapper-stripping
// via unwrapCLIResponse), so tests written against EOSDevice work
// unchanged against GNMIDevice.
type GNMIDevice struct {
	BaseDevice
	target *gnmitarget.Target
	cache  *CommandCache
}

// NewGNMIDevice constructs a gNMI-backed device. Callers should prefer
// device.New(cfg) which handles default ports and dispatch.
func NewGNMIDevice(config DeviceConfig) *GNMIDevice {
	if config.Port == 0 {
		config.Port = 6030
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	dev := &GNMIDevice{
		BaseDevice: BaseDevice{
			Config: config,
			State:  ConnectionStateClosed,
		},
	}
	if !config.DisableCache {
		dev.cache = NewCommandCache(128, 60*time.Second)
	}
	return dev
}

func (d *GNMIDevice) Connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.State == ConnectionStateConnected || d.State == ConnectionStateEstablished {
		return nil
	}

	d.State = ConnectionStateConnecting

	addr := fmt.Sprintf("%s:%d", d.Config.Host, d.Config.Port)
	opts := []gnmiapi.TargetOption{
		gnmiapi.Address(addr),
		gnmiapi.Username(d.Config.Username),
		gnmiapi.Password(d.Config.Password),
		gnmiapi.Timeout(d.Config.Timeout),
	}
	if d.Config.Insecure {
		opts = append(opts, gnmiapi.SkipVerify(true))
	}

	target, err := gnmiapi.NewTarget(opts...)
	if err != nil {
		d.State = ConnectionStateError
		return fmt.Errorf("build gNMI target for %s: %w", d.Config.Name, err)
	}
	if err := target.CreateGNMIClient(ctx); err != nil {
		d.State = ConnectionStateError
		return fmt.Errorf("dial gNMI for %s: %w", d.Config.Name, err)
	}
	d.target = target
	d.State = ConnectionStateConnected
	d.ConnectionTime = time.Now()

	// Probe with show version (matches EOSDevice.Connect behavior) so
	// IsEstablished() / HardwareModel() have the expected post-Connect
	// invariants. Execute itself does not yet exist; for now we just
	// transition state without populating Model. Task 5 will replace
	// this with a real probe.
	d.State = ConnectionStateEstablished
	logger.Infof("Successfully connected to %s via gNMI", d.Config.Name)
	return nil
}

func (d *GNMIDevice) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.State = ConnectionStateClosed
	if d.cache != nil {
		d.cache.Clear()
	}
	if d.target != nil {
		_ = d.target.Close()
		d.target = nil
	}
	return nil
}

// Execute, ExecuteBatch, and Refresh return explicit errors until Tasks 5
// and 6 implement them. Returning an error makes accidental use during
// development obvious.

func (d *GNMIDevice) Execute(ctx context.Context, cmd Command) (*CommandResult, error) {
	return nil, fmt.Errorf("GNMIDevice.Execute not yet implemented")
}

func (d *GNMIDevice) ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error) {
	return nil, fmt.Errorf("GNMIDevice.ExecuteBatch not yet implemented")
}

func (d *GNMIDevice) Refresh(ctx context.Context) error {
	return fmt.Errorf("GNMIDevice.Refresh not yet implemented")
}
