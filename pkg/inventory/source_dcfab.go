package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fluidstackio/go-anta/internal/logger"
	"github.com/fluidstackio/go-anta/pkg/device"
	"gopkg.in/yaml.v3"
)

// DcfabConfig is the YAML schema for `kind: dcfab` inventory files.
type DcfabConfig struct {
	Env       string   `yaml:"env"`       // prod | dev (default prod)
	Region    string   `yaml:"region"`    // required
	Roles     []string `yaml:"roles"`     // optional filter
	Platforms []string `yaml:"platforms"` // optional filter
	Endpoint  string   `yaml:"endpoint"`  // optional override
	PreferIP  string   `yaml:"prefer_ip"` // ipv4 | ipv6 (default ipv6)
}

// DcfabSource implements Source by querying the dcfab GraphQL API.
// Credentials are not stored on the source — the caller applies them
// via Inventory.ApplyDefaults after Load.
type DcfabSource struct {
	cfg DcfabConfig
}

func (s *DcfabSource) Kind() string { return "dcfab" }

// SetRegion overrides the region after the source was constructed.
// Used by the CLI --region flag.
func (s *DcfabSource) SetRegion(region string) { s.cfg.Region = region }

// SetRoles overrides the roles filter after the source was constructed.
// Used by the CLI --roles flag.
func (s *DcfabSource) SetRoles(roles []string) { s.cfg.Roles = roles }

// dcfabEndpoint resolves the HTTPS endpoint from the config. Explicit
// Endpoint wins; otherwise Env selects between prod/dev defaults.
func dcfabEndpoint(cfg DcfabConfig) string {
	if cfg.Endpoint != "" {
		return cfg.Endpoint
	}
	switch cfg.Env {
	case "dev":
		return "https://dcfab.fluidstack.xyz"
	default:
		return "https://dcfab.fluidstack.io"
	}
}

// queryURL builds the full GET URL for the dcfab GraphQL endpoint with
// the configured filters baked into the query string.
func (s *DcfabSource) queryURL(endpoint string) string {
	q := s.buildQuery()
	v := url.Values{}
	v.Set("query", q)
	return strings.TrimRight(endpoint, "/") + "/v1alpha1/query?" + v.Encode()
}

// buildQuery assembles the GraphQL query. Returns ActiveDevice fields
// for inventory mapping: name, role, platform, and the management
// interface addresses.
func (s *DcfabSource) buildQuery() string {
	args := []string{`implementation: ACTIVE`, `limit: 5000`}
	if len(s.cfg.Roles) > 0 {
		args = append(args, fmt.Sprintf(`roles: %s`, stringSliceLiteral(s.cfg.Roles)))
	}
	if len(s.cfg.Platforms) > 0 {
		args = append(args, fmt.Sprintf(`platforms: %s`, stringSliceLiteral(s.cfg.Platforms)))
	}
	return fmt.Sprintf(`{
  region(name: %q) {
    devices(%s) {
      ... on ActiveDevice {
        name
        role
        platform
        managementInterface {
          addresses {
            address
            version
          }
        }
      }
    }
  }
}`, s.cfg.Region, strings.Join(args, ", "))
}

// stringSliceLiteral formats ["a","b"] for embedding in a GraphQL
// argument list.
func stringSliceLiteral(in []string) string {
	parts := make([]string, len(in))
	for i, s := range in {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// dcfabResponse is the JSON shape returned by the dcfab GraphQL endpoint
// for our region/devices query.
type dcfabResponse struct {
	Data struct {
		Region *struct {
			Devices []dcfabDevice `json:"devices"`
		} `json:"region"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors,omitempty"`
}

type dcfabDevice struct {
	Name                string              `json:"name"`
	Role                string              `json:"role"`
	Platform            string              `json:"platform"`
	ManagementInterface *dcfabMgmtInterface `json:"managementInterface"`
}

type dcfabMgmtInterface struct {
	Addresses []dcfabAddress `json:"addresses"`
}

type dcfabAddress struct {
	Address string `json:"address"`
	Version int    `json:"version"`
}

// dcfabPaginationCap is the GraphQL complexity-limit-derived ceiling
// from the dcfab skill docs. Hitting exactly this number signals
// possible truncation.
const dcfabPaginationCap = 5000

// dcfabHTTPTimeout caps a single dcfab query. The ctx deadline still
// dominates if shorter; this just protects against hung connections
// when the caller passed an unbounded context.
const dcfabHTTPTimeout = 30 * time.Second

func (s *DcfabSource) Load(ctx context.Context) (*Inventory, error) {
	endpoint := dcfabEndpoint(s.cfg)
	u := s.queryURL(endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("dcfab: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: dcfabHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dcfab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("dcfab: status %d: %s", resp.StatusCode, body)
	}

	var parsed dcfabResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("dcfab: decode response: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("dcfab: GraphQL error: %s", parsed.Errors[0].Message)
	}
	if parsed.Data.Region == nil {
		return nil, fmt.Errorf("dcfab: region %q not found", s.cfg.Region)
	}

	devices := parsed.Data.Region.Devices
	if len(devices) == dcfabPaginationCap {
		return nil, fmt.Errorf("dcfab: response hit %d-device pagination cap; narrow your roles/platforms filter or implement pagination", dcfabPaginationCap)
	}

	inv := &Inventory{}
	for _, d := range devices {
		host := pickMgmtAddress(d.ManagementInterface, s.cfg.PreferIP)
		if host == "" {
			logger.Warnf("dcfab: skipping device %s — no management address", d.Name)
			continue
		}
		inv.Devices = append(inv.Devices, device.DeviceConfig{
			Name: d.Name,
			Host: host,
			Tags: buildDcfabTags(d, s.cfg.Region),
		})
	}
	return inv, nil
}

// pickMgmtAddress returns the management address per the prefer_ip
// policy. Falls back to the other family if the preferred isn't
// present. CIDR suffix (/NN) is stripped.
func pickMgmtAddress(mi *dcfabMgmtInterface, prefer string) string {
	if mi == nil {
		return ""
	}
	if prefer == "" {
		prefer = "ipv6"
	}
	var preferred, fallback string
	for _, a := range mi.Addresses {
		stripped := stripCIDR(a.Address)
		switch a.Version {
		case 6:
			if prefer == "ipv6" && preferred == "" {
				preferred = stripped
			} else if fallback == "" {
				fallback = stripped
			}
		case 4:
			if prefer == "ipv4" && preferred == "" {
				preferred = stripped
			} else if fallback == "" {
				fallback = stripped
			}
		}
	}
	if preferred != "" {
		return preferred
	}
	return fallback
}

func buildDcfabTags(d dcfabDevice, region string) []string {
	tags := []string{}
	if d.Role != "" {
		tags = append(tags, "role:"+d.Role)
	}
	if d.Platform != "" {
		tags = append(tags, "platform:"+d.Platform)
	}
	if region != "" {
		tags = append(tags, "region:"+region)
	}
	return tags
}

func init() {
	RegisterSource("dcfab", func(node *yaml.Node) (Source, error) {
		var cfg struct {
			Kind        string `yaml:"kind"`
			DcfabConfig `yaml:",inline"`
		}
		if err := node.Decode(&cfg); err != nil {
			return nil, fmt.Errorf("dcfab source: decode YAML: %w", err)
		}
		if cfg.Region == "" {
			return nil, fmt.Errorf("dcfab source: region is required")
		}
		return &DcfabSource{cfg: cfg.DcfabConfig}, nil
	})
}
