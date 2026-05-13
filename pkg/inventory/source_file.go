package inventory

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/fluidstackio/go-anta/pkg/device"
)

// FileSource produces an Inventory from a static YAML file: an explicit
// device list, plus optional `networks:` and `ranges:` blocks that get
// expanded into device entries at Load time.
type FileSource struct {
	devices  []deviceEntry
	networks []NetworkDefinition
	ranges   []RangeDefinition
}

// deviceEntry mirrors DeviceConfig but is used at YAML decode time so
// we can keep the unmarshaling localized to this file.
type deviceEntry struct {
	Name           string            `yaml:"name"`
	Host           string            `yaml:"host"`
	Port           int               `yaml:"port,omitempty"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	EnablePassword string            `yaml:"enable_password,omitempty"`
	Tags           []string          `yaml:"tags,omitempty"`
	Insecure       bool              `yaml:"insecure,omitempty"`
	Plaintext      bool              `yaml:"plaintext,omitempty"`
	Transport      string            `yaml:"transport,omitempty"`
	DisableCache   bool              `yaml:"disable_cache,omitempty"`
	Extra          map[string]string `yaml:"extra,omitempty"`
}

func (e deviceEntry) toConfig() device.DeviceConfig {
	return device.DeviceConfig{
		Name:           e.Name,
		Host:           e.Host,
		Port:           e.Port,
		Username:       e.Username,
		Password:       e.Password,
		EnablePassword: e.EnablePassword,
		Tags:           e.Tags,
		Insecure:       e.Insecure,
		Plaintext:      e.Plaintext,
		Transport:      e.Transport,
		DisableCache:   e.DisableCache,
		Extra:          e.Extra,
	}
}

// Kind returns the registered name for this source type.
func (s *FileSource) Kind() string { return "file" }

// Load expands networks and ranges, converts entries to DeviceConfig, and
// validates the resulting Inventory.
func (s *FileSource) Load(ctx context.Context) (*Inventory, error) {
	inv := &Inventory{
		Networks: s.networks,
		Ranges:   s.ranges,
	}
	for _, d := range s.devices {
		inv.Devices = append(inv.Devices, d.toConfig())
	}
	if err := inv.expandNetworks(); err != nil {
		return nil, fmt.Errorf("expand networks: %w", err)
	}
	if err := inv.expandRanges(); err != nil {
		return nil, fmt.Errorf("expand ranges: %w", err)
	}
	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("validate inventory: %w", err)
	}
	return inv, nil
}

func init() {
	RegisterSource("file", func(node *yaml.Node) (Source, error) {
		var cfg struct {
			Kind     string              `yaml:"kind"`
			Devices  []deviceEntry       `yaml:"devices"`
			Networks []NetworkDefinition `yaml:"networks"`
			Ranges   []RangeDefinition   `yaml:"ranges"`
		}
		if err := node.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("file source: decode YAML: %w", err)
		}
		return &FileSource{
			devices:  cfg.Devices,
			networks: cfg.Networks,
			ranges:   cfg.Ranges,
		}, nil
	})
}
