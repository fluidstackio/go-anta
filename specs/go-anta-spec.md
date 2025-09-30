# GANTA - Golang Network Test Automation Framework

## Project Overview

GANTA (Golang ANTA) is a network testing framework inspired by the Python ANTA project. It provides automated testing capabilities for network devices, particularly Arista EOS devices, with support for concurrent test execution, flexible inventory management, and comprehensive reporting.

## Core Requirements

### 1. Device Management
- Support for Arista EOS devices via eAPI (HTTP/HTTPS)
- SSH connectivity for file operations
- Connection pooling and concurrent connection management
- Device state tracking (online, established, hardware model)
- Automatic reconnection and retry logic
- Command caching to reduce API calls

### 2. Test Framework
- Pluggable test system with interface-based design
- Command templating with parameter substitution
- Test categorization and tagging
- Input validation using struct tags
- Concurrent test execution across multiple devices
- Test result aggregation and reporting

### 3. Inventory Management
- YAML/JSON inventory file support
- Device filtering by tags, names, or IP ranges
- Network range and host-based device discovery
- Credential management and security
- Inventory validation and error handling

### 4. CLI Interface
- Cobra-based command-line interface
- Multiple subcommands (nrfu, check, get, debug)
- Configuration file support
- Environment variable overrides
- Progress tracking and verbose logging

### 5. Reporting
- Multiple output formats (table, CSV, JSON, markdown)
- Test result filtering and sorting
- Performance metrics and statistics
- Export capabilities for integration

## Technical Architecture

### Project Structure
```
go-anta/
├── cmd/
│   └── go-anta/
│       └── main.go                 # Application entry point
├── internal/
│   ├── device/
│   │   ├── device.go              # Device interface and base types
│   │   ├── eos.go                 # Arista EOS device implementation
│   │   ├── connection.go          # Connection management
│   │   └── cache.go               # Command result caching
│   ├── test/
│   │   ├── test.go                # Test interface and base types
│   │   ├── catalog.go             # Test catalog management
│   │   ├── runner.go              # Test execution engine
│   │   └── registry.go            # Test registration system
│   ├── inventory/
│   │   ├── inventory.go           # Inventory management
│   │   ├── models.go              # Inventory data models
│   │   └── parser.go              # File parsing utilities
│   ├── cli/
│   │   ├── commands/
│   │   │   ├── nrfu.go            # Network Ready For Use command
│   │   │   ├── check.go           # Device check command
│   │   │   ├── get.go             # Data retrieval command
│   │   │   └── debug.go           # Debug utilities
│   │   ├── root.go                # Root command setup
│   │   └── utils.go               # CLI utilities
│   ├── reporter/
│   │   ├── reporter.go            # Reporter interface
│   │   ├── table.go               # Table format reporter
│   │   ├── csv.go                 # CSV format reporter
│   │   ├── json.go                # JSON format reporter
│   │   └── markdown.go            # Markdown format reporter
│   └── config/
│       ├── config.go              # Configuration management
│       └── defaults.go            # Default configuration values
├── tests/
│   ├── connectivity/
│   │   ├── reachability.go        # Network reachability tests
│   │   └── lldp.go                # LLDP neighbor tests
│   ├── hardware/
│   │   ├── temperature.go         # Temperature monitoring
│   │   ├── inventory.go           # Hardware inventory
│   │   └── transceivers.go        # Transceiver validation
│   ├── routing/
│   │   ├── bgp.go                 # BGP configuration tests
│   │   ├── ospf.go                # OSPF configuration tests
│   │   └── static.go              # Static route tests
│   └── system/
│       ├── version.go             # Software version tests
│       └── uptime.go              # System uptime tests
├── examples/
│   ├── inventory.yaml             # Example inventory file
│   ├── catalog.yaml               # Example test catalog
│   └── config.yaml                # Example configuration
├── docs/
│   ├── README.md                  # Project documentation
│   ├── INSTALL.md                 # Installation guide
│   └── USAGE.md                   # Usage examples
├── go.mod                         # Go module definition
├── go.sum                         # Go module checksums
├── Makefile                       # Build automation
└── .gitignore                     # Git ignore rules
```

### Core Interfaces

#### Device Interface
```go
type Device interface {
    Name() string
    Host() string
    Tags() []string
    Connect(ctx context.Context) error
    Disconnect() error
    Execute(ctx context.Context, cmd Command) (*CommandResult, error)
    ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error)
    IsOnline() bool
    IsEstablished() bool
    HardwareModel() string
    Refresh(ctx context.Context) error
}
```

#### Test Interface
```go
type Test interface {
    Name() string
    Description() string
    Categories() []string
    Commands() []Command
    Execute(ctx context.Context, device Device) (*TestResult, error)
    ValidateInput(input interface{}) error
}
```

#### Reporter Interface
```go
type Reporter interface {
    Report(results []TestResult) error
    SetOutput(w io.Writer)
    SetFormat(format string)
}
```

## Data Models

### Device Configuration
```go
type DeviceConfig struct {
    Name            string            `yaml:"name" json:"name"`
    Host            string            `yaml:"host" json:"host"`
    Port            int               `yaml:"port,omitempty" json:"port,omitempty"`
    Username        string            `yaml:"username" json:"username"`
    Password        string            `yaml:"password" json:"password"`
    EnablePassword  string            `yaml:"enable_password,omitempty" json:"enable_password,omitempty"`
    Tags            []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
    Timeout         time.Duration     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
    Insecure        bool              `yaml:"insecure,omitempty" json:"insecure,omitempty"`
    DisableCache    bool              `yaml:"disable_cache,omitempty" json:"disable_cache,omitempty"`
}
```

### Command Structure
```go
type Command struct {
    Template    string                 `yaml:"template" json:"template"`
    Params      map[string]interface{} `yaml:"params,omitempty" json:"params,omitempty"`
    Version     string                 `yaml:"version,omitempty" json:"version,omitempty"`
    Revision    int                    `yaml:"revision,omitempty" json:"revision,omitempty"`
    Format      string                 `yaml:"format,omitempty" json:"format,omitempty"`
    UseCache    bool                   `yaml:"use_cache,omitempty" json:"use_cache,omitempty"`
}
```

### Test Result
```go
type TestResult struct {
    TestName    string        `json:"test_name"`
    DeviceName  string        `json:"device_name"`
    Status      TestStatus    `json:"status"`
    Message     string        `json:"message,omitempty"`
    Duration    time.Duration `json:"duration"`
    Timestamp   time.Time     `json:"timestamp"`
    Categories  []string      `json:"categories"`
    CustomField string        `json:"custom_field,omitempty"`
}
```

### Test Status Enum
```go
type TestStatus int

const (
    TestUnset TestStatus = iota
    TestSuccess
    TestFailure
    TestError
    TestSkipped
)
```

## Configuration Files

### Inventory File Format
```yaml
# inventory.yaml
devices:
  - name: "leaf1"
    host: "192.168.1.10"
    username: "admin"
    password: "password"
    tags: ["leaf", "datacenter"]
  - name: "spine1"
    host: "192.168.1.1"
    username: "admin"
    password: "password"
    tags: ["spine", "datacenter"]

networks:
  - network: "192.168.2.0/24"
    username: "admin"
    password: "password"
    tags: ["management"]

ranges:
  - start: "10.0.1.1"
    end: "10.0.1.10"
    username: "admin"
    password: "password"
    tags: ["test"]
```

### Test Catalog Format
```yaml
# catalog.yaml
tests:
  - name: "VerifyReachability"
    module: "connectivity"
    inputs:
      hosts:
        - destination: "8.8.8.8"
          vrf: "default"
          reachable: true
        - destination: "1.1.1.1"
          vrf: "default"
          reachable: true
      repeat: 2
      size: 100

  - name: "VerifyTemperature"
    module: "hardware"
    inputs:
      check_temp_sensors: true
      failure_margin: 5
```

### Application Configuration
```yaml
# config.yaml
log:
  level: "info"
  file: "go-anta.log"

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
```

## CLI Commands

### Root Command
```bash
go-anta [flags] [command]
```

Global flags:
- `--config, -c`: Configuration file path
- `--log-level, -l`: Log level (debug, info, warn, error)
- `--log-file`: Log file path
- `--verbose, -v`: Verbose output

### NRFU Command (Network Ready For Use)
```bash
go-anta nrfu [flags]
```

Flags:
- `--inventory, -i`: Inventory file path
- `--catalog, -c`: Test catalog file path
- `--tags, -t`: Filter devices by tags (comma-separated)
- `--devices, -d`: Filter specific devices (comma-separated)
- `--tests, -T`: Filter specific tests (comma-separated)
- `--concurrency, -j`: Maximum concurrent connections
- `--dry-run`: Show what would be executed without running
- `--ignore-status`: Always return exit code 0
- `--hide`: Hide results by status (success, failure, error, skipped)
- `--output, -o`: Output file path
- `--format, -f`: Output format (table, csv, json, markdown)

### Check Command
```bash
go-anta check [flags]
```

Flags:
- `--inventory, -i`: Inventory file path
- `--devices, -d`: Specific devices to check
- `--tags, -t`: Filter devices by tags

### Get Command
```bash
go-anta get [flags] <command>
```

Subcommands:
- `inventory`: Get device inventory information
- `version`: Get device software versions
- `interfaces`: Get interface information
- `bgp`: Get BGP neighbor information

### Debug Command
```bash
go-anta debug [flags]
```

Flags:
- `--device, -d`: Device to debug
- `--command, -c`: Command to execute
- `--raw`: Show raw command output

## Implementation Requirements

### 1. Error Handling
- Comprehensive error wrapping with context
- Graceful degradation for network issues
- Detailed logging for troubleshooting
- User-friendly error messages

### 2. Concurrency
- Goroutine-based concurrent execution
- Semaphore-based connection limiting
- Context-based cancellation
- Proper resource cleanup

### 3. Caching
- In-memory command result caching
- TTL-based cache expiration
- Cache statistics and monitoring
- Configurable cache behavior

### 4. Security
- Secure credential handling
- TLS certificate validation
- SSH key management
- Environment variable support for secrets

### 5. Testing
- Unit tests for all components
- Integration tests with mock devices
- Performance benchmarks
- Test coverage reporting

### 6. Documentation
- Comprehensive API documentation
- Usage examples and tutorials
- Configuration reference
- Troubleshooting guide

## Dependencies

### Core Dependencies
```go
require (
    github.com/spf13/cobra v1.7.0
    github.com/spf13/viper v1.16.0
    gopkg.in/yaml.v3 v3.0.1
    github.com/stretchr/testify v1.8.4
)
```

### Optional Dependencies
```go
require (
    github.com/gorilla/websocket v1.5.0  // For WebSocket support
    github.com/prometheus/client_golang v1.16.0  // For metrics
    github.com/gin-gonic/gin v1.9.1  // For web UI
)
```

## Build and Deployment

### Makefile Targets
```makefile
.PHONY: build test clean install

build:
	go build -o bin/go-anta cmd/go-anta/main.go

test:
	go test -v ./...

clean:
	rm -rf bin/

install:
	go install cmd/go-anta/main.go

docker:
	docker build -t go-anta .

release:
	goreleaser release
```

### Docker Support
- Multi-stage Dockerfile for minimal image size
- Alpine Linux base image
- Non-root user execution
- Health check endpoint

## Performance Requirements

### Scalability
- Support for 1000+ concurrent device connections
- Memory usage under 1GB for 1000 devices
- Test execution time under 5 minutes for 100 devices
- Configurable resource limits

### Reliability
- 99.9% uptime for long-running tests
- Automatic retry for transient failures
- Graceful handling of device disconnections
- Comprehensive error recovery

## Future Enhancements

### Phase 2 Features
- Web-based user interface
- REST API for integration
- Real-time test monitoring
- Advanced reporting and analytics

### Phase 3 Features
- Plugin system for custom tests
- Integration with CI/CD pipelines
- Machine learning-based anomaly detection
- Multi-vendor device support

## Success Criteria

1. **Functionality**: All core features working as specified
2. **Performance**: Meets scalability and reliability requirements
3. **Usability**: Intuitive CLI and clear documentation
4. **Maintainability**: Clean code with comprehensive tests
5. **Compatibility**: Works with Arista EOS devices
6. **Extensibility**: Easy to add new tests and device types

## Getting Started

1. Clone the repository
2. Install Go 1.21 or later
3. Run `make build` to build the binary
4. Create inventory and catalog files
5. Run `go-anta nrfu` to execute tests

This specification provides a comprehensive foundation for implementing GANTA, ensuring it meets the requirements for a production-ready network testing framework.
