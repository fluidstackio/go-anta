package inventory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/fluidstackio/go-anta/pkg/device"
)

// NetboxSource implements Source by fetching devices from a Netbox
// instance via its REST API. Configured via the `kind: netbox` YAML
// schema (or the legacy top-level `netbox:` block).
type NetboxSource struct {
	config   NetboxConfig
	query    NetboxQuery
	defaults DeviceDefaults
}

// Kind returns the registered source kind "netbox".
func (s *NetboxSource) Kind() string { return "netbox" }

// Load queries Netbox and returns the resulting Inventory. Credentials
// are NOT populated on the returned devices; the caller is expected to
// run Inventory.ApplyDefaults afterward.
func (s *NetboxSource) Load(ctx context.Context) (*Inventory, error) {
	client := NewNetboxClient(s.config)
	devices, err := client.GetDevices(ctx, s.query)
	if err != nil {
		return nil, fmt.Errorf("netbox: fetch devices: %w", err)
	}

	inv := &Inventory{}
	for _, d := range devices {
		cfg := netboxDeviceToConfig(d, s.defaults)
		if cfg.Name == "" || cfg.Host == "" {
			continue
		}
		inv.Devices = append(inv.Devices, cfg)
	}
	return inv, nil
}

// netboxDeviceToConfig maps a Netbox API response into a DeviceConfig.
// Credentials come from defaults; the call site will typically also
// invoke Inventory.ApplyDefaults afterward to fill in env/CLI values.
//
// IP resolution order: PrimaryIP → PrimaryIP4 → PrimaryIP6.
// Tags use tag.Name (matching existing LoadFromNetbox behaviour).
func netboxDeviceToConfig(d NetboxDevice, defaults DeviceDefaults) device.DeviceConfig {
	host := ""
	if d.PrimaryIP != nil && d.PrimaryIP.Address != "" {
		host = stripCIDR(d.PrimaryIP.Address)
	} else if d.PrimaryIP4 != nil && d.PrimaryIP4.Address != "" {
		host = stripCIDR(d.PrimaryIP4.Address)
	} else if d.PrimaryIP6 != nil && d.PrimaryIP6.Address != "" {
		host = stripCIDR(d.PrimaryIP6.Address)
	}

	tags := make([]string, 0, len(d.Tags)+4)
	for _, tag := range d.Tags {
		tags = append(tags, tag.Name)
	}
	if d.Site.Name != "" {
		tags = append(tags, "site:"+d.Site.Slug)
	}
	if d.DeviceRole.Name != "" {
		tags = append(tags, "role:"+d.DeviceRole.Slug)
	}
	if d.Platform != nil && d.Platform.Name != "" {
		tags = append(tags, "platform:"+d.Platform.Slug)
	}
	if d.DeviceType.Manufacturer.Name != "" {
		tags = append(tags, "vendor:"+d.DeviceType.Manufacturer.Slug)
	}

	extra := map[string]string{
		"netbox_id":    fmt.Sprintf("%d", d.ID),
		"site":         d.Site.Name,
		"role":         d.DeviceRole.Name,
		"device_type":  d.DeviceType.Model,
		"manufacturer": d.DeviceType.Manufacturer.Name,
	}
	if d.Platform != nil {
		extra["platform"] = d.Platform.Name
	}
	if d.Serial != "" {
		extra["serial"] = d.Serial
	}

	return device.DeviceConfig{
		Name:           d.Name,
		Host:           host,
		Tags:           tags,
		Extra:          extra,
		Username:       defaults.Username,
		Password:       defaults.Password,
		EnablePassword: defaults.EnablePassword,
		Timeout:        defaults.Timeout,
		Transport:      defaults.Transport,
		Port:           defaults.Port,
		Insecure:       defaults.Insecure,
		Plaintext:      defaults.Plaintext,
	}
}

// stripCIDR removes a /NN suffix from an IP, returning just the address.
// Netbox primary_ip fields come back as "10.0.0.1/24" style strings.
func stripCIDR(s string) string {
	if i := strings.IndexByte(s, '/'); i >= 0 {
		return s[:i]
	}
	return s
}

// NewNetboxSource constructs a NetboxSource directly (without going
// through the YAML loader). Used by CLI code that synthesizes a source
// from --netbox-url / --netbox-query flags.
func NewNetboxSource(config NetboxConfig, query NetboxQuery) *NetboxSource {
	return &NetboxSource{config: config, query: query}
}

// LoadFromNetbox is a back-compat wrapper around NetboxSource. New
// callers should construct a NetboxSource and call Load directly so
// they can pass a context.
func LoadFromNetbox(config NetboxConfig, query NetboxQuery, credentials map[string]interface{}) (*Inventory, error) {
	defaults := DeviceDefaults{}
	if v, ok := credentials["username"].(string); ok {
		defaults.Username = v
	}
	if v, ok := credentials["password"].(string); ok {
		defaults.Password = v
	}
	if v, ok := credentials["enable_password"].(string); ok {
		defaults.EnablePassword = v
	}
	if v, ok := credentials["timeout"].(string); ok {
		if dur, err := time.ParseDuration(v); err == nil {
			defaults.Timeout = dur
		}
	}
	if v, ok := credentials["insecure"].(bool); ok {
		defaults.Insecure = v
	}
	if v, ok := credentials["port"].(int); ok {
		defaults.Port = v
	}
	src := &NetboxSource{config: config, query: query, defaults: defaults}
	inv, err := src.Load(context.Background())
	if err != nil {
		return nil, err
	}
	if len(inv.Devices) == 0 {
		return nil, fmt.Errorf("no devices with primary IP found in netbox response")
	}
	return inv, nil
}

// NetboxInventoryConfig defines the structure for a legacy Netbox
// inventory configuration file (top-level `netbox:` block). Kept for
// callers that construct the struct directly; LoadNetboxInventory is the
// entry point for file-based loading.
type NetboxInventoryConfig struct {
	Netbox      NetboxConfig           `yaml:"netbox" json:"netbox"`
	Query       NetboxQuery            `yaml:"query" json:"query"`
	Credentials map[string]interface{} `yaml:"credentials,omitempty" json:"credentials,omitempty"`
}

// LoadNetboxInventory is a back-compat wrapper that reads a legacy
// inventory YAML (with a top-level netbox: block) and uses NetboxSource
// under the hood. NETBOX_URL / NETBOX_TOKEN env vars override the
// values in the file if set (matching legacy behavior).
func LoadNetboxInventory(path string) (*Inventory, error) {
	src, err := LoadSource(path)
	if err != nil {
		return nil, err
	}
	nb, ok := src.(*NetboxSource)
	if !ok {
		return nil, fmt.Errorf("inventory %s: expected netbox source, got %s", path, src.Kind())
	}
	// Env-var overrides (legacy behavior).
	if v := os.Getenv("NETBOX_URL"); v != "" {
		nb.config.URL = v
	}
	if v := os.Getenv("NETBOX_TOKEN"); v != "" {
		nb.config.Token = v
	}
	// Validate required fields with clear messages (better DX than waiting
	// for the HTTP layer to fail).
	if nb.config.URL == "" {
		return nil, fmt.Errorf("inventory %s: netbox URL is required (set in YAML or NETBOX_URL env)", path)
	}
	if nb.config.Token == "" {
		return nil, fmt.Errorf("inventory %s: netbox token is required (set in YAML or NETBOX_TOKEN env)", path)
	}
	// Re-parse credentials from the YAML file to apply as device defaults,
	// matching legacy behavior where LoadNetboxInventory delegated to
	// LoadFromNetbox which extracted credentials from the parsed config.
	nb.defaults, err = loadNetboxCredentialsFromYAML(path)
	if err != nil {
		return nil, fmt.Errorf("inventory %s: parse credentials: %w", path, err)
	}
	return nb.Load(context.Background())
}

// loadNetboxCredentialsFromYAML parses the top-level credentials block from a
// legacy Netbox inventory YAML and returns it as a DeviceDefaults value.
func loadNetboxCredentialsFromYAML(path string) (DeviceDefaults, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DeviceDefaults{}, err
	}
	var cfg NetboxInventoryConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DeviceDefaults{}, err
	}
	defaults := DeviceDefaults{}
	if v, ok := cfg.Credentials["username"].(string); ok {
		defaults.Username = v
	}
	if v, ok := cfg.Credentials["password"].(string); ok {
		defaults.Password = v
	}
	if v, ok := cfg.Credentials["enable_password"].(string); ok {
		defaults.EnablePassword = v
	}
	if v, ok := cfg.Credentials["timeout"].(string); ok {
		if dur, err := time.ParseDuration(v); err == nil {
			defaults.Timeout = dur
		}
	}
	if v, ok := cfg.Credentials["insecure"].(bool); ok {
		defaults.Insecure = v
	}
	if v, ok := cfg.Credentials["port"].(int); ok {
		defaults.Port = v
	}
	return defaults, nil
}

func init() {
	RegisterSource("netbox", func(node *yaml.Node) (Source, error) {
		// Two flavors: new format (top-level url/token/query) and legacy
		// (top-level `netbox:` block wrapping the same fields).
		var newFormat struct {
			Kind     string      `yaml:"kind"`
			URL      string      `yaml:"url"`
			Token    string      `yaml:"token"`
			Insecure bool        `yaml:"insecure"`
			Query    NetboxQuery `yaml:"query"`
		}
		if err := node.Decode(&newFormat); err == nil && newFormat.URL != "" {
			return &NetboxSource{
				config: NetboxConfig{URL: newFormat.URL, Token: newFormat.Token, Insecure: newFormat.Insecure},
				query:  newFormat.Query,
			}, nil
		}

		var legacy struct {
			Netbox struct {
				URL      string `yaml:"url"`
				Token    string `yaml:"token"`
				Insecure bool   `yaml:"insecure"`
			} `yaml:"netbox"`
			Query NetboxQuery `yaml:"query"`
		}
		if err := node.Decode(&legacy); err != nil {
			return nil, fmt.Errorf("netbox source: decode YAML: %w", err)
		}
		if legacy.Netbox.URL == "" {
			return nil, fmt.Errorf("netbox source: URL is required")
		}
		return &NetboxSource{
			config: NetboxConfig{URL: legacy.Netbox.URL, Token: legacy.Netbox.Token, Insecure: legacy.Netbox.Insecure},
			query:  legacy.Query,
		}, nil
	})
}
