package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/fluidstackio/go-anta/internal/logger"
	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/inventory"
	"github.com/fluidstackio/go-anta/pkg/reporter"
	"github.com/fluidstackio/go-anta/pkg/test"
	"github.com/spf13/cobra"
)

var (
	inventoryFile  string
	catalogFile    string
	netboxURL      string
	netboxToken    string
	netboxQuery    string
	tags           string
	devices        string
	tests          string
	limit          string
	deviceUsername string
	devicePassword string
	concurrency    int
	dryRun         bool
	ignoreStatus   bool
	hide           string
	outputFile     string
	logLevel       string
	verbose        bool
	quiet          bool
	progress       bool
	silent         bool
	transport      string
	source         string
	region         string
	filter         string
	plaintext      bool
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
	NrfuCmd.Flags().StringVar(&transport, "transport", "", "transport for device connections: eapi or gnmi. When set, overrides per-device YAML transport; otherwise the YAML value is used (or eapi if unset).")
	NrfuCmd.Flags().StringVar(&source, "source", "", "override the YAML inventory kind (file, netbox, dcfab)")
	NrfuCmd.Flags().StringVar(&region, "region", "", "dcfab region filter")
	NrfuCmd.Flags().StringVar(&filter, "filter", "", "dcfab GraphQL filter (e.g. 'roles: [\"fm0\"], platforms: [\"eos\"]'); overrides YAML filter")
	NrfuCmd.Flags().BoolVar(&plaintext, "plaintext", false, "use plaintext gRPC for gnmi transport (no TLS); ignored for eapi")
	NrfuCmd.Flags().IntVarP(&concurrency, "concurrency", "j", 10, "maximum concurrent connections")
	NrfuCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be executed without running")
	NrfuCmd.Flags().BoolVar(&ignoreStatus, "ignore-status", false, "always return exit code 0")
	NrfuCmd.Flags().StringVar(&hide, "hide", "", "hide results by status (success, failure, error, skipped)")
	NrfuCmd.Flags().StringVarP(&outputFile, "output", "o", "", "output file path (default: report.html in cwd; use - for stdout)")
	NrfuCmd.Flags().StringVar(&logLevel, "log-level", "warn", "log level (trace, debug, info, warn, error, fatal)")
	NrfuCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output (equivalent to --log-level=debug)")
	NrfuCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "quiet mode - only show results (equivalent to --log-level=error)")
	NrfuCmd.Flags().BoolVar(&silent, "silent", false, "silent mode - no logging output during execution, only show final results")
	NrfuCmd.Flags().BoolVarP(&progress, "progress", "p", true, "show progress bars during test execution")

	_ = NrfuCmd.MarkFlagRequired("catalog")
}

func runNrfu(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Validate the transport override early so a bad value produces a
	// clear error instead of silently failing every device-construct
	// call inside the connect loop.
	switch transport {
	case "", "eapi", "gnmi":
	default:
		return fmt.Errorf("unknown --transport value %q (supported: eapi, gnmi)", transport)
	}

	// Configure logging based on flags IMMEDIATELY before any other operations
	configureLogging()
	runStart := time.Now()

	inv, err := LoadInventoryForRun(ctx, InventoryLoadOptions{
		Path:           inventoryFile,
		SourceOverride: source,
		NetboxURL:      netboxURL,
		NetboxToken:    netboxToken,
		NetboxQuery:    netboxQuery,
		Region:         region,
		Filter:         filter,
		Defaults: inventory.DeviceDefaults{
			Username:  deviceUsername,
			Password:  devicePassword,
			Transport: transport,
			Insecure:  true, // existing default for lab use
			Plaintext: plaintext,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to load inventory: %w", err)
	}

	catalog, err := test.LoadCatalog(catalogFile)
	if err != nil {
		return fmt.Errorf("failed to load catalog: %w", err)
	}

	if tags != "" {
		var fErr error
		inv, fErr = inv.FilterByTags(strings.Split(tags, ","))
		if fErr != nil {
			return fmt.Errorf("--tags filter: %w", fErr)
		}
	}

	if devices != "" {
		var fErr error
		inv, fErr = inv.FilterByNames(strings.Split(devices, ","))
		if fErr != nil {
			return fmt.Errorf("--devices filter: %w", fErr)
		}
	}

	if limit != "" {
		var fErr error
		inv, fErr = inv.FilterByLimit(limit)
		if fErr != nil {
			return fmt.Errorf("--limit filter: %w", fErr)
		}
	}

	if tests != "" {
		var fErr error
		catalog, fErr = catalog.FilterByName(strings.Split(tests, ","))
		if fErr != nil {
			return fmt.Errorf("--tests filter: %w", fErr)
		}
	}

	// R7: catch typos in catalog (Module, Name) tuples upfront. Without
	// this, an unknown test surfaces as a per-device "Test not found"
	// — N devices × M unknown tests duplicate errors instead of one.
	if err := catalog.ValidateAgainst(test.GetRegistry()); err != nil {
		return fmt.Errorf("catalog: %w", err)
	}

	if len(inv.Devices) == 0 {
		return fmt.Errorf("no devices to run against (check your inventory / filters)")
	}
	if len(catalog.Tests) == 0 {
		return fmt.Errorf("no tests to run (check your catalog / filters)")
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
	deviceInfo := make([]reporter.DeviceInfo, 0, len(inv.Devices))
	for _, devConfig := range inv.Devices {
		if transport != "" {
			devConfig.Transport = transport
		}
		info := reporter.DeviceInfo{
			Name:      devConfig.Name,
			Host:      devConfig.Host,
			Transport: devConfig.Transport,
			Port:      devConfig.Port,
			Tags:      devConfig.Tags,
		}
		dev, err := device.New(devConfig)
		if err != nil {
			logger.Errorf("Failed to construct device %s: %v", devConfig.Name, err)
			info.ConnectError = err.Error()
			deviceInfo = append(deviceInfo, info)
			continue
		}
		if err := dev.Connect(ctx); err != nil {
			if !silent {
				fmt.Fprintf(os.Stderr, "Warning: Failed to connect to %s: %v\n", devConfig.Name, err)
			}
			info.ConnectError = err.Error()
			deviceInfo = append(deviceInfo, info)
			continue
		}
		info.Connected = true
		info.Model = dev.HardwareModel()
		// EOS version is exposed in `show version`'s `version` field;
		// Connect already ran the probe but doesn't store the version,
		// so query once more here. Cheap, and only on devices the test
		// run will actually use.
		if v, err := dev.Execute(ctx, device.Command{Template: "show version", Format: "json", UseCache: true}); err == nil {
			if m, ok := v.Output.(map[string]any); ok {
				if ver, ok := m["version"].(string); ok {
					info.EOSVersion = ver
				}
			}
		}
		deviceList = append(deviceList, dev)
		deviceInfo = append(deviceInfo, info)
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

	report := &reporter.Report{
		Title:     fmt.Sprintf("nrfu — %s", catalogFile),
		Started:   runStart,
		Completed: time.Now(),
		Devices:   deviceInfo,
		Results:   results,
	}

	// Default output is report.html in the current directory so the
	// user doesn't get binary-ish HTML dumped to their terminal. Pass
	// --output - to force stdout, or --output path.html to override.
	outPath := outputFile
	if outPath == "" {
		outPath = "report.html"
	}
	var output io.Writer = os.Stdout
	if outPath != "-" {
		file, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()
		output = file
	}
	if err := reporter.Render(output, report); err != nil {
		return fmt.Errorf("failed to render HTML report: %w", err)
	}
	if outPath != "-" && !silent {
		fmt.Fprintf(os.Stderr, "Report written to %s\n", outPath)
	}

	if !ignoreStatus {
		for _, result := range results {
			if result.Status == test.TestFailure || result.Status == test.TestError {
				return ErrTestsFailed
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
