package inventory

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fluidstackio/go-anta/internal/logger"
)

// NetboxConfig contains configuration for Netbox API connection
type NetboxConfig struct {
	URL      string        `yaml:"url" json:"url"`
	Token    string        `yaml:"token" json:"-"`
	Insecure bool          `yaml:"insecure,omitempty" json:"insecure,omitempty"`
	Timeout  time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// String returns a redacted NetboxConfig so the API token cannot leak
// via fmt.Sprintf("%v", cfg) or logger calls.
func (c NetboxConfig) String() string {
	token := ""
	if c.Token != "" {
		token = "[REDACTED]"
	}
	return fmt.Sprintf("NetboxConfig{URL:%s Token:%s Insecure:%t Timeout:%s}",
		c.URL, token, c.Insecure, c.Timeout)
}

func (c NetboxConfig) GoString() string { return c.String() }

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
			return nil, fmt.Errorf("netbox API returned status %d", resp.StatusCode)
		}
		
		var nbResp NetboxResponse
		if err := json.NewDecoder(resp.Body).Decode(&nbResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		
		allDevices = append(allDevices, nbResp.Results...)
		logger.Debugf("Fetched %d devices from current page, total so far: %d", len(nbResp.Results), len(allDevices))
		
		// Follow pagination. query.Limit controls Netbox's per-page size, not
		// a cap on total results, so we always follow Next until exhausted.
		if nbResp.Next != "" {
			apiURL = nbResp.Next
		} else {
			break
		}
	}
	
	logger.Infof("Successfully loaded %d devices from Netbox", len(allDevices))
	return allDevices, nil
}

