package inventory

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/internal/logger"
	"gopkg.in/yaml.v3"
)

// NetboxConfig contains configuration for Netbox API connection
type NetboxConfig struct {
	URL      string `yaml:"url" json:"url"`
	Token    string `yaml:"token" json:"token"`
	Insecure bool   `yaml:"insecure,omitempty" json:"insecure,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// NetboxClient provides access to Netbox API
type NetboxClient struct {
	config NetboxConfig
	client *http.Client
}

// NewNetboxClient creates a new Netbox API client
func NewNetboxClient(config NetboxConfig) *NetboxClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: config.Insecure,
		},
	}

	return &NetboxClient{
		config: config,
		client: &http.Client{
			Transport: tr,
			Timeout:   config.Timeout,
		},
	}
}

// NetboxQuery defines parameters for querying devices from Netbox
type NetboxQuery struct {
	// Device filters
	Site           string   `yaml:"site,omitempty" json:"site,omitempty"`
	SiteID         int      `yaml:"site_id,omitempty" json:"site_id,omitempty"`
	Role           string   `yaml:"role,omitempty" json:"role,omitempty"`
	RoleID         int      `yaml:"role_id,omitempty" json:"role_id,omitempty"`
	DeviceType     string   `yaml:"device_type,omitempty" json:"device_type,omitempty"`
	DeviceTypeID   int      `yaml:"device_type_id,omitempty" json:"device_type_id,omitempty"`
	Manufacturer   string   `yaml:"manufacturer,omitempty" json:"manufacturer,omitempty"`
	ManufacturerID int      `yaml:"manufacturer_id,omitempty" json:"manufacturer_id,omitempty"`
	Platform       string   `yaml:"platform,omitempty" json:"platform,omitempty"`
	PlatformID     int      `yaml:"platform_id,omitempty" json:"platform_id,omitempty"`
	Status         string   `yaml:"status,omitempty" json:"status,omitempty"`
	Tags           []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Tenant         string   `yaml:"tenant,omitempty" json:"tenant,omitempty"`
	TenantID       int      `yaml:"tenant_id,omitempty" json:"tenant_id,omitempty"`
	Region         string   `yaml:"region,omitempty" json:"region,omitempty"`
	RegionID       int      `yaml:"region_id,omitempty" json:"region_id,omitempty"`
	Name           string   `yaml:"name,omitempty" json:"name,omitempty"`
	NameContains   string   `yaml:"name__ic,omitempty" json:"name__ic,omitempty"`
	
	// Custom fields
	CustomFields map[string]string `yaml:"custom_fields,omitempty" json:"custom_fields,omitempty"`
	
	// Query options
	Limit  int  `yaml:"limit,omitempty" json:"limit,omitempty"`
	Offset int  `yaml:"offset,omitempty" json:"offset,omitempty"`
	
	// Include inactive devices
	IncludeInactive bool `yaml:"include_inactive,omitempty" json:"include_inactive,omitempty"`
}

// NetboxDevice represents a device from Netbox API
type NetboxDevice struct {
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	DeviceType  NetboxDeviceType       `json:"device_type"`
	DeviceRole  NetboxDeviceRole       `json:"device_role"`
	Platform    *NetboxPlatform        `json:"platform"`
	Serial      string                 `json:"serial"`
	Site        NetboxSite             `json:"site"`
	Status      NetboxStatus           `json:"status"`
	PrimaryIP   *NetboxIPAddress       `json:"primary_ip"`
	PrimaryIP4  *NetboxIPAddress       `json:"primary_ip4"`
	PrimaryIP6  *NetboxIPAddress       `json:"primary_ip6"`
	Tags        []NetboxTag            `json:"tags"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type NetboxDeviceType struct {
	ID           int                  `json:"id"`
	Display      string               `json:"display"`
	Manufacturer NetboxManufacturer   `json:"manufacturer"`
	Model        string               `json:"model"`
	Slug         string               `json:"slug"`
}

type NetboxManufacturer struct {
	ID      int    `json:"id"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

type NetboxDeviceRole struct {
	ID      int    `json:"id"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

type NetboxPlatform struct {
	ID      int    `json:"id"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

type NetboxSite struct {
	ID      int    `json:"id"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

type NetboxStatus struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type NetboxIPAddress struct {
	ID      int    `json:"id"`
	Display string `json:"display"`
	Address string `json:"address"`
}

type NetboxTag struct {
	ID      int    `json:"id"`
	Display string `json:"display"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

type NetboxResponse struct {
	Count    int            `json:"count"`
	Next     string         `json:"next"`
	Previous string         `json:"previous"`
	Results  []NetboxDevice `json:"results"`
}

// GetDevices queries Netbox for devices matching the given criteria
func (c *NetboxClient) GetDevices(ctx context.Context, query NetboxQuery) ([]NetboxDevice, error) {
	logger.Infof("Querying Netbox API: %s", c.config.URL)
	params := url.Values{}
	
	// Apply filters - use IDs if provided, otherwise use slugs
	if query.SiteID > 0 {
		params.Set("site_id", fmt.Sprintf("%d", query.SiteID))
	} else if query.Site != "" {
		params.Set("site", query.Site)
	}
	
	if query.RoleID > 0 {
		params.Set("role_id", fmt.Sprintf("%d", query.RoleID))
	} else if query.Role != "" {
		params.Set("role", query.Role)
	}
	
	if query.DeviceTypeID > 0 {
		params.Set("device_type_id", fmt.Sprintf("%d", query.DeviceTypeID))
	} else if query.DeviceType != "" {
		params.Set("device_type", query.DeviceType)
	}
	
	if query.ManufacturerID > 0 {
		params.Set("manufacturer_id", fmt.Sprintf("%d", query.ManufacturerID))
	} else if query.Manufacturer != "" {
		params.Set("manufacturer", query.Manufacturer)
	}
	
	if query.PlatformID > 0 {
		params.Set("platform_id", fmt.Sprintf("%d", query.PlatformID))
	} else if query.Platform != "" {
		params.Set("platform", query.Platform)
	}
	
	if query.TenantID > 0 {
		params.Set("tenant_id", fmt.Sprintf("%d", query.TenantID))
	} else if query.Tenant != "" {
		params.Set("tenant", query.Tenant)
	}
	
	if query.RegionID > 0 {
		params.Set("region_id", fmt.Sprintf("%d", query.RegionID))
	} else if query.Region != "" {
		params.Set("region", query.Region)
	}
	
	if query.Status != "" {
		params.Set("status", query.Status)
	} else if !query.IncludeInactive {
		params.Set("status", "active")
	}
	
	if query.Name != "" {
		params.Set("name", query.Name)
	}
	if query.NameContains != "" {
		params.Set("name__ic", query.NameContains)
	}
	
	// Add tags
	for _, tag := range query.Tags {
		params.Add("tag", tag)
	}
	
	// Add custom fields
	for key, value := range query.CustomFields {
		params.Set(fmt.Sprintf("cf_%s", key), value)
	}
	
	// Set pagination
	if query.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", query.Limit))
	} else {
		params.Set("limit", "1000")
	}
	if query.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", query.Offset))
	}
	
	// Build URL
	apiURL := fmt.Sprintf("%s/api/dcim/devices/?%s", strings.TrimRight(c.config.URL, "/"), params.Encode())
	logger.Debugf("Netbox API URL: %s", apiURL)
	
	var allDevices []NetboxDevice
	
	for apiURL != "" {
		logger.Debugf("Fetching Netbox page: %s", apiURL)
		req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		
		req.Header.Set("Authorization", fmt.Sprintf("Token %s", c.config.Token))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := c.client.Do(req)
		if err != nil {
			logger.Errorf("Failed to query Netbox: %v", err)
			return nil, fmt.Errorf("failed to query Netbox: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			logger.Errorf("Netbox API returned status %d", resp.StatusCode)
			return nil, fmt.Errorf("Netbox API returned status %d", resp.StatusCode)
		}
		
		var nbResp NetboxResponse
		if err := json.NewDecoder(resp.Body).Decode(&nbResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		
		allDevices = append(allDevices, nbResp.Results...)
		logger.Debugf("Fetched %d devices from current page, total so far: %d", len(nbResp.Results), len(allDevices))
		
		// Check for next page
		if nbResp.Next != "" && query.Limit == 0 {
			apiURL = nbResp.Next
		} else {
			break
		}
	}
	
	logger.Infof("Successfully loaded %d devices from Netbox", len(allDevices))
	return allDevices, nil
}

// LoadFromNetbox loads inventory from Netbox using the specified query
func LoadFromNetbox(config NetboxConfig, query NetboxQuery, credentials map[string]interface{}) (*Inventory, error) {
	ctx := context.Background()
	client := NewNetboxClient(config)
	
	logger.Infof("Loading inventory from Netbox")
	devices, err := client.GetDevices(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to load devices from Netbox: %w", err)
	}
	
	inv := &Inventory{
		Devices: make([]device.DeviceConfig, 0, len(devices)),
	}
	
	// Default credentials - set admin as default username if not provided
	defaultUsername := "admin"
	defaultPassword := ""
	defaultEnablePassword := ""
	defaultPort := 443
	defaultInsecure := true
	defaultTimeout := 30 * time.Second
	
	if username, ok := credentials["username"].(string); ok && username != "" {
		defaultUsername = username
	}
	if password, ok := credentials["password"].(string); ok {
		defaultPassword = password
	}
	if enablePassword, ok := credentials["enable_password"].(string); ok {
		defaultEnablePassword = enablePassword
	}
	if port, ok := credentials["port"].(int); ok {
		defaultPort = port
	}
	if insecure, ok := credentials["insecure"].(bool); ok {
		defaultInsecure = insecure
	}
	if timeout, ok := credentials["timeout"].(string); ok {
		if dur, err := time.ParseDuration(timeout); err == nil {
			defaultTimeout = dur
		}
	}
	
	for _, nbDevice := range devices {
		// Skip devices without primary IP
		if nbDevice.PrimaryIP == nil && nbDevice.PrimaryIP4 == nil {
			logger.Debugf("Skipping device %s: no primary IP address", nbDevice.Name)
			continue
		}
		
		// Get IP address
		ipAddr := ""
		if nbDevice.PrimaryIP != nil {
			ipAddr = strings.Split(nbDevice.PrimaryIP.Address, "/")[0]
		} else if nbDevice.PrimaryIP4 != nil {
			ipAddr = strings.Split(nbDevice.PrimaryIP4.Address, "/")[0]
		}
		
		if ipAddr == "" {
			continue
		}
		
		// Build tags from Netbox tags and metadata
		tags := make([]string, 0)
		for _, tag := range nbDevice.Tags {
			tags = append(tags, tag.Name)
		}
		
		// Add metadata as tags
		if nbDevice.Site.Name != "" {
			tags = append(tags, fmt.Sprintf("site:%s", nbDevice.Site.Slug))
		}
		if nbDevice.DeviceRole.Name != "" {
			tags = append(tags, fmt.Sprintf("role:%s", nbDevice.DeviceRole.Slug))
		}
		if nbDevice.Platform != nil && nbDevice.Platform.Name != "" {
			tags = append(tags, fmt.Sprintf("platform:%s", nbDevice.Platform.Slug))
		}
		if nbDevice.DeviceType.Manufacturer.Name != "" {
			tags = append(tags, fmt.Sprintf("vendor:%s", nbDevice.DeviceType.Manufacturer.Slug))
		}
		
		// Override credentials from custom fields if available
		deviceUsername := defaultUsername
		devicePassword := defaultPassword
		deviceEnablePassword := defaultEnablePassword
		
		if username, ok := nbDevice.CustomFields["username"].(string); ok && username != "" {
			deviceUsername = username
		}
		if password, ok := nbDevice.CustomFields["password"].(string); ok && password != "" {
			devicePassword = password
		}
		if enablePassword, ok := nbDevice.CustomFields["enable_password"].(string); ok && enablePassword != "" {
			deviceEnablePassword = enablePassword
		}
		
		// Create device config
		devConfig := device.DeviceConfig{
			Name:           nbDevice.Name,
			Host:           ipAddr,
			Port:           defaultPort,
			Username:       deviceUsername,
			Password:       devicePassword,
			EnablePassword: deviceEnablePassword,
			Tags:           tags,
			Timeout:        defaultTimeout,
			Insecure:       defaultInsecure,
			Extra: map[string]string{
				"netbox_id":     fmt.Sprintf("%d", nbDevice.ID),
				"site":          nbDevice.Site.Name,
				"role":          nbDevice.DeviceRole.Name,
				"device_type":   nbDevice.DeviceType.Model,
				"manufacturer": nbDevice.DeviceType.Manufacturer.Name,
			},
		}
		
		// Add platform info if available
		if nbDevice.Platform != nil {
			devConfig.Extra["platform"] = nbDevice.Platform.Name
		}
		
		// Add serial number if available
		if nbDevice.Serial != "" {
			devConfig.Extra["serial"] = nbDevice.Serial
		}
		
		logger.Debugf("Added device %s (%s) to inventory", nbDevice.Name, ipAddr)
		inv.Devices = append(inv.Devices, devConfig)
	}
	
	if len(inv.Devices) == 0 {
		logger.Warnf("No devices with primary IP found in Netbox query results")
		return nil, fmt.Errorf("no devices with primary IP found in Netbox query results")
	}
	
	logger.Infof("Created inventory with %d devices from Netbox", len(inv.Devices))
	return inv, nil
}

// NetboxInventoryConfig defines the structure for Netbox inventory configuration
type NetboxInventoryConfig struct {
	Netbox      NetboxConfig           `yaml:"netbox" json:"netbox"`
	Query       NetboxQuery            `yaml:"query" json:"query"`
	Credentials map[string]interface{} `yaml:"credentials,omitempty" json:"credentials,omitempty"`
}

// LoadNetboxInventory loads inventory from a Netbox configuration file
func LoadNetboxInventory(path string) (*Inventory, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open Netbox inventory file: %w", err)
	}
	defer file.Close()
	
	var config NetboxInventoryConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse Netbox inventory: %w", err)
	}
	
	// Check for environment variable overrides
	if envURL := os.Getenv("NETBOX_URL"); envURL != "" {
		config.Netbox.URL = envURL
	}
	if envToken := os.Getenv("NETBOX_TOKEN"); envToken != "" {
		config.Netbox.Token = envToken
	}
	
	// Validate configuration
	if config.Netbox.URL == "" {
		return nil, fmt.Errorf("Netbox URL is required")
	}
	if config.Netbox.Token == "" {
		return nil, fmt.Errorf("Netbox API token is required")
	}
	
	return LoadFromNetbox(config.Netbox, config.Query, config.Credentials)
}