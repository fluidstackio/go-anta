package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/fluidstackio/go-anta/pkg/inventory"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	invFile           string
	invNetboxURL      string
	invNetboxToken    string
	invNetboxQuery    string
	invTags           string
	invDevices        string
	invLimit          string
	invFormat         string
	invShowTags       bool
	invShowExtra      bool
	invSource         string
	invRegion         string
	invFilter         string
	invDeviceUsername string
	invDevicePassword string
	invTransport      string
	invPlaintext      bool
)

var InventoryCmd = &cobra.Command{
	Use:   "inventory",
	Short: "View and validate inventory",
	Long:  `The inventory command displays devices that would be tested without actually connecting to them.`,
	RunE:  runInventory,
}

func init() {
	InventoryCmd.Flags().StringVarP(&invFile, "inventory", "i", "", "inventory file path")
	InventoryCmd.Flags().StringVar(&invNetboxURL, "netbox-url", "", "Netbox URL (can also use NETBOX_URL env var)")
	InventoryCmd.Flags().StringVar(&invNetboxToken, "netbox-token", "", "Netbox API token (can also use NETBOX_TOKEN env var)")
	InventoryCmd.Flags().StringVar(&invNetboxQuery, "netbox-query", "", "Netbox query filter (e.g., 'site=dc1,role=leaf')")
	InventoryCmd.Flags().StringVarP(&invTags, "tags", "t", "", "filter devices by tags (comma-separated)")
	InventoryCmd.Flags().StringVarP(&invDevices, "devices", "d", "", "filter specific devices (comma-separated)")
	InventoryCmd.Flags().StringVar(&invLimit, "limit", "", "limit devices: hostname, comma-separated list (host1,host2), index (0), range (0-2), or wildcard (leaf*)")
	InventoryCmd.Flags().StringVarP(&invFormat, "format", "f", "table", "output format (table, json, yaml, count)")
	InventoryCmd.Flags().BoolVar(&invShowTags, "show-tags", false, "show device tags")
	InventoryCmd.Flags().BoolVar(&invShowExtra, "show-extra", false, "show extra device metadata")
	InventoryCmd.Flags().StringVar(&invSource, "source", "", "override the YAML inventory kind (file, netbox, dcfab)")
	InventoryCmd.Flags().StringVar(&invRegion, "region", "", "dcfab region filter")
	InventoryCmd.Flags().StringVar(&invFilter, "filter", "", "dcfab GraphQL filter (e.g. 'roles: [\"fm0\"], platforms: [\"eos\"]'); overrides YAML filter")
	InventoryCmd.Flags().StringVar(&invDeviceUsername, "device-username", "", "device username (overrides DEVICE_USERNAME env var)")
	InventoryCmd.Flags().StringVar(&invDevicePassword, "device-password", "", "device password (overrides DEVICE_PASSWORD env var)")
	InventoryCmd.Flags().StringVar(&invTransport, "transport", "", "transport for device connections: eapi or gnmi. When set, overrides per-device YAML transport; otherwise the YAML value is used (or eapi if unset).")
	InventoryCmd.Flags().BoolVar(&invPlaintext, "plaintext", false, "use plaintext gRPC for gnmi transport (no TLS); ignored for eapi")
}

func runInventory(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	inv, err := LoadInventoryForRun(ctx, InventoryLoadOptions{
		Path:           invFile,
		SourceOverride: invSource,
		NetboxURL:      invNetboxURL,
		NetboxToken:    invNetboxToken,
		NetboxQuery:    invNetboxQuery,
		Region:         invRegion,
		Filter:         invFilter,
		Defaults: inventory.DeviceDefaults{
			Username:  invDeviceUsername,
			Password:  invDevicePassword,
			Transport: invTransport,
			Insecure:  true,
			Plaintext: invPlaintext,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to load inventory: %w", err)
	}

	// Apply filters
	if invTags != "" {
		tagList := strings.Split(invTags, ",")
		inv = inv.FilterByTags(tagList)
	}

	if invDevices != "" {
		deviceList := strings.Split(invDevices, ",")
		inv = inv.FilterByNames(deviceList)
	}

	if invLimit != "" {
		inv = inv.FilterByLimit(invLimit)
	}

	// Display inventory based on format
	switch invFormat {
	case "count":
		fmt.Printf("%d\n", len(inv.Devices))
		
	case "json":
		return outputJSON(inv)
		
	case "yaml":
		return outputYAML(inv)
		
	default: // table
		return outputTable(inv)
	}

	return nil
}

func outputTable(inv *inventory.Inventory) error {
	if len(inv.Devices) == 0 {
		fmt.Println("No devices found matching the criteria")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	if invShowTags && invShowExtra {
		fmt.Fprintln(w, "Name\tHost\tPort\tUsername\tTags\tExtra")
		fmt.Fprintln(w, "----\t----\t----\t--------\t----\t-----")
	} else if invShowTags {
		fmt.Fprintln(w, "Name\tHost\tPort\tUsername\tTags")
		fmt.Fprintln(w, "----\t----\t----\t--------\t----")
	} else if invShowExtra {
		fmt.Fprintln(w, "Name\tHost\tPort\tUsername\tExtra")
		fmt.Fprintln(w, "----\t----\t----\t--------\t-----")
	} else {
		fmt.Fprintln(w, "Name\tHost\tPort\tUsername")
		fmt.Fprintln(w, "----\t----\t----\t--------")
	}

	// Devices
	for _, dev := range inv.Devices {
		port := dev.Port
		if port == 0 {
			port = 443
		}

		if invShowTags && invShowExtra {
			tags := strings.Join(dev.Tags, ", ")
			extra := formatExtra(dev.Extra)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
				dev.Name, dev.Host, port, dev.Username, tags, extra)
		} else if invShowTags {
			tags := strings.Join(dev.Tags, ", ")
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				dev.Name, dev.Host, port, dev.Username, tags)
		} else if invShowExtra {
			extra := formatExtra(dev.Extra)
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
				dev.Name, dev.Host, port, dev.Username, extra)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n",
				dev.Name, dev.Host, port, dev.Username)
		}
	}

	// Summary
	fmt.Fprintf(w, "\nTotal devices: %d\n", len(inv.Devices))

	// Tag summary if requested
	if invShowTags {
		tagCount := make(map[string]int)
		for _, dev := range inv.Devices {
			for _, tag := range dev.Tags {
				tagCount[tag]++
			}
		}
		
		if len(tagCount) > 0 {
			fmt.Fprintln(w, "\nTag Summary:")
			for tag, count := range tagCount {
				fmt.Fprintf(w, "  %s: %d devices\n", tag, count)
			}
		}
	}

	return nil
}

func outputJSON(inv *inventory.Inventory) error {
	// Create a simplified output structure
	type deviceOutput struct {
		Name     string            `json:"name"`
		Host     string            `json:"host"`
		Port     int               `json:"port"`
		Username string            `json:"username"`
		Tags     []string          `json:"tags,omitempty"`
		Extra    map[string]string `json:"extra,omitempty"`
	}

	output := struct {
		Count   int            `json:"count"`
		Devices []deviceOutput `json:"devices"`
	}{
		Count:   len(inv.Devices),
		Devices: make([]deviceOutput, 0, len(inv.Devices)),
	}

	for _, dev := range inv.Devices {
		port := dev.Port
		if port == 0 {
			port = 443
		}

		d := deviceOutput{
			Name:     dev.Name,
			Host:     dev.Host,
			Port:     port,
			Username: dev.Username,
		}

		if invShowTags && len(dev.Tags) > 0 {
			d.Tags = dev.Tags
		}

		if invShowExtra && len(dev.Extra) > 0 {
			d.Extra = dev.Extra
		}

		output.Devices = append(output.Devices, d)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

func outputYAML(inv *inventory.Inventory) error {
	// Output in standard GANTA inventory format
	encoder := yaml.NewEncoder(os.Stdout)
	return encoder.Encode(inv)
}

func formatExtra(extra map[string]string) string {
	if len(extra) == 0 {
		return "-"
	}

	parts := make([]string, 0, len(extra))
	for key, value := range extra {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, ", ")
}

