package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/inventory"
	"github.com/spf13/cobra"
)

var (
	checkNetboxURL      string
	checkNetboxToken    string
	checkNetboxQuery    string
	checkNoConnect      bool
	checkLimit          string
	checkDeviceUsername string
	checkDevicePassword string
	checkTransport      string
	checkSource         string
	checkRegion         string
	checkRoles          string
	checkPlaintext      bool
)

var CheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check device connectivity and status",
	Long:  `The check command verifies that devices in the inventory are reachable and establishes connections to them.`,
	RunE:  runCheck,
}

func init() {
	CheckCmd.Flags().StringVarP(&inventoryFile, "inventory", "i", "", "inventory file path")
	CheckCmd.Flags().StringVar(&checkNetboxURL, "netbox-url", "", "Netbox URL (can also use NETBOX_URL env var)")
	CheckCmd.Flags().StringVar(&checkNetboxToken, "netbox-token", "", "Netbox API token (can also use NETBOX_TOKEN env var)")
	CheckCmd.Flags().StringVar(&checkNetboxQuery, "netbox-query", "", "Netbox query filter (e.g., 'site=dc1,role=leaf')")
	CheckCmd.Flags().StringVarP(&devices, "devices", "d", "", "specific devices to check (comma-separated)")
	CheckCmd.Flags().StringVarP(&tags, "tags", "t", "", "filter devices by tags (comma-separated)")
	CheckCmd.Flags().StringVar(&checkLimit, "limit", "", "limit devices: hostname, comma-separated list (host1,host2), index (0), range (0-2), or wildcard (leaf*)")
	CheckCmd.Flags().StringVar(&checkDeviceUsername, "device-username", "", "device username (overrides DEVICE_USERNAME env var)")
	CheckCmd.Flags().StringVar(&checkDevicePassword, "device-password", "", "device password (overrides DEVICE_PASSWORD env var)")
	CheckCmd.Flags().BoolVar(&checkNoConnect, "no-connect", false, "only show inventory without connecting to devices")
	CheckCmd.Flags().StringVar(&checkTransport, "transport", "", "transport for device connections: eapi or gnmi. When set, overrides per-device YAML transport; otherwise the YAML value is used (or eapi if unset).")
	CheckCmd.Flags().StringVar(&checkSource, "source", "", "override the YAML inventory kind (file, netbox, dcfab)")
	CheckCmd.Flags().StringVar(&checkRegion, "region", "", "dcfab region filter")
	CheckCmd.Flags().StringVar(&checkRoles, "roles", "", "dcfab roles filter (comma-separated)")
	CheckCmd.Flags().BoolVar(&checkPlaintext, "plaintext", false, "use plaintext gRPC for gnmi transport (no TLS); ignored for eapi")
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Validate the transport override early so a bad value produces a
	// clear error instead of silently failing every device-construct
	// call inside the connect loop.
	switch checkTransport {
	case "", "eapi", "gnmi":
	default:
		return fmt.Errorf("unknown --transport value %q (supported: eapi, gnmi)", checkTransport)
	}

	inv, err := LoadInventoryForRun(ctx, InventoryLoadOptions{
		Path:           inventoryFile,
		SourceOverride: checkSource,
		NetboxURL:      checkNetboxURL,
		NetboxToken:    checkNetboxToken,
		NetboxQuery:    checkNetboxQuery,
		Region:         checkRegion,
		Roles:          checkRoles,
		Defaults: inventory.DeviceDefaults{
			Username:  checkDeviceUsername,
			Password:  checkDevicePassword,
			Transport: checkTransport,
			Insecure:  true,
			Plaintext: checkPlaintext,
		},
	})
	if err != nil {
		return err
	}

	if tags != "" {
		tagList := strings.Split(tags, ",")
		inv = inv.FilterByTags(tagList)
	}

	if devices != "" {
		deviceList := strings.Split(devices, ",")
		inv = inv.FilterByNames(deviceList)
	}

	if checkLimit != "" {
		inv = inv.FilterByLimit(checkLimit)
	}

	fmt.Printf("Checking %d devices...\n\n", len(inv.Devices))

	// If no-connect flag is set, just display the inventory
	if checkNoConnect {
		fmt.Print("Inventory (without connection test):\n\n")
		for i, devConfig := range inv.Devices {
			fmt.Printf("%d. %s (%s:%d) - Tags: %s\n",
				i+1, devConfig.Name, devConfig.Host, devConfig.Port,
				strings.Join(devConfig.Tags, ", "))
		}
		return nil
	}

	successCount := 0
	failCount := 0

	for _, devConfig := range inv.Devices {
		if checkTransport != "" {
			devConfig.Transport = checkTransport
		}
		dev, err := device.New(devConfig)
		if err != nil {
			fmt.Printf("  %s: failed to construct device: %v\n", devConfig.Name, err)
			failCount++
			continue
		}

		fmt.Printf("Checking %s (%s)... ", devConfig.Name, devConfig.Host)

		if err := dev.Connect(ctx); err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			failCount++
			continue
		}

		if dev.IsEstablished() {
			model := dev.HardwareModel()
			if model != "" {
				fmt.Printf("✅ Connected (Model: %s)\n", model)
			} else {
				fmt.Printf("✅ Connected\n")
			}
			successCount++
		} else {
			fmt.Printf("⚠️  Connected but not established\n")
			failCount++
		}

		dev.Disconnect()
	}

	fmt.Printf("\nSummary: %d successful, %d failed\n", successCount, failCount)

	if failCount > 0 {
		return fmt.Errorf("%d devices failed connectivity check", failCount)
	}

	return nil
}


