package inventory

import (
	"context"
	"fmt"
	"net/url"
	"strings"

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
type DcfabSource struct {
	cfg      DcfabConfig
	defaults DeviceDefaults
}

func (s *DcfabSource) Kind() string { return "dcfab" }

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

// Load is implemented in Task 8; returning an explicit error makes
// accidental use during development obvious.
func (s *DcfabSource) Load(ctx context.Context) (*Inventory, error) {
	return nil, fmt.Errorf("DcfabSource.Load not yet implemented")
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
