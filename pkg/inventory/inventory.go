package inventory

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fluidstackio/go-anta/pkg/device"
)

// maxRangeSize caps the number of devices an IP range can expand to,
// preventing runaway memory growth from misconfigured or reversed ranges.
const maxRangeSize = 65536

// Inventory is the result type returned by every Source.Load. Devices
// is the canonical device list; Networks and Ranges are static-YAML
// constructs that get expanded into Devices during FileSource.Load.
type Inventory struct {
	Devices  []device.DeviceConfig `yaml:"devices" json:"devices"`
	Networks []NetworkDefinition   `yaml:"networks,omitempty" json:"networks,omitempty"`
	Ranges   []RangeDefinition     `yaml:"ranges,omitempty" json:"ranges,omitempty"`
}

// NetworkDefinition is a CIDR expanded into one DeviceConfig per host
// IP at static-YAML load time, with the listed credentials and tags
// applied to each generated entry.
type NetworkDefinition struct {
	Network        string   `yaml:"network" json:"network"`
	Username       string   `yaml:"username" json:"username"`
	Password       string   `yaml:"password" json:"-"`
	EnablePassword string   `yaml:"enable_password,omitempty" json:"-"`
	Tags           []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Port           int      `yaml:"port,omitempty" json:"port,omitempty"`
	Insecure       bool     `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

// String redacts Password/EnablePassword so '%v' / '%+v' logger calls
// can't leak credentials baked into network defaults.
func (n NetworkDefinition) String() string {
	return fmt.Sprintf("NetworkDefinition{Network:%s Username:%s Password:[REDACTED] EnablePassword:%s Tags:%v Port:%d Insecure:%t}",
		n.Network, n.Username, redactedIfSet(n.EnablePassword), n.Tags, n.Port, n.Insecure)
}

func (n NetworkDefinition) GoString() string { return n.String() }

// RangeDefinition is an inclusive Start..End IP range expanded into one
// DeviceConfig per address at static-YAML load time. Capped at
// maxRangeSize entries to prevent runaway expansion.
type RangeDefinition struct {
	Start          string   `yaml:"start" json:"start"`
	End            string   `yaml:"end" json:"end"`
	Username       string   `yaml:"username" json:"username"`
	Password       string   `yaml:"password" json:"-"`
	EnablePassword string   `yaml:"enable_password,omitempty" json:"-"`
	Tags           []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Port           int      `yaml:"port,omitempty" json:"port,omitempty"`
	Insecure       bool     `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

func (r RangeDefinition) String() string {
	return fmt.Sprintf("RangeDefinition{Start:%s End:%s Username:%s Password:[REDACTED] EnablePassword:%s Tags:%v Port:%d Insecure:%t}",
		r.Start, r.End, r.Username, redactedIfSet(r.EnablePassword), r.Tags, r.Port, r.Insecure)
}

func (r RangeDefinition) GoString() string { return r.String() }

func redactedIfSet(s string) string {
	if s == "" {
		return ""
	}
	return "[REDACTED]"
}

// LoadInventory is a back-compat wrapper around LoadSource. New callers
// should use LoadSource directly so they can pass a context to Load and
// supply DeviceDefaults via the CLI helper. This wrapper preserves the
// pre-abstraction behavior of returning a validated inventory.
func LoadInventory(path string) (*Inventory, error) {
	src, err := LoadSource(path)
	if err != nil {
		return nil, err
	}
	inv, err := src.Load(context.Background())
	if err != nil {
		return nil, err
	}
	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("validate inventory: %w", err)
	}
	return inv, nil
}

func (i *Inventory) expandNetworks() error {
	for _, netDef := range i.Networks {
		_, ipnet, err := net.ParseCIDR(netDef.Network)
		if err != nil {
			return fmt.Errorf("invalid network %s: %w", netDef.Network, err)
		}

		for ip := ipnet.IP.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
			if isHostIP(ip, ipnet) {
				dev := device.DeviceConfig{
					Name:           ip.String(),
					Host:           ip.String(),
					Port:           netDef.Port,
					Username:       netDef.Username,
					Password:       netDef.Password,
					EnablePassword: netDef.EnablePassword,
					Tags:           netDef.Tags,
					Insecure:       netDef.Insecure,
				}
				i.Devices = append(i.Devices, dev)
			}
		}
	}
	return nil
}

func (i *Inventory) expandRanges() error {
	for _, rangeDef := range i.Ranges {
		startIP := net.ParseIP(rangeDef.Start)
		endIP := net.ParseIP(rangeDef.End)

		if startIP == nil {
			return fmt.Errorf("invalid start IP: %s", rangeDef.Start)
		}
		if endIP == nil {
			return fmt.Errorf("invalid end IP: %s", rangeDef.End)
		}

		// Normalize to 16-byte form so IPv4 and IPv6 compare/iterate consistently.
		start16 := startIP.To16()
		end16 := endIP.To16()
		if start16 == nil || end16 == nil {
			return fmt.Errorf("could not normalize range %s-%s", rangeDef.Start, rangeDef.End)
		}

		// Reject cross-family ranges (one IPv4, one IPv6) — iterating across them
		// would never terminate via Equal().
		if (startIP.To4() == nil) != (endIP.To4() == nil) {
			return fmt.Errorf("range %s-%s mixes IPv4 and IPv6", rangeDef.Start, rangeDef.End)
		}

		// Reject reversed ranges; otherwise inc() would walk forward forever.
		if bytes.Compare(start16, end16) > 0 {
			return fmt.Errorf("range start %s is greater than end %s", rangeDef.Start, rangeDef.End)
		}

		ip := make(net.IP, len(start16))
		copy(ip, start16)
		count := 0
		for ; !ip.Equal(end16); inc(ip) {
			if count >= maxRangeSize {
				return fmt.Errorf("range %s-%s exceeds maximum size %d", rangeDef.Start, rangeDef.End, maxRangeSize)
			}
			dev := device.DeviceConfig{
				Name:           ip.String(),
				Host:           ip.String(),
				Port:           rangeDef.Port,
				Username:       rangeDef.Username,
				Password:       rangeDef.Password,
				EnablePassword: rangeDef.EnablePassword,
				Tags:           rangeDef.Tags,
				Insecure:       rangeDef.Insecure,
			}
			i.Devices = append(i.Devices, dev)
			count++
		}

		dev := device.DeviceConfig{
			Name:           endIP.String(),
			Host:           endIP.String(),
			Port:           rangeDef.Port,
			Username:       rangeDef.Username,
			Password:       rangeDef.Password,
			EnablePassword: rangeDef.EnablePassword,
			Tags:           rangeDef.Tags,
			Insecure:       rangeDef.Insecure,
		}
		i.Devices = append(i.Devices, dev)
	}
	return nil
}

func (i *Inventory) Validate() error {
	if len(i.Devices) == 0 {
		return fmt.Errorf("inventory must contain at least one device")
	}

	deviceNames := make(map[string]bool)
	for idx, dev := range i.Devices {
		if dev.Name == "" {
			return fmt.Errorf("device at index %d has no name", idx)
		}
		if dev.Host == "" {
			return fmt.Errorf("device '%s' has no host specified", dev.Name)
		}
		if dev.Username == "" {
			return fmt.Errorf("device '%s' has no username specified", dev.Name)
		}
		if deviceNames[dev.Name] {
			return fmt.Errorf("duplicate device name: %s", dev.Name)
		}
		deviceNames[dev.Name] = true
	}

	return nil
}

// FilterByTags keeps devices that carry at least one of `tags`. The
// returned error names any requested tags that matched zero devices.
// Best-effort callers can ignore the error; the filtered inventory is
// always returned.
func (i *Inventory) FilterByTags(tags []string) (*Inventory, error) {
	if len(tags) == 0 {
		return i, nil
	}

	wanted := make(map[string]bool, len(tags))
	for _, tag := range tags {
		wanted[strings.TrimSpace(tag)] = false
	}

	filtered := &Inventory{Devices: make([]device.DeviceConfig, 0)}
	for _, dev := range i.Devices {
		appended := false
		for _, devTag := range dev.Tags {
			if _, ok := wanted[devTag]; ok {
				wanted[devTag] = true
				if !appended {
					filtered.Devices = append(filtered.Devices, dev)
					appended = true
				}
			}
		}
	}
	return filtered, missingFilterErr("tag(s)", wanted)
}

// FilterByNames keeps devices whose name is in `names`. The returned
// error names any requested names with no matching device.
func (i *Inventory) FilterByNames(names []string) (*Inventory, error) {
	if len(names) == 0 {
		return i, nil
	}

	wanted := make(map[string]bool, len(names))
	for _, name := range names {
		wanted[strings.TrimSpace(name)] = false
	}

	filtered := &Inventory{Devices: make([]device.DeviceConfig, 0)}
	for _, dev := range i.Devices {
		if _, ok := wanted[dev.Name]; ok {
			wanted[dev.Name] = true
			filtered.Devices = append(filtered.Devices, dev)
		}
	}
	return filtered, missingFilterErr("device name(s)", wanted)
}

// FilterByLimit filters devices using various limit patterns. The
// returned error is non-nil when the pattern matched no devices —
// typos like `--limit nosuchhost` no longer silently produce an empty
// run that looks like success.
func (i *Inventory) FilterByLimit(limitPattern string) (*Inventory, error) {
	if limitPattern == "" {
		return i, nil
	}

	filtered := &Inventory{
		Devices: make([]device.DeviceConfig, 0),
	}

	// Check if it contains commas - treat as comma-separated list of hostnames
	if strings.Contains(limitPattern, ",") {
		hostnames := strings.Split(limitPattern, ",")
		hostnameSet := make(map[string]bool)
		for _, hostname := range hostnames {
			hostnameSet[strings.TrimSpace(hostname)] = true
		}

		for _, dev := range i.Devices {
			if hostnameSet[dev.Name] {
				filtered.Devices = append(filtered.Devices, dev)
			}
		}
		return filtered, limitMissErr(filtered, limitPattern)
	}

	// Parse different limit patterns
	switch {
	case strings.Contains(limitPattern, "-") && !strings.Contains(limitPattern, "*") && !strings.Contains(limitPattern, "?"):
		// Range pattern: "0-2", "1-5" (but not if it contains wildcards like "host-*")
		parts := strings.Split(limitPattern, "-")
		if len(parts) == 2 {
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil && start >= 0 && end >= start && end < len(i.Devices) {
				for idx := start; idx <= end; idx++ {
					filtered.Devices = append(filtered.Devices, i.Devices[idx])
				}
				return filtered, limitMissErr(filtered, limitPattern)
			}
		}
		// If range parsing failed, treat as hostname
		fallthrough

	case strings.Contains(limitPattern, "*") || strings.Contains(limitPattern, "?"):
		// Wildcard pattern: "leaf*", "spine?", "*border*"
		for _, dev := range i.Devices {
			if matched, _ := filepath.Match(limitPattern, dev.Name); matched {
				filtered.Devices = append(filtered.Devices, dev)
			}
		}
		return filtered, limitMissErr(filtered, limitPattern)

	default:
		// Check if it's a numeric index
		if idx, err := strconv.Atoi(limitPattern); err == nil {
			if idx >= 0 && idx < len(i.Devices) {
				filtered.Devices = append(filtered.Devices, i.Devices[idx])
			}
			return filtered, limitMissErr(filtered, limitPattern)
		}

		// Treat as exact hostname match
		for _, dev := range i.Devices {
			if dev.Name == limitPattern {
				filtered.Devices = append(filtered.Devices, dev)
				break
			}
		}
		return filtered, limitMissErr(filtered, limitPattern)
	}
}

// missingFilterErr / limitMissErr are local helpers — sibling to the
// catalog package's `missingErr` but kept here to avoid an awkward
// pkg/test ↔ pkg/inventory dependency. Both return nil on a non-empty
// result and a descriptive error otherwise.
func missingFilterErr(label string, seen map[string]bool) error {
	var missing []string
	for k, found := range seen {
		if !found {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("no matches for %s: %v", label, missing)
}

func limitMissErr(filtered *Inventory, pattern string) error {
	if len(filtered.Devices) > 0 {
		return nil
	}
	return fmt.Errorf("--limit %q matched no devices", pattern)
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func isHostIP(ip net.IP, ipnet *net.IPNet) bool {
	ones, bits := ipnet.Mask.Size()
	if ones == bits {
		return true
	}
	if ones == bits-1 {
		return true
	}

	broadcast := make(net.IP, len(ip))
	copy(broadcast, ip)
	for i := range broadcast {
		broadcast[i] = ipnet.IP[i] | ^ipnet.Mask[i]
	}

	return !ip.Equal(ipnet.IP) && !ip.Equal(broadcast)
}

// DeviceDefaults holds connection settings that the CLI / env supplies
// at run time. Sources that fetch from APIs (Netbox, dcfab) leave these
// empty on the devices they return; the caller overlays them via
// Inventory.ApplyDefaults.
type DeviceDefaults struct {
	Username       string
	Password       string
	EnablePassword string
	Timeout        time.Duration
	Transport      string
	Insecure       bool
	Plaintext      bool
	Port           int
}

// ApplyDefaults returns a new Inventory in which each device has any
// empty connection-config fields filled in from d. Per-device YAML
// values always win; defaults only fill blanks. The receiver is not
// mutated.
func (i *Inventory) ApplyDefaults(d DeviceDefaults) *Inventory {
	out := &Inventory{
		Networks: i.Networks,
		Ranges:   i.Ranges,
		Devices:  make([]device.DeviceConfig, len(i.Devices)),
	}
	for idx, dev := range i.Devices {
		if dev.Username == "" {
			dev.Username = d.Username
		}
		if dev.Password == "" {
			dev.Password = d.Password
		}
		if dev.EnablePassword == "" {
			dev.EnablePassword = d.EnablePassword
		}
		if dev.Timeout == 0 {
			dev.Timeout = d.Timeout
		}
		if dev.Transport == "" {
			dev.Transport = d.Transport
		}
		if dev.Port == 0 {
			dev.Port = d.Port
		}
		// Booleans: only flip false→true; never the reverse.
		if !dev.Insecure && d.Insecure {
			dev.Insecure = true
		}
		if !dev.Plaintext && d.Plaintext {
			dev.Plaintext = true
		}
		out.Devices[idx] = dev
	}
	return out
}
