# GO-ANTA API Documentation

GO-ANTA is a comprehensive network testing and validation framework for Arista EOS devices, providing both CLI tools and programmatic APIs for network automation and testing.

## Table of Contents

1. [Overview](#overview)
2. [CLI Commands](#cli-commands)
3. [Test Framework API](#test-framework-api)
4. [Device Management API](#device-management-api)
5. [Inventory Management](#inventory-management)
6. [Reporter System](#reporter-system)
7. [Configuration Management](#configuration-management)
8. [Platform Support](#platform-support)
9. [Extension Guide](#extension-guide)
10. [Examples](#examples)

## Overview

GO-ANTA provides multiple layers of APIs:

- **CLI Commands**: User-facing command-line interface
- **Test Framework**: Core testing infrastructure and test implementations
- **Device API**: Device connection and command execution
- **Inventory API**: Static and dynamic device inventory management
- **Reporter API**: Pluggable result formatting and output
- **Configuration API**: Application and device configuration management

### Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   CLI Layer     │    │  Test Framework │    │  Reporter Layer │
│   (Commands)    │────│   (Core API)    │────│   (Output)     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                   ┌─────────────────┐
                   │  Device Layer   │
                   │  (EOS API)      │
                   └─────────────────┘
                            │
                   ┌─────────────────┐
                   │ Inventory Layer │
                   │ (Static/Netbox) │
                   └─────────────────┘
```

## CLI Commands

### NRFU Command

The Network Ready For Use (NRFU) command is the primary interface for running network tests.

#### Syntax

```bash
go-anta nrfu [flags]
```

#### Core Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--catalog` | `-C` | string | *required* | Test catalog file path |
| `--inventory` | `-i` | string | *required* | Inventory file path (unless using Netbox) |
| `--format` | `-f` | string | `table` | Output format (table, csv, json, markdown) |
| `--concurrency` | `-j` | int | `10` | Maximum concurrent connections |
| `--dry-run` | | bool | `false` | Show what would be executed without running |
| `--progress` | `-p` | bool | `true` | Show progress bars during execution |

#### Filtering Flags

| Flag | Type | Description |
|------|------|-------------|
| `--devices` | string | Filter specific devices (comma-separated) |
| `--tests` | string | Filter specific tests (comma-separated) |
| `--tags` | string | Filter devices by tags (comma-separated) |
| `--limit` | string | Limit devices (hostname, range, or wildcard) |
| `--hide` | string | Hide results by status (success, failure, error, skipped) |

#### Netbox Integration Flags

| Flag | Type | Description |
|------|------|-------------|
| `--netbox-url` | string | Netbox URL (or NETBOX_URL env var) |
| `--netbox-token` | string | Netbox API token (or NETBOX_TOKEN env var) |
| `--netbox-query` | string | Netbox query filter |

#### Logging and Output Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--log-level` | | string | `warn` | Log level (trace, debug, info, warn, error, fatal) |
| `--verbose` | `-v` | bool | `false` | Enable verbose output (debug level) |
| `--quiet` | `-q` | bool | `false` | Quiet mode (error level only) |
| `--silent` | | bool | `false` | Silent mode (no logging during execution) |
| `--output` | `-o` | string | | Output file path |

#### Device Authentication Flags

| Flag | Type | Description |
|------|------|-------------|
| `--device-username` | string | Device username (overrides DEVICE_USERNAME env var) |
| `--device-password` | string | Device password (overrides DEVICE_PASSWORD env var) |

#### Examples

```bash
# Basic test execution
go-anta nrfu -i inventory.yaml -C catalog.yaml

# Filter by devices and tests
go-anta nrfu -i inventory.yaml -C catalog.yaml --devices="leaf1,spine1" --tests="VerifyBGPPeers"

# Use Netbox inventory
go-anta nrfu --netbox-url="https://netbox.example.com" --netbox-token="your-token" \
  --netbox-query="site=dc1,role=leaf" -C catalog.yaml

# Export results to JSON
go-anta nrfu -i inventory.yaml -C catalog.yaml --format=json --output=results.json

# Dry run to preview execution
go-anta nrfu -i inventory.yaml -C catalog.yaml --dry-run

# Silent execution with progress bars
go-anta nrfu -i inventory.yaml -C catalog.yaml --silent
```

### Check Command

Verify device connectivity and basic information.

#### Syntax

```bash
go-anta check [flags]
```

#### Examples

```bash
# Check connectivity for all devices
go-anta check -i inventory.yaml

# Check specific devices
go-anta check -i inventory.yaml --devices="leaf1,spine1"

# Use Netbox for device discovery
go-anta check --netbox-url="https://netbox.example.com" --netbox-token="your-token"
```

### Inventory Command

View and validate inventory configurations.

#### Syntax

```bash
go-anta inventory [flags]
```

#### Examples

```bash
# List all devices in table format
go-anta inventory -i inventory.yaml

# Export inventory to JSON
go-anta inventory -i inventory.yaml --format=json

# Show device count only
go-anta inventory -i inventory.yaml --format=count

# Validate Netbox inventory
go-anta inventory --netbox-url="https://netbox.example.com" --netbox-token="your-token"
```

## Test Framework API

### Core Interfaces

#### Test Interface

The `Test` interface defines the contract for all network tests:

```go
package test

import (
    "context"
    "github.com/gavmckee/go-anta/internal/device"
)

type Test interface {
    // Test identification
    Name() string
    Description() string
    Categories() []string

    // Test execution
    Execute(ctx context.Context, dev device.Device) (*TestResult, error)

    // Input validation
    ValidateInput(input interface{}) error
}
```

#### TestResult Structure

```go
type TestResult struct {
    TestName   string        `json:"test_name"`
    DeviceName string        `json:"device_name"`
    Status     TestStatus    `json:"status"`
    Message    string        `json:"message"`
    Timestamp  time.Time     `json:"timestamp"`
    Duration   time.Duration `json:"duration"`
    Categories []string      `json:"categories"`
    Details    interface{}   `json:"details,omitempty"`
}

type TestStatus int

const (
    TestSuccess TestStatus = iota
    TestFailure
    TestError
    TestSkipped
)
```

### Test Registry

The test registry manages test discovery and instantiation:

```go
package test

type Registry struct {
    tests map[string]map[string]TestFactory
}

type TestFactory func(inputs map[string]interface{}) (Test, error)

// Register a test factory
func Register(module, name string, factory TestFactory) error

// Get a test instance
func GetTest(module, name string) (Test, error)

// Get a test instance with inputs
func GetTestWithInputs(module, name string, inputs map[string]interface{}) (Test, error)

// List all available tests
func ListTests() map[string][]string
```

#### Usage Example

```go
// Register a custom test
err := test.Register("custom", "VerifyCustomFunction", func(inputs map[string]interface{}) (test.Test, error) {
    return NewVerifyCustomFunction(inputs)
})

// Get a test instance
testInstance, err := test.GetTestWithInputs("routing", "VerifyBGPPeers", map[string]interface{}{
    "peers": []map[string]interface{}{
        {"peer": "10.0.0.1", "state": "Established", "asn": 65001},
    },
})
```

### Test Catalog

Test catalogs define which tests to run and their configuration:

```go
type Catalog struct {
    Tests []TestDefinition `yaml:"tests" json:"tests"`
}

type TestDefinition struct {
    Name       string                 `yaml:"name" json:"name"`
    Module     string                 `yaml:"module" json:"module"`
    Inputs     map[string]interface{} `yaml:"inputs,omitempty" json:"inputs,omitempty"`
    Categories []string               `yaml:"categories,omitempty" json:"categories,omitempty"`
    Tags       []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
}
```

#### Catalog File Example

```yaml
tests:
  - name: "VerifyBGPPeers"
    module: "routing"
    categories: ["routing", "bgp"]
    inputs:
      peers:
        - peer: "10.0.0.1"
          state: "Established"
          asn: 65001
        - peer: "10.0.0.2"
          state: "Established"
          asn: 65002

  - name: "VerifyTemperature"
    module: "hardware"
    categories: ["hardware", "environmental"]
    inputs:
      check_temp_sensors: true
      failure_margin: 5
```

### Test Runner

The test runner executes tests concurrently across devices:

```go
type Runner struct {
    maxConcurrency int
    results        []TestResult
    mu             sync.Mutex
}

func NewRunner(maxConcurrency int) *Runner

func (r *Runner) Run(ctx context.Context, tests []TestDefinition, devices []device.Device) ([]TestResult, error)
```

#### Progress Runner

Enhanced runner with visual progress tracking:

```go
type ProgressRunner struct {
    *Runner
    enableProgress bool
    pw             progress.Writer
}

func NewProgressRunner(maxConcurrency int, enableProgress bool) *ProgressRunner
```

#### Usage Example

```go
// Create a runner
runner := test.NewRunner(10) // 10 concurrent tests

// Load catalog and inventory
catalog, err := test.LoadCatalog("catalog.yaml")
inventory, err := inventory.LoadInventory("inventory.yaml")

// Connect to devices
var devices []device.Device
for _, devConfig := range inventory.Devices {
    dev := device.NewEOSDevice(devConfig)
    if err := dev.Connect(ctx); err != nil {
        continue
    }
    devices = append(devices, dev)
    defer dev.Disconnect()
}

// Run tests
results, err := runner.Run(ctx, catalog.Tests, devices)
```

### Available Test Modules

#### Connectivity Tests

| Test Name | Description | Key Inputs |
|-----------|-------------|------------|
| `VerifyReachability` | Test network reachability | `hosts`, `repeat`, `size` |
| `VerifyLLDPNeighbors` | Verify LLDP neighbor adjacencies | `interfaces` |

#### Hardware Tests

| Test Name | Description | Key Inputs |
|-----------|-------------|------------|
| `VerifyTemperature` | Check device temperature sensors | `check_temp_sensors`, `failure_margin` |
| `VerifyTransceivers` | Validate optical transceivers | `check_manufacturer`, `manufacturers` |
| `VerifyPowerSupplies` | Check power supply status | `minimum_supplies` |
| `VerifyInventory` | Verify hardware inventory | `check_psus`, `check_fans` |

#### Routing Tests

| Test Name | Description | Key Inputs |
|-----------|-------------|------------|
| `VerifyBGPPeers` | Verify BGP peer states | `peers` |
| `VerifyBGPPeerCount` | Check BGP peer counts | `address_families` |
| `VerifyBGPSpecificPeers` | Validate specific BGP peers | `address_families`, `bgp_peers` |
| `VerifyBFDPeers` | Check BFD peer status | `peers` |
| `VerifyStaticRoutes` | Verify static routes | `routes` |

#### System Tests

| Test Name | Description | Key Inputs |
|-----------|-------------|------------|
| `VerifyEOSVersion` | Check EOS software version | `minimum_version`, `versions` |
| `VerifyUptime` | Verify device uptime | `minimum_uptime` |
| `VerifyNTP` | Check NTP synchronization | `servers` |
| `VerifyDNSResolution` | Test DNS resolution | `servers`, `fqdn` |

#### Security Tests

| Test Name | Description | Key Inputs |
|-----------|-------------|------------|
| `VerifyAPISSLCertificate` | Validate API SSL certificates | `certificates` |
| `VerifySSHStatus` | Check SSH service status | `enabled` |
| `VerifyTelnetStatus` | Check Telnet service status | `enabled` |

### Creating Custom Tests

#### Base Test Structure

```go
type VerifyCustomTest struct {
    test.BaseTest
    CustomParam string `yaml:"custom_param" json:"custom_param"`
}

func NewVerifyCustomTest(inputs map[string]any) (test.Test, error) {
    t := &VerifyCustomTest{
        BaseTest: test.BaseTest{
            TestName:        "VerifyCustomTest",
            TestDescription: "Custom test description",
            TestCategories:  []string{"custom"},
        },
    }

    if inputs != nil {
        if param, ok := inputs["custom_param"].(string); ok {
            t.CustomParam = param
        }
    }

    return t, nil
}

func (t *VerifyCustomTest) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
    result := &test.TestResult{
        TestName:   t.Name(),
        DeviceName: dev.Name(),
        Status:     test.TestSuccess,
        Categories: t.Categories(),
    }

    // Execute device commands
    cmd := device.Command{
        Template: "show custom command",
        Format:   "json",
        UseCache: false,
    }

    cmdResult, err := dev.Execute(ctx, cmd)
    if err != nil {
        result.Status = test.TestError
        result.Message = fmt.Sprintf("Failed to execute command: %v", err)
        return result, nil
    }

    // Process results
    // ... test logic here ...

    return result, nil
}

func (t *VerifyCustomTest) ValidateInput(input any) error {
    if t.CustomParam == "" {
        return fmt.Errorf("custom_param is required")
    }
    return nil
}
```

#### Registering Custom Tests

```go
func init() {
    test.Register("custom", "VerifyCustomTest", NewVerifyCustomTest)
}
```

## Device Management API

### Device Interface

The core device interface provides standardized access to network devices:

```go
package device

import "context"

type Device interface {
    // Device identification
    Name() string
    Host() string
    Tags() []string

    // Connection management
    Connect(ctx context.Context) error
    Disconnect() error
    IsOnline() bool
    IsEstablished() bool

    // Command execution
    Execute(ctx context.Context, cmd Command) (*CommandResult, error)
    ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error)

    // Device information
    HardwareModel() string
    Refresh(ctx context.Context) error
}
```

### Device Configuration

```go
type DeviceConfig struct {
    Name           string            `yaml:"name" json:"name"`
    Host           string            `yaml:"host" json:"host"`
    Port           int               `yaml:"port,omitempty" json:"port,omitempty"`
    Username       string            `yaml:"username" json:"username"`
    Password       string            `yaml:"password" json:"password"`
    EnablePassword string            `yaml:"enable_password,omitempty" json:"enable_password,omitempty"`
    Tags           []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
    Timeout        time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
    Insecure       bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
    DisableCache   bool              `yaml:"disable_cache,omitempty" json:"disable_cache,omitempty"`
    Extra          map[string]string `yaml:"extra,omitempty" json:"extra,omitempty"`
}
```

### Command Structure

```go
type Command struct {
    Template   string                 `json:"template"`
    Format     string                 `json:"format"`
    UseCache   bool                   `json:"use_cache"`
    Revision   int                    `json:"revision,omitempty"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type CommandResult struct {
    Output    interface{} `json:"output"`
    Error     error       `json:"error,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
    Duration  time.Duration `json:"duration"`
    Cached    bool        `json:"cached"`
}
```

### EOS Device Implementation

The EOS device implementation provides HTTP-based eAPI communication:

```go
func NewEOSDevice(config DeviceConfig) Device

// Usage example
dev := device.NewEOSDevice(device.DeviceConfig{
    Name:     "leaf1",
    Host:     "192.168.1.10",
    Port:     443,
    Username: "admin",
    Password: "admin123",
    Insecure: true,
})

err := dev.Connect(ctx)
if err != nil {
    return err
}
defer dev.Disconnect()

// Execute single command
cmd := device.Command{
    Template: "show interfaces status",
    Format:   "json",
    UseCache: false,
}
result, err := dev.Execute(ctx, cmd)

// Execute batch commands
cmds := []device.Command{
    {Template: "show version", Format: "json"},
    {Template: "show ip bgp summary", Format: "json"},
}
results, err := dev.ExecuteBatch(ctx, cmds)
```

### Command Caching

The device implementation includes intelligent command caching:

- **TTL-based expiration**: Commands are cached with configurable TTL
- **Context-aware**: Cache keys include device context
- **Thread-safe**: Concurrent access protection
- **Memory efficient**: LRU eviction for memory management

## Inventory Management

### Static Inventory

Static inventory loads devices from YAML configuration files:

```go
package inventory

type Inventory struct {
    Devices  []DeviceConfig   `yaml:"devices" json:"devices"`
    Networks []NetworkConfig  `yaml:"networks,omitempty" json:"networks,omitempty"`
    Ranges   []RangeConfig    `yaml:"ranges,omitempty" json:"ranges,omitempty"`
}

func LoadInventory(filename string) (*Inventory, error)
```

#### Inventory File Format

```yaml
devices:
  - name: "leaf1"
    host: "192.168.1.10"
    port: 443
    username: "admin"
    password: "admin123"
    tags: ["leaf", "datacenter"]
    insecure: true

networks:
  - network: "192.168.2.0/24"
    username: "admin"
    password: "admin123"
    tags: ["management"]
    insecure: true

ranges:
  - start: "10.0.1.1"
    end: "10.0.1.10"
    username: "admin"
    password: "admin123"
    tags: ["test"]
    port: 443
    insecure: true
```

#### Filtering Methods

```go
// Filter by device names
func (inv *Inventory) FilterByNames(names []string) *Inventory

// Filter by tags
func (inv *Inventory) FilterByTags(tags []string) *Inventory

// Filter by limit expression
func (inv *Inventory) FilterByLimit(limit string) *Inventory
```

### Netbox Integration

Dynamic inventory integration with Netbox IPAM/DCIM:

```go
type NetboxConfig struct {
    URL      string `yaml:"url" json:"url"`
    Token    string `yaml:"token" json:"token"`
    Insecure bool   `yaml:"insecure" json:"insecure"`
}

type NetboxQuery struct {
    Site           string   `json:"site,omitempty"`
    SiteID         int      `json:"site_id,omitempty"`
    Role           string   `json:"role,omitempty"`
    RoleID         int      `json:"role_id,omitempty"`
    DeviceType     string   `json:"device_type,omitempty"`
    DeviceTypeID   int      `json:"device_type_id,omitempty"`
    Manufacturer   string   `json:"manufacturer,omitempty"`
    ManufacturerID int      `json:"manufacturer_id,omitempty"`
    Platform       string   `json:"platform,omitempty"`
    PlatformID     int      `json:"platform_id,omitempty"`
    Status         string   `json:"status,omitempty"`
    Tenant         string   `json:"tenant,omitempty"`
    TenantID       int      `json:"tenant_id,omitempty"`
    Region         string   `json:"region,omitempty"`
    RegionID       int      `json:"region_id,omitempty"`
    Name           string   `json:"name,omitempty"`
    NameContains   string   `json:"name_contains,omitempty"`
    Tags           []string `json:"tags,omitempty"`
    IncludeInactive bool    `json:"include_inactive,omitempty"`
}

func LoadFromNetbox(config NetboxConfig, query NetboxQuery, credentials map[string]interface{}) (*Inventory, error)
```

#### Netbox Usage Examples

```go
// Load devices from Netbox
config := inventory.NetboxConfig{
    URL:   "https://netbox.example.com",
    Token: "your-api-token",
}

query := inventory.NetboxQuery{
    Site:   "dc1",
    Role:   "leaf",
    Status: "active",
}

credentials := map[string]interface{}{
    "username": "admin",
    "password": "admin123",
    "insecure": true,
}

inv, err := inventory.LoadFromNetbox(config, query, credentials)
```

## Reporter System

### Reporter Interface

The reporter system provides pluggable output formatting:

```go
package reporter

import "io"

type Reporter interface {
    Report(results []test.TestResult) error
    SetOutput(w io.Writer)
    SetFormat(format string)
}
```

### Available Reporters

#### Table Reporter

Professional table output with colors and device grouping:

```go
func NewTableReporter() *TableReporter
```

**Features:**
- Device-grouped results with merged cells
- Color-coded status indicators
- Professional styling with borders
- Summary statistics with percentages
- Success rate with contextual messages

#### CSV Reporter

Comma-separated values for data analysis:

```go
func NewCSVReporter() *CSVReporter
```

**Output columns:** Device, Test, Status, Message, Duration, Categories

#### JSON Reporter

Structured JSON output for programmatic processing:

```go
func NewJSONReporter() *JSONReporter
```

**Output format:**
```json
{
  "results": [
    {
      "test_name": "VerifyBGPPeers",
      "device_name": "leaf1",
      "status": "success",
      "message": "",
      "timestamp": "2023-01-01T00:00:00Z",
      "duration": 1500000000,
      "categories": ["routing", "bgp"]
    }
  ],
  "summary": {
    "total": 10,
    "success": 8,
    "failure": 1,
    "error": 1,
    "skipped": 0
  }
}
```

#### Markdown Reporter

GitHub-flavored markdown for documentation:

```go
func NewMarkdownReporter() *MarkdownReporter
```

### Custom Reporter Implementation

```go
type CustomReporter struct {
    output io.Writer
}

func NewCustomReporter() *CustomReporter {
    return &CustomReporter{output: os.Stdout}
}

func (r *CustomReporter) SetOutput(w io.Writer) {
    r.output = w
}

func (r *CustomReporter) SetFormat(format string) {
    // Handle format-specific configuration
}

func (r *CustomReporter) Report(results []test.TestResult) error {
    // Custom formatting logic
    for _, result := range results {
        fmt.Fprintf(r.output, "Custom format: %s - %s\n",
            result.DeviceName, result.TestName)
    }
    return nil
}
```

## Configuration Management

### Configuration Structure

```go
package config

type Config struct {
    Log      LogConfig      `yaml:"log" json:"log"`
    Device   DeviceConfig   `yaml:"device" json:"device"`
    Test     TestConfig     `yaml:"test" json:"test"`
    Reporter ReporterConfig `yaml:"reporter" json:"reporter"`
}

type LogConfig struct {
    Level  string `yaml:"level" json:"level"`
    File   string `yaml:"file,omitempty" json:"file,omitempty"`
    Format string `yaml:"format" json:"format"`
}

type DeviceConfig struct {
    Timeout     time.Duration `yaml:"timeout" json:"timeout"`
    Concurrency int           `yaml:"concurrency" json:"concurrency"`
    Retries     int           `yaml:"retries" json:"retries"`
}

type TestConfig struct {
    Timeout     time.Duration `yaml:"timeout" json:"timeout"`
    CacheEnable bool          `yaml:"cache_enable" json:"cache_enable"`
    CacheTTL    time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
}

type ReporterConfig struct {
    Format string `yaml:"format" json:"format"`
    Output string `yaml:"output,omitempty" json:"output,omitempty"`
}
```

### Configuration File Example

```yaml
log:
  level: "info"
  file: "/var/log/go-anta.log"
  format: "json"

device:
  timeout: "30s"
  concurrency: 10
  retries: 3

test:
  timeout: "60s"
  cache_enable: true
  cache_ttl: "5m"

reporter:
  format: "table"
  output: "results.json"
```

### Loading Configuration

```go
func LoadConfig(filename string) (*Config, error)

// Usage
config, err := config.LoadConfig("config.yaml")
if err != nil {
    return err
}
```

### Environment Variable Support

Configuration supports environment variable overrides:

- `GO_ANTA_LOG_LEVEL`: Override log level
- `GO_ANTA_DEVICE_TIMEOUT`: Override device timeout
- `GO_ANTA_TEST_CACHE_TTL`: Override test cache TTL
- `NETBOX_URL`: Netbox URL
- `NETBOX_TOKEN`: Netbox API token
- `DEVICE_USERNAME`: Default device username
- `DEVICE_PASSWORD`: Default device password

## Platform Support

### Platform Detection

```go
package platform

var VirtualPlatforms = []string{
    "cEOSLab",
    "vEOS-lab",
    "cEOSCloudLab",
    "vEOS",
}

func IsVirtualPlatform(hardwareModel string) bool
func GetPlatformType(hardwareModel string) string
```

### Platform-Specific Test Skipping

Tests can automatically skip execution on virtual platforms:

```go
func (t *MyHardwareTest) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
    if platform.IsVirtualPlatform(dev.HardwareModel()) {
        return &test.TestResult{
            TestName:   t.Name(),
            DeviceName: dev.Name(),
            Status:     test.TestSkipped,
            Message:    fmt.Sprintf("Test skipped on virtual platform: %s", dev.HardwareModel()),
        }, nil
    }

    // Continue with test execution
    // ...
}
```

## Extension Guide

### Adding Custom Tests

1. **Create test structure**:
```go
type VerifyCustomFeature struct {
    test.BaseTest
    FeatureParam string `yaml:"feature_param" json:"feature_param"`
}
```

2. **Implement factory function**:
```go
func NewVerifyCustomFeature(inputs map[string]any) (test.Test, error) {
    // Implementation
}
```

3. **Implement test interface**:
```go
func (t *VerifyCustomFeature) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
    // Test logic
}

func (t *VerifyCustomFeature) ValidateInput(input any) error {
    // Input validation
}
```

4. **Register the test**:
```go
func init() {
    test.Register("custom", "VerifyCustomFeature", NewVerifyCustomFeature)
}
```

### Adding Custom Reporters

1. **Implement reporter interface**:
```go
type CustomReporter struct {
    output io.Writer
}

func (r *CustomReporter) Report(results []test.TestResult) error {
    // Custom formatting
}
```

2. **Register in command handler**:
```go
switch format {
case "custom":
    rep = reporter.NewCustomReporter()
default:
    rep = reporter.NewTableReporter()
}
```

### Adding Device Types

1. **Implement device interface**:
```go
type CustomDevice struct {
    config DeviceConfig
}

func (d *CustomDevice) Connect(ctx context.Context) error {
    // Connection logic
}

func (d *CustomDevice) Execute(ctx context.Context, cmd Command) (*CommandResult, error) {
    // Command execution
}
```

2. **Create factory function**:
```go
func NewCustomDevice(config DeviceConfig) Device {
    return &CustomDevice{config: config}
}
```

## Examples

### Complete Testing Workflow

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/gavmckee/go-anta/internal/device"
    "github.com/gavmckee/go-anta/internal/inventory"
    "github.com/gavmckee/go-anta/internal/reporter"
    "github.com/gavmckee/go-anta/internal/test"
)

func main() {
    ctx := context.Background()

    // Load test catalog
    catalog, err := test.LoadCatalog("catalog.yaml")
    if err != nil {
        fmt.Printf("Error loading catalog: %v\n", err)
        os.Exit(1)
    }

    // Load device inventory
    inv, err := inventory.LoadInventory("inventory.yaml")
    if err != nil {
        fmt.Printf("Error loading inventory: %v\n", err)
        os.Exit(1)
    }

    // Connect to devices
    var devices []device.Device
    for _, devConfig := range inv.Devices {
        dev := device.NewEOSDevice(devConfig)
        if err := dev.Connect(ctx); err != nil {
            fmt.Printf("Failed to connect to %s: %v\n", devConfig.Name, err)
            continue
        }
        devices = append(devices, dev)
        defer dev.Disconnect()
    }

    // Create test runner with progress
    runner := test.NewProgressRunner(10, true)

    // Execute tests
    results, err := runner.Run(ctx, catalog.Tests, devices)
    if err != nil {
        fmt.Printf("Error running tests: %v\n", err)
        os.Exit(1)
    }

    // Generate report
    reporter := reporter.NewTableReporter()
    if err := reporter.Report(results); err != nil {
        fmt.Printf("Error generating report: %v\n", err)
        os.Exit(1)
    }
}
```

### Custom Test Implementation

```go
package custom

import (
    "context"
    "fmt"

    "github.com/gavmckee/go-anta/internal/device"
    "github.com/gavmckee/go-anta/internal/test"
)

type VerifyCustomProtocol struct {
    test.BaseTest
    Protocol string `yaml:"protocol" json:"protocol"`
    Port     int    `yaml:"port" json:"port"`
}

func NewVerifyCustomProtocol(inputs map[string]any) (test.Test, error) {
    t := &VerifyCustomProtocol{
        BaseTest: test.BaseTest{
            TestName:        "VerifyCustomProtocol",
            TestDescription: "Verify custom protocol status",
            TestCategories:  []string{"custom", "protocol"},
        },
    }

    if inputs != nil {
        if protocol, ok := inputs["protocol"].(string); ok {
            t.Protocol = protocol
        }
        if port, ok := inputs["port"].(int); ok {
            t.Port = port
        }
    }

    return t, nil
}

func (t *VerifyCustomProtocol) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
    result := &test.TestResult{
        TestName:   t.Name(),
        DeviceName: dev.Name(),
        Status:     test.TestSuccess,
        Categories: t.Categories(),
    }

    // Execute device command
    cmd := device.Command{
        Template: fmt.Sprintf("show %s status", t.Protocol),
        Format:   "json",
        UseCache: false,
    }

    cmdResult, err := dev.Execute(ctx, cmd)
    if err != nil {
        result.Status = test.TestError
        result.Message = fmt.Sprintf("Failed to execute command: %v", err)
        return result, nil
    }

    // Process command output
    if data, ok := cmdResult.Output.(map[string]any); ok {
        if status, ok := data["status"].(string); ok {
            if status != "enabled" {
                result.Status = test.TestFailure
                result.Message = fmt.Sprintf("Protocol %s is %s, expected enabled", t.Protocol, status)
            }
        } else {
            result.Status = test.TestError
            result.Message = "Unable to parse protocol status"
        }
    }

    return result, nil
}

func (t *VerifyCustomProtocol) ValidateInput(input any) error {
    if t.Protocol == "" {
        return fmt.Errorf("protocol must be specified")
    }
    if t.Port <= 0 {
        return fmt.Errorf("port must be a positive integer")
    }
    return nil
}

// Register the test
func init() {
    test.Register("custom", "VerifyCustomProtocol", NewVerifyCustomProtocol)
}
```

### Netbox Integration Example

```go
package main

import (
    "context"
    "fmt"

    "github.com/gavmckee/go-anta/internal/inventory"
)

func main() {
    // Configure Netbox connection
    config := inventory.NetboxConfig{
        URL:      "https://netbox.example.com",
        Token:    "your-api-token",
        Insecure: false,
    }

    // Define query for leaf switches in DC1
    query := inventory.NetboxQuery{
        Site:   "dc1",
        Role:   "leaf",
        Status: "active",
        Tags:   []string{"production"},
    }

    // Device credentials
    credentials := map[string]interface{}{
        "username": "admin",
        "password": "admin123",
        "insecure": true,
    }

    // Load devices from Netbox
    inv, err := inventory.LoadFromNetbox(config, query, credentials)
    if err != nil {
        fmt.Printf("Error loading from Netbox: %v\n", err)
        return
    }

    fmt.Printf("Loaded %d devices from Netbox\n", len(inv.Devices))
    for _, dev := range inv.Devices {
        fmt.Printf("- %s (%s)\n", dev.Name, dev.Host)
    }
}
```

This comprehensive API documentation provides detailed information about all aspects of the GO-ANTA framework, from basic CLI usage to advanced programmatic integration. The framework's modular design allows for easy extension and customization while maintaining a consistent interface across all components.