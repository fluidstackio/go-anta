package commands

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/inventory"
	"github.com/gavmckee/go-anta/internal/logger"
	"github.com/gavmckee/go-anta/internal/reporter"
	"github.com/gavmckee/go-anta/internal/test"
	"github.com/spf13/cobra"
)

var (
	inventoryFile     string
	catalogFile       string
	netboxURL         string
	netboxToken       string
	netboxQuery       string
	tags              string
	devices           string
	tests             string
	limit             string
	deviceUsername    string
	devicePassword    string
	concurrency       int
	dryRun            bool
	ignoreStatus      bool
	hide              string
	outputFile        string
	format            string
	logLevel          string
	verbose           bool
	quiet             bool
	progress          bool
	silent            bool
)

var NrfuCmd = &cobra.Command{
	Use:   "nrfu",
	Short: "Network Ready For Use - Run network tests",
	Long: `The NRFU command runs a series of tests against network devices to verify
that the network is ready for use. Tests are defined in a catalog file and
devices are specified in an inventory file.`,
	RunE: runNrfu,
}

func init() {
	NrfuCmd.Flags().StringVarP(&inventoryFile, "inventory", "i", "", "inventory file path (required unless using Netbox)")
	NrfuCmd.Flags().StringVarP(&catalogFile, "catalog", "C", "", "test catalog file path (required)")
	NrfuCmd.Flags().StringVar(&netboxURL, "netbox-url", "", "Netbox URL (can also use NETBOX_URL env var)")
	NrfuCmd.Flags().StringVar(&netboxToken, "netbox-token", "", "Netbox API token (can also use NETBOX_TOKEN env var)")
	NrfuCmd.Flags().StringVar(&netboxQuery, "netbox-query", "", "Netbox query filter (e.g., 'site=dc1,role=leaf')")
	NrfuCmd.Flags().StringVarP(&tags, "tags", "t", "", "filter devices by tags (comma-separated)")
	NrfuCmd.Flags().StringVarP(&devices, "devices", "d", "", "filter specific devices (comma-separated)")
	NrfuCmd.Flags().StringVarP(&tests, "tests", "T", "", "filter specific tests (comma-separated)")
	NrfuCmd.Flags().StringVar(&limit, "limit", "", "limit devices: hostname, comma-separated list (host1,host2), index (0), range (0-2), or wildcard (leaf*)")
	NrfuCmd.Flags().StringVar(&deviceUsername, "device-username", "", "device username (overrides DEVICE_USERNAME env var)")
	NrfuCmd.Flags().StringVar(&devicePassword, "device-password", "", "device password (overrides DEVICE_PASSWORD env var)")
	NrfuCmd.Flags().IntVarP(&concurrency, "concurrency", "j", 10, "maximum concurrent connections")
	NrfuCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be executed without running")
	NrfuCmd.Flags().BoolVar(&ignoreStatus, "ignore-status", false, "always return exit code 0")
	NrfuCmd.Flags().StringVar(&hide, "hide", "", "hide results by status (success, failure, error, skipped)")
	NrfuCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file path")
	NrfuCmd.Flags().StringVarP(&format, "format", "f", "table", "output format (table, csv, json, markdown)")
	NrfuCmd.Flags().StringVar(&logLevel, "log-level", "warn", "log level (trace, debug, info, warn, error, fatal)")
	NrfuCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output (equivalent to --log-level=debug)")
	NrfuCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "quiet mode - only show results (equivalent to --log-level=error)")
	NrfuCmd.Flags().BoolVar(&silent, "silent", false, "silent mode - no logging output during execution, only show final results")
	NrfuCmd.Flags().BoolVarP(&progress, "progress", "p", true, "show progress bars during test execution")

	_ = NrfuCmd.MarkFlagRequired("catalog")
}

func runNrfu(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Configure logging based on flags IMMEDIATELY before any other operations
	configureLogging()

	var inv *inventory.Inventory
	var err error

	// Check if Netbox is being used
	if netboxURL != "" || os.Getenv("NETBOX_URL") != "" {
		inv, err = loadNetboxInventory(ctx)
	} else if inventoryFile != "" {
		inv, err = inventory.LoadInventory(inventoryFile)
	} else {
		return fmt.Errorf("either --inventory or --netbox-url must be specified")
	}
	
	if err != nil {
		return fmt.Errorf("failed to load inventory: %w", err)
	}

	catalog, err := test.LoadCatalog(catalogFile)
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	if tags != "" {
		tagList := strings.Split(tags, ",")
		inv = inv.FilterByTags(tagList)
	}

	if devices != "" {
		deviceList := strings.Split(devices, ",")
		inv = inv.FilterByNames(deviceList)
	}

	if limit != "" {
		inv = inv.FilterByLimit(limit)
	}

	if tests != "" {
		testList := strings.Split(tests, ",")
		catalog = catalog.FilterByName(testList)
	}

	if dryRun {
		fmt.Printf("Would run %d tests on %d devices\n\n", len(catalog.Tests), len(inv.Devices))
		
		// Display devices in a table
		fmt.Println("Devices:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "Name\tHost\tTags")
		fmt.Fprintln(w, "----\t----\t----")
		for _, dev := range inv.Devices {
			tags := "-"
			if len(dev.Tags) > 0 {
				tags = strings.Join(dev.Tags, ", ")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", dev.Name, dev.Host, tags)
		}
		w.Flush()
		
		// Display tests
		fmt.Println("\nTests:")
		for _, t := range catalog.Tests {
			cats := ""
			if len(t.Categories) > 0 {
				cats = fmt.Sprintf(" [%s]", strings.Join(t.Categories, ", "))
			}
			fmt.Printf("  - %s (module: %s)%s\n", t.Name, t.Module, cats)
		}
		
		fmt.Printf("\nTotal: %d tests × %d devices = %d test executions\n", 
			len(catalog.Tests), len(inv.Devices), len(catalog.Tests)*len(inv.Devices))
		return nil
	}

	deviceList := make([]device.Device, 0, len(inv.Devices))
	for _, devConfig := range inv.Devices {
		dev := device.NewEOSDevice(devConfig)
		if err := dev.Connect(ctx); err != nil {
			if !silent {
				fmt.Fprintf(os.Stderr, "Warning: Failed to connect to %s: %v\n", devConfig.Name, err)
			}
			continue
		}
		deviceList = append(deviceList, dev)
		defer dev.Disconnect()
	}

	if len(deviceList) == 0 {
		return fmt.Errorf("no devices available for testing")
	}

	// Use progress runner if progress is enabled, otherwise use standard runner
	var results []test.TestResult
	if progress && !quiet && !silent {
		progressRunner := test.NewProgressRunner(concurrency, true)
		results, err = progressRunner.Run(ctx, catalog.Tests, deviceList)
	} else {
		runner := test.NewRunner(concurrency)
		results, err = runner.Run(ctx, catalog.Tests, deviceList)
	}
	if err != nil {
		return fmt.Errorf("failed to run tests: %w", err)
	}

	if hide != "" {
		results = filterResults(results, hide)
	}

	var rep reporter.Reporter
	switch format {
	case "csv":
		rep = reporter.NewCSVReporter()
	case "json":
		rep = reporter.NewJSONReporter()
	case "markdown":
		rep = reporter.NewMarkdownReporter()
	default:
		rep = reporter.NewTableReporter()
	}

	var output = os.Stdout
	if outputFile != "" {
		file, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	}

	rep.SetOutput(output)
	if err := rep.Report(results); err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if !ignoreStatus {
		for _, result := range results {
			if result.Status == test.TestFailure || result.Status == test.TestError {
				os.Exit(1)
			}
		}
	}

	return nil
}

func filterResults(results []test.TestResult, hide string) []test.TestResult {
	hideList := strings.Split(hide, ",")
	hideMap := make(map[string]bool)
	for _, h := range hideList {
		hideMap[strings.TrimSpace(h)] = true
	}

	filtered := make([]test.TestResult, 0)
	for _, result := range results {
		if !hideMap[result.Status.String()] {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func loadNetboxInventory(ctx context.Context) (*inventory.Inventory, error) {
	// Get Netbox configuration
	url := netboxURL
	if url == "" {
		url = os.Getenv("NETBOX_URL")
	}
	if url == "" {
		return nil, fmt.Errorf("Netbox URL is required (use --netbox-url or NETBOX_URL env var)")
	}

	token := netboxToken
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

	if netboxQuery != "" {
		// Handle both formats: "?site_id=14&platform_id=5" or "site=dc1,platform=eos"
		queryStr := strings.TrimPrefix(netboxQuery, "?")
		
		// Determine separator
		separator := ","
		if strings.Contains(queryStr, "&") {
			separator = "&"
		}
		
		pairs := strings.Split(queryStr, separator)
		for _, pair := range pairs {
			kv := strings.Split(pair, "=")
			if len(kv) != 2 {
				continue
			}
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch key {
			case "site", "site__slug":
				query.Site = value
			case "site_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.SiteID = id
				}
			case "role", "role__slug", "device_role", "device_role__slug":
				query.Role = value
			case "role_id", "device_role_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.RoleID = id
				}
			case "device_type", "device_type__slug":
				query.DeviceType = value
			case "device_type_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.DeviceTypeID = id
				}
			case "manufacturer", "manufacturer__slug":
				query.Manufacturer = value
			case "manufacturer_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.ManufacturerID = id
				}
			case "platform", "platform__slug":
				query.Platform = value
			case "platform_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.PlatformID = id
				}
			case "status":
				query.Status = value
			case "tenant", "tenant__slug":
				query.Tenant = value
			case "tenant_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.TenantID = id
				}
			case "region", "region__slug":
				query.Region = value
			case "region_id":
				if id, err := strconv.Atoi(value); err == nil {
					query.RegionID = id
				}
			case "name":
				query.Name = value
			case "name__ic", "name_contains":
				query.NameContains = value
			case "tag":
				query.Tags = append(query.Tags, value)
			}
		}
	}

	// Get device credentials - CLI flags override env vars
	credentials := make(map[string]interface{})
	username := deviceUsername
	if username == "" {
		username = os.Getenv("DEVICE_USERNAME")
	}
	if username == "" {
		username = "admin"  // Default username
	}
	credentials["username"] = username
	
	password := devicePassword
	if password == "" {
		password = os.Getenv("DEVICE_PASSWORD")
	}
	if password != "" {
		credentials["password"] = password
	}
	if enablePassword := os.Getenv("DEVICE_ENABLE_PASSWORD"); enablePassword != "" {
		credentials["enable_password"] = enablePassword
	}
	credentials["insecure"] = true // Default for lab environments

	config := inventory.NetboxConfig{
		URL:      url,
		Token:    token,
		Insecure: os.Getenv("NETBOX_INSECURE") == "true",
	}

	fmt.Printf("Loading devices from Netbox: %s\n", url)
	return inventory.LoadFromNetbox(config, query, credentials)
}

// configureLogging sets up logging based on command line flags
func configureLogging() {
	// Handle flag precedence: silent > quiet > verbose > log-level
	if silent {
		// Silent mode: no output at all during execution
		logger.SetLevel("fatal")
	} else if quiet {
		logger.SetLevel("error")
	} else if verbose {
		logger.SetLevel("debug")
	} else if progress && !verbose {
		// When progress bars are enabled and not in verbose mode, suppress most logging
		// to keep the progress display clean
		logger.SetLevel("error")
	} else {
		logger.SetLevel(logLevel)
	}
}