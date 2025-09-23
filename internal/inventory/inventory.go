package inventory

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"gopkg.in/yaml.v3"
)

type Inventory struct {
	Devices  []device.DeviceConfig `yaml:"devices" json:"devices"`
	Networks []NetworkDefinition   `yaml:"networks,omitempty" json:"networks,omitempty"`
	Ranges   []RangeDefinition    `yaml:"ranges,omitempty" json:"ranges,omitempty"`
}

type NetworkDefinition struct {
	Network        string            `yaml:"network" json:"network"`
	Username       string            `yaml:"username" json:"username"`
	Password       string            `yaml:"password" json:"password"`
	EnablePassword string            `yaml:"enable_password,omitempty" json:"enable_password,omitempty"`
	Tags           []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Port           int               `yaml:"port,omitempty" json:"port,omitempty"`
	Insecure       bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

type RangeDefinition struct {
	Start          string            `yaml:"start" json:"start"`
	End            string            `yaml:"end" json:"end"`
	Username       string            `yaml:"username" json:"username"`
	Password       string            `yaml:"password" json:"password"`
	EnablePassword string            `yaml:"enable_password,omitempty" json:"enable_password,omitempty"`
	Tags           []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
	Port           int               `yaml:"port,omitempty" json:"port,omitempty"`
	Insecure       bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

func LoadInventory(path string) (*Inventory, error) {
	// Check if it's a Netbox inventory file
	if isNetboxInventory(path) {
		return LoadNetboxInventory(path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open inventory file: %w", err)
	}
	defer file.Close()

	var inv Inventory
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&inv); err != nil {
		return nil, fmt.Errorf("failed to parse inventory: %w", err)
	}

	if err := inv.expandNetworks(); err != nil {
		return nil, fmt.Errorf("failed to expand networks: %w", err)
	}

	if err := inv.expandRanges(); err != nil {
		return nil, fmt.Errorf("failed to expand ranges: %w", err)
	}

	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("inventory validation failed: %w", err)
	}

	return &inv, nil
}

func isNetboxInventory(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	var check map[string]interface{}
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&check); err != nil {
		return false
	}

	_, hasNetbox := check["netbox"]
	return hasNetbox
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

		for ip := startIP; !ip.Equal(endIP); inc(ip) {
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

func (i *Inventory) FilterByTags(tags []string) *Inventory {
	if len(tags) == 0 {
		return i
	}

	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[strings.TrimSpace(tag)] = true
	}

	filtered := &Inventory{
		Devices: make([]device.DeviceConfig, 0),
	}

	for _, dev := range i.Devices {
		for _, devTag := range dev.Tags {
			if tagSet[devTag] {
				filtered.Devices = append(filtered.Devices, dev)
				break
			}
		}
	}

	return filtered
}

func (i *Inventory) FilterByNames(names []string) *Inventory {
	if len(names) == 0 {
		return i
	}

	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[strings.TrimSpace(name)] = true
	}

	filtered := &Inventory{
		Devices: make([]device.DeviceConfig, 0),
	}

	for _, dev := range i.Devices {
		if nameSet[dev.Name] {
			filtered.Devices = append(filtered.Devices, dev)
		}
	}

	return filtered
}

// FilterByLimit filters devices using various limit patterns
func (i *Inventory) FilterByLimit(limitPattern string) *Inventory {
	if limitPattern == "" {
		return i
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
		return filtered
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
				return filtered
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
		return filtered

	default:
		// Check if it's a numeric index
		if idx, err := strconv.Atoi(limitPattern); err == nil {
			if idx >= 0 && idx < len(i.Devices) {
				filtered.Devices = append(filtered.Devices, i.Devices[idx])
			}
			return filtered
		}
		
		// Treat as exact hostname match
		for _, dev := range i.Devices {
			if dev.Name == limitPattern {
				filtered.Devices = append(filtered.Devices, dev)
				break
			}
		}
		return filtered
	}
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