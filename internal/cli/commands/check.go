package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

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
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	var inv *inventory.Inventory
	var err error

	// Load inventory from Netbox or file
	if checkNetboxURL != "" || os.Getenv("NETBOX_URL") != "" {
		inv, err = loadCheckNetboxInventory(ctx)
	} else if inventoryFile != "" {
		inv, err = inventory.LoadInventory(inventoryFile)
	} else {
		return fmt.Errorf("either --inventory or --netbox-url must be specified")
	}

	if err != nil {
		return fmt.Errorf("failed to load inventory: %w", err)
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
		dev := device.NewEOSDevice(devConfig)

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

func loadCheckNetboxInventory(ctx context.Context) (*inventory.Inventory, error) {
	// Get Netbox configuration
	url := checkNetboxURL
	if url == "" {
		url = os.Getenv("NETBOX_URL")
	}
	if url == "" {
		return nil, fmt.Errorf("Netbox URL is required (use --netbox-url or NETBOX_URL env var)")
	}

	token := checkNetboxToken
	if token == "" {
		token = os.Getenv("NETBOX_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("Netbox API token is required (use --netbox-token or NETBOX_TOKEN env var)")
	}

	// Parse query parameters
	query := inventory.NetboxQuery{
		IncludeInactive: false,
	}

	if checkNetboxQuery != "" {
		// Parse query string (format: key1=value1,key2=value2)
		pairs := strings.Split(checkNetboxQuery, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch key {
			case "site":
				query.Site = value
			case "role":
				query.Role = value
			case "device_type":
				query.DeviceType = value
			case "manufacturer":
				query.Manufacturer = value
			case "platform":
				query.Platform = value
			case "status":
				query.Status = value
			case "tenant":
				query.Tenant = value
			case "region":
				query.Region = value
			case "name":
				query.Name = value
			case "name_contains":
				query.NameContains = value
			case "tag":
				query.Tags = append(query.Tags, value)
			}
		}
	}

	// Get device credentials - CLI flags override env vars
	credentials := make(map[string]interface{})
	username := checkDeviceUsername
	if username == "" {
		username = os.Getenv("DEVICE_USERNAME")
	}
	if username == "" {
		username = "admin" // Default username
	}
	credentials["username"] = username

	password := checkDevicePassword
	if password == "" {
		password = os.Getenv("DEVICE_PASSWORD")
	}
	if password != "" {
		credentials["password"] = password
	}
	if enablePassword := os.Getenv("DEVICE_ENABLE_PASSWORD"); enablePassword != "" {
		credentials["enable_password"] = enablePassword
	}
	credentials["insecure"] = true

	config := inventory.NetboxConfig{
		URL:      url,
		Token:    token,
		Insecure: os.Getenv("NETBOX_INSECURE") == "true",
	}

	fmt.Fprintf(os.Stderr, "Loading devices from Netbox: %s\n", url)
	return inventory.LoadFromNetbox(config, query, credentials)
}

