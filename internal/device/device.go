package device

import (
	"context"
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
	Password       string            `yaml:"password" json:"password"`
	EnablePassword string            `yaml:"enable_password,omitempty" json:"enable_password,omitempty"`
	Tags           []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Timeout        time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Insecure       bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
	DisableCache   bool              `yaml:"disable_cache,omitempty" json:"disable_cache,omitempty"`
	Extra          map[string]string `yaml:"extra,omitempty" json:"extra,omitempty"`
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
	return d.State == ConnectionStateConnected || d.State == ConnectionStateEstablished
}

func (d *BaseDevice) IsEstablished() bool {
	return d.State == ConnectionStateEstablished
}

func (d *BaseDevice) HardwareModel() string {
	return d.Model
}