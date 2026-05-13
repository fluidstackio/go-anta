package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fluidstackio/go-anta/internal/logger"
	"github.com/fluidstackio/go-anta/pkg/inventory"
)

// InventoryLoadOptions bundles every CLI flag/env value that affects how
// the inventory is loaded for a single command run.
type InventoryLoadOptions struct {
	// Source location.
	Path           string // --inventory; YAML file path
	SourceOverride string // --source; overrides YAML kind

	// Source-specific override knobs (any subset may be set).
	NetboxURL   string // --netbox-url; if set without Path, synthesize a netbox source
	NetboxToken string // --netbox-token
	NetboxQuery string // --netbox-query (raw key=value,key=value)
	Region      string // --region (dcfab override)
	Roles       string // --roles (dcfab override, comma-separated)

	// Credential / connection defaults applied after Load.
	Defaults inventory.DeviceDefaults
}

// LoadInventoryForRun resolves which inventory source to use, loads it,
// then overlays defaults and validates. This is the single entry point
// used by nrfu, check, and inventory commands so source-specific parsing
// lives in one place.
func LoadInventoryForRun(ctx context.Context, opts InventoryLoadOptions) (*inventory.Inventory, error) {
	// Validate --source override up front so a bad value fails fast.
	switch opts.SourceOverride {
	case "", "file", "netbox", "dcfab":
	default:
		return nil, fmt.Errorf("unknown --source value %q (supported: file, netbox, dcfab)", opts.SourceOverride)
	}

	// Apply env-var fallbacks to Defaults before we hand them to the source.
	if opts.Defaults.Username == "" {
		opts.Defaults.Username = os.Getenv("DEVICE_USERNAME")
	}
	if opts.Defaults.Password == "" {
		opts.Defaults.Password = os.Getenv("DEVICE_PASSWORD")
	}
	if opts.Defaults.EnablePassword == "" {
		opts.Defaults.EnablePassword = os.Getenv("DEVICE_ENABLE_PASSWORD")
	}

	switch {
	case opts.Path != "":
		var src inventory.Source
		var err error
		if opts.SourceOverride != "" {
			src, err = inventory.LoadSourceAs(opts.Path, opts.SourceOverride)
		} else {
			src, err = inventory.LoadSource(opts.Path)
		}
		if err != nil {
			return nil, err
		}
		// Apply CLI overrides to the source before loading. --region and
		// --roles only mean something for a dcfab source; warn if the user
		// supplied them with a different kind so they aren't silently
		// confused about why their filter had no effect.
		if d, ok := src.(*inventory.DcfabSource); ok {
			if opts.Region != "" {
				d.SetRegion(opts.Region)
			}
			if opts.Roles != "" {
				d.SetRoles(splitCSV(opts.Roles))
			}
		} else if opts.Region != "" || opts.Roles != "" {
			logger.Warnf("--region/--roles only apply to dcfab sources; ignored for %s source", src.Kind())
		}
		inv, err := src.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("inventory %s: %w", opts.Path, err)
		}
		inv = inv.ApplyDefaults(opts.Defaults)
		if err := inv.Validate(); err != nil {
			return nil, fmt.Errorf("inventory %s: %w", opts.Path, err)
		}
		return inv, nil

	case opts.NetboxURL != "" || os.Getenv("NETBOX_URL") != "":
		return loadNetboxFromFlags(ctx, opts)

	default:
		return nil, fmt.Errorf("either --inventory or --netbox-url must be specified")
	}
}

func loadNetboxFromFlags(ctx context.Context, opts InventoryLoadOptions) (*inventory.Inventory, error) {
	url := opts.NetboxURL
	if url == "" {
		url = os.Getenv("NETBOX_URL")
	}
	token := opts.NetboxToken
	if token == "" {
		token = os.Getenv("NETBOX_TOKEN")
	}
	query := parseNetboxQueryString(opts.NetboxQuery)

	src := inventory.NewNetboxSource(inventory.NetboxConfig{URL: url, Token: token}, query)
	inv, err := src.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("netbox: %w", err)
	}
	inv = inv.ApplyDefaults(opts.Defaults)
	if err := inv.Validate(); err != nil {
		return nil, fmt.Errorf("netbox: %w", err)
	}
	return inv, nil
}

// parseNetboxQueryString turns a "k1=v1,k2=v2" CLI string into a
// NetboxQuery. Unknown keys are silently ignored (matching the original
// CLI behavior); recognized keys are listed below — note that site_id,
// role_id, and device_type_id are now included (check.go was silently
// missing these in the original per-command parsers).
func parseNetboxQueryString(s string) inventory.NetboxQuery {
	q := inventory.NetboxQuery{}
	if s == "" {
		return q
	}
	for _, pair := range strings.Split(s, ",") {
		idx := strings.IndexByte(pair, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(pair[:idx])
		value := strings.TrimSpace(pair[idx+1:])
		switch key {
		case "site":
			q.Site = value
		case "role":
			q.Role = value
		case "device_type":
			q.DeviceType = value
		case "manufacturer":
			q.Manufacturer = value
		case "platform":
			q.Platform = value
		case "status":
			q.Status = value
		case "tenant":
			q.Tenant = value
		case "region":
			q.Region = value
		case "name":
			q.Name = value
		case "name_contains":
			q.NameContains = value
		case "tag":
			q.Tags = append(q.Tags, value)
		case "site_id":
			if n, err := strconv.Atoi(value); err == nil {
				q.SiteID = n
			}
		case "role_id":
			if n, err := strconv.Atoi(value); err == nil {
				q.RoleID = n
			}
		case "device_type_id":
			if n, err := strconv.Atoi(value); err == nil {
				q.DeviceTypeID = n
			}
		}
	}
	return q
}

// splitCSV splits a comma-separated string into trimmed, non-empty tokens.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
