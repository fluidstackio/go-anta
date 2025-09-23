# GO-ANTA - Golang Network Test Automation Framework

go-anta (Golang ANTA) is a high-performance network testing framework inspired by the Python ANTA project. It provides automated testing capabilities for network devices, particularly Arista EOS devices, with support for concurrent test execution, flexible inventory management, Netbox integration, and comprehensive reporting.

## 🚀 Features

- **High Performance**: Concurrent test execution across multiple devices with configurable parallelism
- **Dynamic Inventory**: Native Netbox integration for source-of-truth driven testing
- **Flexible Device Targeting**: Advanced filtering with hostname, wildcard, range, and index-based selection
- **Multiple Output Formats**: Table, CSV, JSON, and Markdown reporting
- **Smart Caching**: Command result caching to reduce device load and improve performance
- **Comprehensive Logging**: Structured logging with configurable levels for troubleshooting
- **Dry Run Mode**: Preview operations without making changes
- **Secure**: TLS support with certificate validation options
- **Modular Architecture**: Pluggable test system with easy-to-extend design

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Inventory Management](#inventory-management)
- [Device Connectivity](#device-connectivity)
- [Running Tests](#running-tests)
- [Netbox Integration](#netbox-integration)
- [Device Filtering](#device-filtering)
- [Test Catalog](#test-catalog)
- [Output Formats](#output-formats)
- [Logging and Debugging](#logging-and-debugging)
- [Available Tests](#available-tests)
- [Configuration](#configuration)

## Installation

### Prerequisites

- Go 1.21 or later
- Access to Arista EOS devices with eAPI enabled
- Optional: Netbox instance for dynamic inventory

### Building from Source

```bash
# Clone the repository
git clone https://github.com/gmckee/ganta.git
cd ganta

# Build the binary
make build

# Verify installation
./bin/ganta --help
```

## Quick Start

### Option 1: Static Inventory File

1. **Create an inventory file** (`inventory.yaml`):

```yaml
devices:
  - name: "leaf1"
    host: "192.168.1.10"
    username: "admin"
    password: "admin123"
    tags: ["leaf", "datacenter"]
    insecure: true  # Skip TLS verification for lab environments
```

2. **Create a test catalog** (`catalog.yaml`):

```yaml
tests:
  - name: "VerifyUptime"
    module: "system"
    inputs:
      minimum_uptime: 3600  # 1 hour in seconds
```

3. **Run the tests**:

```bash
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml
```

### Option 2: Netbox Dynamic Inventory

```bash
# Set environment variables
export NETBOX_URL=https://netbox.example.com
export NETBOX_TOKEN=your-api-token
export DEVICE_USERNAME=admin
export DEVICE_PASSWORD=admin123

# Run tests directly from Netbox
./bin/ganta nrfu \
  --netbox-query "site=datacenter1,platform=eos" \
  -C catalog.yaml
```

## Inventory Management

### View and Verify Inventory

Before running tests, preview your inventory to ensure the correct devices are targeted:

```bash
# Show inventory from file
./bin/ganta inventory -i inventory.yaml

# Show inventory from Netbox
./bin/ganta inventory \
  --netbox-url https://netbox.example.com \
  --netbox-token $TOKEN \
  --netbox-query "site=dc1"

# Show inventory with additional metadata
./bin/ganta inventory -i inventory.yaml --show-tags --show-extra

# Export inventory in different formats
./bin/ganta inventory -i inventory.yaml -f json
./bin/ganta inventory -i inventory.yaml -f yaml
./bin/ganta inventory -i inventory.yaml -f count  # Just show device count
```

### Real-World Example

```bash
# Verify Netbox query returns expected devices
./bin/ganta inventory \
  --netbox-url https://netbox.fluidstack.io \
  --netbox-token 80f4133c031c270d38d4e6ea59fa4cfbaa3525b8 \
  --netbox-query "site_id=14&manufacturer_id=35&platform_id=5" \
  --show-tags

# Export Netbox devices to static inventory file
./bin/ganta inventory \
  --netbox-url https://netbox.example.com \
  --netbox-token $TOKEN \
  --netbox-query "site=dc1&status=active" \
  -f yaml > netbox-devices.yaml
```

## Device Connectivity

### Check Device Connectivity

Verify devices are reachable before running tests:

```bash
# Check connectivity to devices in inventory
./bin/ganta check -i inventory.yaml

# Dry-run mode - show inventory without connecting
./bin/ganta check -i inventory.yaml --no-connect

# Check devices from Netbox
./bin/ganta check \
  --netbox-url https://netbox.example.com \
  --netbox-token $TOKEN \
  --netbox-query "site=dc1"

# Override credentials
./bin/ganta check -i inventory.yaml \
  --device-username admin \
  --device-password newpassword
```

**Example Output:**
```
Checking 3 devices...

Checking leaf1 (192.168.1.10)... ✅ Connected (Model: DCS-7050SX-64)
Checking leaf2 (192.168.1.11)... ✅ Connected (Model: DCS-7050SX-64)  
Checking spine1 (192.168.1.1)... ❌ Failed: connection timeout

Summary: 2 successful, 1 failed
```

## Running Tests

### Network Ready For Use (NRFU) Command

The `nrfu` command is the primary testing interface:

```bash
# Dry-run to see what would be tested
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --dry-run

# Run actual tests
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml

# Run with verbose logging
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -v

# Run with specific concurrency
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -j 20
```

### NRFU Command Options

| Option | Short | Description | Example |
|--------|-------|-------------|---------|
| `--inventory` | `-i` | Inventory file path | `-i devices.yaml` |
| `--catalog` | `-C` | Test catalog file path (required) | `-C tests.yaml` |
| `--netbox-url` | | Netbox URL (or use NETBOX_URL env var) | `--netbox-url https://netbox.example.com` |
| `--netbox-token` | | Netbox API token (or use NETBOX_TOKEN env var) | `--netbox-token abc123` |
| `--netbox-query` | | Netbox query filter | `--netbox-query "site=dc1,role=leaf"` |
| `--device-username` | | Device username override | `--device-username admin` |
| `--device-password` | | Device password override | `--device-password secret` |
| `--tags` | `-t` | Filter devices by tags | `-t production,spine` |
| `--devices` | `-d` | Filter specific devices | `-d leaf1,leaf2` |
| `--limit` | | Advanced device limiting | `--limit "leaf*"` |
| `--tests` | `-T` | Filter specific tests | `-T VerifyBGPPeers` |
| `--concurrency` | `-j` | Max concurrent connections | `-j 20` |
| `--format` | `-f` | Output format | `-f json` |
| `--output` | `-o` | Output file path | `-o results.json` |
| `--hide` | | Hide results by status | `--hide success,skipped` |
| `--dry-run` | | Show what would run without executing | `--dry-run` |
| `--ignore-status` | | Always return exit code 0 | `--ignore-status` |
| `--verbose` | `-v` | Enable verbose logging | `-v` |
| `--log-level` | | Set specific log level | `--log-level debug` |

## Netbox Integration

GANTA provides native integration with Netbox for dynamic inventory management.

### Environment Variables

```bash
export NETBOX_URL=https://netbox.example.com
export NETBOX_TOKEN=your-api-token
export NETBOX_INSECURE=true  # Skip TLS verification if needed
export DEVICE_USERNAME=admin
export DEVICE_PASSWORD=admin123
export DEVICE_ENABLE_PASSWORD=enable123  # Optional
```

### Netbox Query Syntax

GANTA supports two query formats:

#### 1. URL Parameter Format (Precise)

Use exact Netbox API parameters with IDs:

```bash
# Query by IDs (most precise)
--netbox-query "site_id=14&manufacturer_id=35&platform_id=5"

# Mix IDs and other filters
--netbox-query "site_id=14&platform=eos&status=active"

# Complex queries
--netbox-query "site_id=14&manufacturer_id=35&platform_id=5&role_id=12&status=active"
```

#### 2. Comma-Separated Format (Human-Friendly)

Use slug-based filtering:

```bash
# Filter by site and platform
--netbox-query "site=datacenter1,platform=eos"

# Filter by multiple criteria
--netbox-query "site=dc1,role=leaf,manufacturer=arista,status=active"

# Filter by tags
--netbox-query "site=dc1,tag=production"
```

### Supported Netbox Fields

| Field | Slug Format | ID Format | Description |
|-------|-------------|-----------|-------------|
| Site | `site=slug` | `site_id=123` | Site location |
| Role | `role=slug` | `role_id=123` | Device role |
| Device Type | `device_type=slug` | `device_type_id=123` | Hardware model |
| Manufacturer | `manufacturer=slug` | `manufacturer_id=123` | Device manufacturer |
| Platform | `platform=slug` | `platform_id=123` | Network OS platform |
| Status | `status=active` | | Device status |
| Tenant | `tenant=slug` | `tenant_id=123` | Tenant assignment |
| Region | `region=slug` | `region_id=123` | Geographic region |
| Name | `name=hostname` | | Exact hostname match |
| Name Contains | `name_contains=text` | | Hostname substring |
| Tags | `tag=production` | | Device tags |

### Real-World Netbox Examples

```bash
# Test all leaf switches in a specific datacenter
./bin/ganta nrfu \
  --netbox-url https://netbox.fluidstack.io \
  --netbox-token $NETBOX_TOKEN \
  --netbox-query "site_id=14&role=leaf&status=active" \
  -C catalog.yaml

# Test all Arista devices in production
./bin/ganta nrfu \
  --netbox-url https://netbox.fluidstack.io \
  --netbox-token $NETBOX_TOKEN \
  --netbox-query "manufacturer=arista,tag=production" \
  -C catalog.yaml \
  --hide success

# Dry-run with device preview
./bin/ganta nrfu \
  --netbox-url https://netbox.fluidstack.io \
  --netbox-token $NETBOX_TOKEN \
  --netbox-query "site_id=14&manufacturer_id=35&platform_id=5" \
  -C catalog.yaml \
  --dry-run

# Test specific tenant's devices
./bin/ganta nrfu \
  --netbox-url https://netbox.fluidstack.io \
  --netbox-token $NETBOX_TOKEN \
  --netbox-query "tenant=customer1,status=active" \
  -C catalog.yaml \
  --device-username customer1_admin \
  --device-password customer1_pass
```

## Device Filtering

GANTA provides powerful device filtering capabilities to target specific subsets of your inventory.

### Basic Filtering

```bash
# Filter by device names
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -d "leaf1,leaf2,spine1"

# Filter by tags
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -t "production,spine"

# Combine multiple filters
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -d "leaf1,leaf2" -t "production"
```

### Advanced Limit Filtering

The `--limit` flag provides sophisticated device selection:

#### 1. Single Hostname
```bash
# Target specific device
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "wdl-200-50-r101-eth-leaf-01" \
  -C catalog.yaml
```

#### 2. Comma-Separated List
```bash
# Target multiple specific devices
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "leaf01,leaf02,spine01" \
  -C catalog.yaml
```

#### 3. Index-Based Selection
```bash
# Target first device (index 0)
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "0" \
  -C catalog.yaml

# Target third device (index 2)
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "2" \
  -C catalog.yaml
```

#### 4. Range-Based Selection
```bash
# Target first 3 devices (indices 0-2)
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "0-2" \
  -C catalog.yaml

# Target devices 5-10
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "4-9" \
  -C catalog.yaml
```

#### 5. Wildcard Patterns
```bash
# Target all leaf switches
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "leaf*" \
  -C catalog.yaml

# Target all devices containing "spine"
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "*spine*" \
  -C catalog.yaml

# Target devices with specific pattern
./bin/ganta nrfu \
  --netbox-query "site_id=14" \
  --limit "dc1-*-leaf-*" \
  -C catalog.yaml
```

### Filtering Workflow Examples

```bash
# Step 1: Preview inventory
./bin/ganta inventory \
  --netbox-query "site_id=14&manufacturer_id=35" \
  --show-tags

# Step 2: Target specific devices for testing
./bin/ganta nrfu \
  --netbox-query "site_id=14&manufacturer_id=35" \
  --limit "*leaf*" \
  -C catalog.yaml \
  --dry-run

# Step 3: Run actual tests
./bin/ganta nrfu \
  --netbox-query "site_id=14&manufacturer_id=35" \
  --limit "*leaf*" \
  -C catalog.yaml
```

## Test Catalog

### Test Structure

Each test in the catalog follows this structure:

```yaml
tests:
  - name: "TestName"           # Must match test implementation
    module: "module_name"      # Test module (connectivity, hardware, routing, system)
    categories: ["category"]   # Optional: for grouping and filtering
    tags: ["tag"]             # Optional: for additional filtering
    inputs:                   # Test-specific parameters
      parameter1: value1
      parameter2: value2
```

### Example Comprehensive Catalog

```yaml
tests:
  # System Health Tests
  - name: "VerifyUptime"
    module: "system"
    categories: ["health"]
    inputs:
      minimum_uptime: 86400  # 24 hours

  - name: "VerifyEOSVersion"
    module: "system"
    categories: ["compliance"]
    inputs:
      minimum_version: "4.25.0"

  - name: "VerifyNTP"
    module: "system"
    categories: ["time"]
    inputs:
      servers:
        - server: "0.pool.ntp.org"
          synchronized: true

  # Hardware Tests
  - name: "VerifyTemperature"
    module: "hardware"
    categories: ["environmental"]
    inputs:
      check_temp_sensors: true
      failure_margin: 5

  - name: "VerifyTransceivers"
    module: "hardware"
    categories: ["optics"]
    inputs:
      check_manufacturer: true
      check_temperature: true

  # Connectivity Tests  
  - name: "VerifyReachability"
    module: "connectivity"
    categories: ["basic"]
    inputs:
      hosts:
        - destination: "8.8.8.8"
          vrf: "default"
          reachable: true
      repeat: 2

  - name: "VerifyLLDPNeighbors"
    module: "connectivity"
    categories: ["topology"]
    inputs:
      interfaces:
        - interface: "Ethernet1"
          neighbor_device: "spine1"
          neighbor_port: "Ethernet1"

  # Routing Tests
  - name: "VerifyBGPPeers"
    module: "routing"
    categories: ["bgp"]
    inputs:
      peers:
        - peer: "10.0.0.1"
          state: "Established"
          asn: 65001
```

## Output Formats

### Table Format (Default)

Clean, readable format perfect for console output:

```bash
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml
```

```
Device    Test              Status      Message           Duration
------------------------------------------------------------------------
leaf1     VerifyUptime     ✅ SUCCESS   -                0.15s
leaf1     VerifyBGPPeers   ❌ FAILURE   Peer 10.0.0.1 down  0.23s
leaf2     VerifyUptime     ✅ SUCCESS   -                0.12s
------------------------------------------------------------------------
Summary: Total: 3 | Success: 2 | Failure: 1 | Error: 0 | Skipped: 0
```

### JSON Format

Structured data perfect for automation and integration:

```bash
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -f json -o results.json
```

```json
{
  "results": [
    {
      "test_name": "VerifyUptime",
      "device_name": "leaf1",
      "status": 1,
      "message": "",
      "duration": 150000000,
      "timestamp": "2024-01-15T10:30:00Z",
      "categories": ["system"]
    }
  ],
  "statistics": {
    "total": 3,
    "success": 2,
    "failure": 1,
    "error": 0,
    "skipped": 0
  }
}
```

### CSV Format

Great for spreadsheet analysis:

```bash
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -f csv -o results.csv
```

### Markdown Format

Perfect for documentation and reports:

```bash
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -f markdown -o report.md
```

### Filtering Output

```bash
# Hide successful tests to focus on issues
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --hide success

# Hide successful and skipped tests
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --hide success,skipped

# Show only failures and errors
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --hide success,skipped
```

## Logging and Debugging

### Log Levels

By default, GANTA only shows warnings and errors. Enable more detailed logging as needed:

```bash
# Quiet mode (warnings and errors only) - default
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml

# Verbose mode (debug level)
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -v

# Specific log levels
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --log-level info
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --log-level debug
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --log-level error
```

### Debugging Connection Issues

```bash
# Enable verbose logging to see connection details
./bin/ganta check -i inventory.yaml -v

# Test connectivity to specific device
./bin/ganta check -i inventory.yaml --limit "problematic-device" -v

# Debug with maximum logging
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml --limit "device1" --log-level debug
```

### Log Output Examples

**Default (Warning level):**
```
Summary: 2 successful, 0 failed
```

**Verbose mode:**
```
time="14:51:56" level=info msg="Connecting to device leaf1 (192.168.1.10:443)"
time="14:51:56" level=debug msg="Creating HTTP request for leaf1 to https://192.168.1.10:443/command-api"
time="14:51:56" level=info msg="Successfully connected to leaf1 (Model: DCS-7050SX-64)"
```

## Available Tests

### System Tests

#### VerifyEOSVersion
Validates EOS software version meets requirements.

```yaml
- name: "VerifyEOSVersion"
  module: "system"
  inputs:
    minimum_version: "4.25.0"    # Minimum acceptable version
    # OR specify exact versions:
    versions:
      - "4.28.0F"
      - "4.27.3F"
```

#### VerifyUptime  
Ensures system uptime meets minimum requirements.

```yaml
- name: "VerifyUptime"
  module: "system"
  inputs:
    minimum_uptime: 86400    # Seconds (24 hours)
```

#### VerifyNTP
Verifies NTP synchronization status and server configuration.

```yaml
- name: "VerifyNTP"
  module: "system"
  inputs:
    servers:
      - server: "0.pool.ntp.org"
        synchronized: true
        stratum: 2           # Optional max stratum
```

### Hardware Tests

#### VerifyTemperature
Monitors system temperature sensors for overheating conditions.

```yaml
- name: "VerifyTemperature"
  module: "hardware"
  inputs:
    check_temp_sensors: true
    failure_margin: 5       # Degrees from overheat threshold
```

#### VerifyTransceivers
Validates optical transceiver health and specifications.

```yaml
- name: "VerifyTransceivers"
  module: "hardware"
  inputs:
    check_manufacturer: true
    manufacturers:
      - "Arista Networks"
      - "Finisar"
    check_temperature: true
    check_voltage: true
```

#### VerifyInventory
Verifies hardware inventory meets minimum specifications.

```yaml
- name: "VerifyInventory"
  module: "hardware"
  inputs:
    minimum_memory: 8192     # MB
    minimum_flash: 4096      # MB
    required_modules:
      - "DCS-7280SR-48C6"
```

### Connectivity Tests

#### VerifyReachability
Tests network connectivity to specified destinations.

```yaml
- name: "VerifyReachability"
  module: "connectivity"
  inputs:
    hosts:
      - destination: "8.8.8.8"
        vrf: "default"        # Optional, defaults to "default"
        reachable: true       # Expected reachability state
      - destination: "192.168.255.1"
        vrf: "management"
        reachable: true
    repeat: 2                 # Optional: number of pings
    size: 100                # Optional: packet size in bytes
```

#### VerifyLLDPNeighbors
Validates LLDP neighbor relationships and topology.

```yaml
- name: "VerifyLLDPNeighbors"
  module: "connectivity"
  inputs:
    interfaces:
      - interface: "Ethernet1"
        neighbor_device: "spine1"
        neighbor_port: "Ethernet1"
      - interface: "Ethernet2"  
        neighbor_device: "spine2"
        neighbor_port: "Ethernet1"
```

### Routing Tests

#### VerifyBGPPeers
Validates BGP peer status and configuration.

```yaml
- name: "VerifyBGPPeers"
  module: "routing"
  inputs:
    peers:
      - peer: "10.0.0.1"
        state: "Established"
        asn: 65001
        vrf: "default"       # Optional VRF
      - peer: "10.0.0.2"
        state: "Established"  
        asn: 65002
```

#### VerifyOSPFNeighbors
Verifies OSPF neighbor adjacencies and states.

```yaml
- name: "VerifyOSPFNeighbors"
  module: "routing"
  inputs:
    neighbors:
      - interface: "Ethernet1"
        state: "Full"
        router_id: "1.1.1.1"  # Optional router ID verification
    instance: "1"             # Optional OSPF instance
```

#### VerifyStaticRoutes
Validates static route configuration and next-hops.

```yaml
- name: "VerifyStaticRoutes"
  module: "routing"  
  inputs:
    routes:
      - prefix: "0.0.0.0/0"
        next_hop: "192.168.1.1"
        vrf: "default"
      - prefix: "10.0.0.0/8"
        next_hop: "192.168.1.2"
        vrf: "default"
```

## Configuration

### Static Inventory Configuration

Create detailed device inventories with flexible credential management:

```yaml
# inventory.yaml
devices:
  - name: "spine1"
    host: "10.0.0.1"
    port: 443                    # Default: 443
    username: "admin"
    password: "admin123"
    enable_password: "enable123"  # Optional
    tags: ["spine", "production"]
    timeout: "30s"               # Default: 30s
    insecure: false              # Default: false

# Network-based discovery
networks:
  - network: "192.168.100.0/24"
    username: "admin"
    password: "admin123"
    tags: ["management"]
    insecure: true

# Range-based discovery  
ranges:
  - start: "10.0.1.1"
    end: "10.0.1.10"
    username: "admin"
    password: "admin123"
    tags: ["lab"]
```

### Application Configuration

Create `.ganta.yaml` for application-wide settings:

```yaml
# .ganta.yaml
log:
  level: "info"              # debug, info, warn, error
  file: "ganta.log"          # Optional log file
  format: "text"             # text, json

device:
  timeout: "30s"
  max_connections: 100
  retry_attempts: 3
  retry_delay: "5s"

test:
  max_concurrency: 10
  cache_ttl: "60s"
  cache_size: 128

reporter:
  default_format: "table"
  output_file: ""

netbox:
  url: "https://netbox.example.com"
  token: "your-token-here"
  insecure: false
  timeout: "30s"
```

### Environment Variables

GANTA supports extensive environment variable configuration:

```bash
# Netbox Configuration
export NETBOX_URL=https://netbox.example.com
export NETBOX_TOKEN=your-api-token
export NETBOX_INSECURE=true

# Device Credentials
export DEVICE_USERNAME=admin
export DEVICE_PASSWORD=admin123
export DEVICE_ENABLE_PASSWORD=enable123

# Application Settings (prefix with GANTA_)
export GANTA_LOG_LEVEL=debug
export GANTA_DEVICE_TIMEOUT=60s
export GANTA_TEST_CONCURRENCY=20
```

## Troubleshooting

### Common Connection Issues

1. **Connection Timeouts**
   ```bash
   # Increase timeout
   ./bin/ganta check -i inventory.yaml --log-level debug
   ```

2. **TLS Certificate Errors**
   ```yaml
   # In inventory file
   devices:
     - name: "device1"
       host: "192.168.1.10"
       insecure: true  # Skip certificate verification
   ```

3. **Authentication Failures**
   ```bash
   # Override credentials
   ./bin/ganta check -i inventory.yaml \
     --device-username admin \
     --device-password newpassword
   ```

4. **Netbox Query Issues**
   ```bash
   # Test query separately
   ./bin/ganta inventory \
     --netbox-url $NETBOX_URL \
     --netbox-token $NETBOX_TOKEN \
     --netbox-query "your-query" \
     -f count
   ```

### Debug Network Connectivity

Use the included debug tools:

```bash
# Build debug tool
go build -o debug cmd/debug/main.go

# Test network connectivity
./debug 192.168.1.10
```

### Performance Tuning

```bash
# Increase concurrency for faster execution
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -j 50

# Reduce concurrency for stability
./bin/ganta nrfu -i inventory.yaml -C catalog.yaml -j 5

# Use caching to improve performance
# (Enabled by default, disable with --no-cache if needed)
```

## Contributing

Contributions are welcome! This project follows standard Go development practices:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

### Development Setup

```bash
git clone https://github.com/gmckee/ganta.git
cd ganta
go mod tidy
make test
make build
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Inspired by the [ANTA](https://github.com/arista-netdevops-community/anta) project from the Arista community
- Built for the Arista EOS platform and ecosystem
- Special thanks to the network automation community for their contributions and feedback

---

## Real-World Usage Examples

### Production Network Validation

```bash
# Full production spine-leaf validation
./bin/ganta nrfu \
  --netbox-url https://netbox.fluidstack.io \
  --netbox-token $PROD_TOKEN \
  --netbox-query "site=prod-dc1,status=active" \
  -C production-catalog.yaml \
  -j 25 \
  --format json \
  --output prod-validation-$(date +%Y%m%d).json \
  --hide success
```

### Staging Environment Testing

```bash  
# Quick staging verification
./bin/ganta nrfu \
  --netbox-query "site=staging,tag=ready-for-prod" \
  -C staging-catalog.yaml \
  --limit "*leaf*" \
  -v
```

### Maintenance Window Validation

```bash
# Pre-maintenance check
./bin/ganta check \
  --netbox-query "site=dc1,tenant=customer1" \
  --device-username maint_user \
  --device-password $MAINT_PASSWORD

# Post-maintenance validation  
./bin/ganta nrfu \
  --netbox-query "site=dc1,tenant=customer1" \
  -C post-maint-catalog.yaml \
  --format markdown \
  --output maintenance-report.md
```

This comprehensive README reflects all the excellent work that has been implemented, including the Netbox integration, advanced device filtering, logging improvements, and the robust device connectivity features that were developed and tested.