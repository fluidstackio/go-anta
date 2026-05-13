package device

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Device interface {
	Name() string
	Host() string
	Tags() []string
	Connect(ctx context.Context) error
	Disconnect() error
	Execute(ctx context.Context, cmd Command) (*CommandResult, error)
	ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error)
	IsOnline() bool
	IsEstablished() bool
	HardwareModel() string
	Refresh(ctx context.Context) error
}

type Command struct {
	Template string                 `yaml:"template" json:"template"`
	Params   map[string]interface{} `yaml:"params,omitempty" json:"params,omitempty"`
	Version  string                 `yaml:"version,omitempty" json:"version,omitempty"`
	Revision int                    `yaml:"revision,omitempty" json:"revision,omitempty"`
	Format   string                 `yaml:"format,omitempty" json:"format,omitempty"`
	UseCache bool                   `yaml:"use_cache,omitempty" json:"use_cache,omitempty"`
}

type CommandResult struct {
	Command   Command       `json:"command"`
	Output    interface{}   `json:"output"`
	Error     error         `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Cached    bool          `json:"cached"`
}

type DeviceConfig struct {
	Name           string            `yaml:"name" json:"name"`
	Host           string            `yaml:"host" json:"host"`
	Port           int               `yaml:"port,omitempty" json:"port,omitempty"`
	Username       string            `yaml:"username" json:"username"`
	Password       string            `yaml:"password" json:"-"`
	EnablePassword string            `yaml:"enable_password,omitempty" json:"-"`
	Tags           []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Timeout        time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Insecure       bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
	Plaintext      bool              `yaml:"plaintext,omitempty" json:"plaintext,omitempty"`
	Transport      string            `yaml:"transport,omitempty" json:"transport,omitempty"`
	DisableCache   bool              `yaml:"disable_cache,omitempty" json:"disable_cache,omitempty"`
	Extra          map[string]string `yaml:"extra,omitempty" json:"extra,omitempty"`
}

// String returns a redacted representation of DeviceConfig that omits
// Password and EnablePassword. This is what fmt.Sprintf("%v", cfg) and
// any logger call using `%v` will produce, so credentials cannot leak
// through unintended `logger.Debugf("config: %+v", cfg)` calls.
func (c DeviceConfig) String() string {
	return fmt.Sprintf(
		"DeviceConfig{Name:%s Host:%s Port:%d Username:%s Password:[REDACTED] EnablePassword:%s Tags:%v Timeout:%s Insecure:%t Transport:%s}",
		c.Name, c.Host, c.Port, c.Username, redactedIfSet(c.EnablePassword), c.Tags, c.Timeout, c.Insecure, c.Transport,
	)
}

// GoString covers the %#v format verb the same way.
func (c DeviceConfig) GoString() string { return c.String() }

// MarshalJSON shadows json:"-" tags above as belt-and-suspenders; the
// tags alone already drop the fields, but this prevents accidental
// re-introduction.
func (c DeviceConfig) MarshalJSON() ([]byte, error) {
	type alias DeviceConfig
	return json.Marshal(struct {
		alias
		Password       string `json:"password,omitempty"`
		EnablePassword string `json:"enable_password,omitempty"`
	}{
		alias: alias(c),
		// Password and EnablePassword intentionally empty in the output.
	})
}

func redactedIfSet(s string) string {
	if s == "" {
		return ""
	}
	return "[REDACTED]"
}

type ConnectionState int

const (
	ConnectionStateClosed ConnectionState = iota
	ConnectionStateConnecting
	ConnectionStateConnected
	ConnectionStateEstablished
	ConnectionStateError
)

func (s ConnectionState) String() string {
	switch s {
	case ConnectionStateClosed:
		return "closed"
	case ConnectionStateConnecting:
		return "connecting"
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateEstablished:
		return "established"
	case ConnectionStateError:
		return "error"
	default:
		return "unknown"
	}
}

type BaseDevice struct {
	Config         DeviceConfig
	State          ConnectionState
	Model          string
	LastRefresh    time.Time
	ConnectionTime time.Time

	// mu protects State, Model, LastRefresh, and ConnectionTime against
	// concurrent reads (from accessor methods and Execute) and writes
	// (from Connect/Disconnect/Refresh). Config and its sub-fields are
	// immutable after construction and don't require the lock.
	mu sync.RWMutex
}

func (d *BaseDevice) Name() string {
	return d.Config.Name
}

func (d *BaseDevice) Host() string {
	return d.Config.Host
}

func (d *BaseDevice) Tags() []string {
	return d.Config.Tags
}

func (d *BaseDevice) IsOnline() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.State == ConnectionStateConnected || d.State == ConnectionStateEstablished
}

func (d *BaseDevice) IsEstablished() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.State == ConnectionStateEstablished
}

func (d *BaseDevice) HardwareModel() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.Model
}