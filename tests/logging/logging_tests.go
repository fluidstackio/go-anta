package logging

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifySyslogLogging verifies if syslog logging is enabled.
//
// Expected Results:
//   - Success: The test will pass if syslog logging is enabled on the device.
//   - Failure: The test will fail if syslog logging is not enabled.
//   - Error: The test will report an error if logging configuration cannot be retrieved.
//
// Examples:
//   - name: VerifySyslogLogging
//     VerifySyslogLogging:
type VerifySyslogLogging struct {
	test.BaseTest
}

func NewVerifySyslogLogging(inputs map[string]any) (test.Test, error) {
	t := &VerifySyslogLogging{
		BaseTest: test.BaseTest{
			TestName:        "VerifySyslogLogging",
			TestDescription: "Verify if syslog logging is enabled",
			TestCategories:  []string{"logging", "syslog"},
		},
	}

	return t, nil
}

func (t *VerifySyslogLogging) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show logging",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get logging configuration: %v", err)
		return result, nil
	}

	issues := []string{}

	if loggingData, ok := cmdResult.Output.(map[string]any); ok {
		// Check if syslog is enabled
		syslogEnabled := false

		if syslogConfig, ok := loggingData["syslogEnabled"].(bool); ok {
			syslogEnabled = syslogConfig
		} else if syslogConfig, ok := loggingData["syslog"].(map[string]any); ok {
			if enabled, ok := syslogConfig["enabled"].(bool); ok {
				syslogEnabled = enabled
			}
		}

		// Alternative check - look for syslog servers or configuration
		if !syslogEnabled {
			if syslogServers, ok := loggingData["syslogServers"].([]any); ok && len(syslogServers) > 0 {
				syslogEnabled = true
			} else if hosts, ok := loggingData["hosts"].(map[string]any); ok && len(hosts) > 0 {
				syslogEnabled = true
			}
		}

		if !syslogEnabled {
			issues = append(issues, "Syslog logging is not enabled")
		}
	} else {
		issues = append(issues, "Could not parse logging configuration")
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Syslog logging issues: %v", issues)
	}

	return result, nil
}

func (t *VerifySyslogLogging) ValidateInput(input any) error {
	return nil
}

// VerifyLoggingPersistent verifies if logging persistent is enabled and logs are saved in flash.
//
// Expected Results:
//   - Success: The test will pass if persistent logging is enabled and logs are saved to flash.
//   - Failure: The test will fail if persistent logging is not enabled or logs are not saved to flash.
//   - Error: The test will report an error if logging configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyLoggingPersistent
//     VerifyLoggingPersistent:
type VerifyLoggingPersistent struct {
	test.BaseTest
}

func NewVerifyLoggingPersistent(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingPersistent{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingPersistent",
			TestDescription: "Verify if logging persistent is enabled and logs are saved in flash",
			TestCategories:  []string{"logging", "persistent"},
		},
	}

	return t, nil
}

func (t *VerifyLoggingPersistent) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show logging",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get logging configuration: %v", err)
		return result, nil
	}

	issues := []string{}

	if loggingData, ok := cmdResult.Output.(map[string]any); ok {
		persistentEnabled := false

		// Check for persistent logging configuration
		if persistent, ok := loggingData["persistent"].(map[string]any); ok {
			if enabled, ok := persistent["enabled"].(bool); ok {
				persistentEnabled = enabled
			}

			// Check if logs are saved to flash
			if persistentEnabled {
				if location, ok := persistent["location"].(string); ok {
					if location != "flash" && location != "/mnt/flash" {
						issues = append(issues, fmt.Sprintf("Persistent logging location is '%s', expected 'flash'", location))
					}
				}
			}
		} else if persistentConfig, ok := loggingData["persistentLogging"].(bool); ok {
			persistentEnabled = persistentConfig
		}

		if !persistentEnabled {
			issues = append(issues, "Persistent logging is not enabled")
		}
	} else {
		issues = append(issues, "Could not parse logging configuration")
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Persistent logging issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyLoggingPersistent) ValidateInput(input any) error {
	return nil
}

// VerifyLoggingSourceIntf verifies logging source-interface for a specified VRF.
//
// Expected Results:
//   - Success: The test will pass if the logging source interface matches the expected interface for the specified VRF.
//   - Failure: The test will fail if the logging source interface does not match or is not configured.
//   - Error: The test will report an error if logging configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyLoggingSourceIntf with specific VRF
//     VerifyLoggingSourceIntf:
//       interface: "Management1"
//       vrf: "MGMT"
//
//   - name: VerifyLoggingSourceIntf default VRF
//     VerifyLoggingSourceIntf:
//       interface: "Loopback0"
type VerifyLoggingSourceIntf struct {
	test.BaseTest
	Interface string `yaml:"interface" json:"interface"`
	VRF       string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyLoggingSourceIntf(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingSourceIntf{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingSourceIntf",
			TestDescription: "Verify logging source-interface for a specified VRF",
			TestCategories:  []string{"logging", "source-interface"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if intf, ok := inputs["interface"].(string); ok {
			t.Interface = intf
		}
		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifyLoggingSourceIntf) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show logging",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get logging configuration: %v", err)
		return result, nil
	}

	issues := []string{}

	if loggingData, ok := cmdResult.Output.(map[string]any); ok {
		found := false

		// Check source interface configuration
		if sourceIntf, ok := loggingData["sourceInterface"].(map[string]any); ok {
			if vrfData, ok := sourceIntf[t.VRF].(map[string]any); ok {
				if configuredIntf, ok := vrfData["interface"].(string); ok {
					found = true
					if configuredIntf != t.Interface {
						issues = append(issues, fmt.Sprintf("Logging source interface for VRF %s is '%s', expected '%s'",
							t.VRF, configuredIntf, t.Interface))
					}
				}
			}
		}

		// Alternative structure check
		if !found {
			if vrfConfig, ok := loggingData["vrfs"].(map[string]any); ok {
				if vrfData, ok := vrfConfig[t.VRF].(map[string]any); ok {
					if configuredIntf, ok := vrfData["sourceInterface"].(string); ok {
						found = true
						if configuredIntf != t.Interface {
							issues = append(issues, fmt.Sprintf("Logging source interface for VRF %s is '%s', expected '%s'",
								t.VRF, configuredIntf, t.Interface))
						}
					}
				}
			}
		}

		// Check global source interface if VRF is default
		if !found && t.VRF == "default" {
			if globalSrcIntf, ok := loggingData["globalSourceInterface"].(string); ok {
				found = true
				if globalSrcIntf != t.Interface {
					issues = append(issues, fmt.Sprintf("Global logging source interface is '%s', expected '%s'",
						globalSrcIntf, t.Interface))
				}
			}
		}

		if !found {
			issues = append(issues, fmt.Sprintf("Logging source interface not configured for VRF %s", t.VRF))
		}
	} else {
		issues = append(issues, "Could not parse logging configuration")
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Logging source interface issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"interface": t.Interface,
			"vrf":       t.VRF,
		}
	}

	return result, nil
}

func (t *VerifyLoggingSourceIntf) ValidateInput(input any) error {
	if t.Interface == "" {
		return fmt.Errorf("interface must be specified")
	}
	return nil
}

// VerifyLoggingHosts verifies logging hosts (syslog servers) for a specified VRF.
//
// Expected Results:
//   - Success: The test will pass if all specified logging hosts are configured for the specified VRF.
//   - Failure: The test will fail if any specified logging host is not configured.
//   - Error: The test will report an error if logging configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyLoggingHosts with multiple servers
//     VerifyLoggingHosts:
//       hosts:
//         - "10.1.1.1"
//         - "10.1.1.2"
//       vrf: "default"
//
//   - name: VerifyLoggingHosts management VRF
//     VerifyLoggingHosts:
//       hosts:
//         - "192.168.1.100"
//       vrf: "MGMT"
type VerifyLoggingHosts struct {
	test.BaseTest
	Hosts []string `yaml:"hosts" json:"hosts"`
	VRF   string   `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyLoggingHosts(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingHosts{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingHosts",
			TestDescription: "Verify logging hosts (syslog servers) for a specified VRF",
			TestCategories:  []string{"logging", "hosts"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if hosts, ok := inputs["hosts"].([]any); ok {
			for _, h := range hosts {
				if host, ok := h.(string); ok {
					t.Hosts = append(t.Hosts, host)
				}
			}
		}
		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifyLoggingHosts) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show logging",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get logging configuration: %v", err)
		return result, nil
	}

	issues := []string{}
	configuredHosts := make(map[string]bool)

	if loggingData, ok := cmdResult.Output.(map[string]any); ok {
		// Check syslog servers/hosts configuration
		if syslogServers, ok := loggingData["syslogServers"].([]any); ok {
			for _, serverData := range syslogServers {
				if server, ok := serverData.(map[string]any); ok {
					var serverIP, serverVRF string

					if ip, ok := server["ipAddress"].(string); ok {
						serverIP = ip
					} else if host, ok := server["host"].(string); ok {
						serverIP = host
					}

					if vrf, ok := server["vrf"].(string); ok {
						serverVRF = vrf
					} else {
						serverVRF = "default" // Default VRF if not specified
					}

					// Only consider servers in the specified VRF
					if serverVRF == t.VRF && serverIP != "" {
						configuredHosts[serverIP] = true
					}
				}
			}
		}

		// Alternative structure - check hosts
		if hosts, ok := loggingData["hosts"].(map[string]any); ok {
			if vrfHosts, ok := hosts[t.VRF].([]any); ok {
				for _, hostData := range vrfHosts {
					if host, ok := hostData.(map[string]any); ok {
						if ip, ok := host["ipAddress"].(string); ok {
							configuredHosts[ip] = true
						} else if hostAddr, ok := host["host"].(string); ok {
							configuredHosts[hostAddr] = true
						}
					}
				}
			}
		}

		// Global hosts check for default VRF
		if t.VRF == "default" {
			if globalHosts, ok := loggingData["globalHosts"].([]any); ok {
				for _, hostData := range globalHosts {
					if host, ok := hostData.(string); ok {
						configuredHosts[host] = true
					} else if hostMap, ok := hostData.(map[string]any); ok {
						if ip, ok := hostMap["ipAddress"].(string); ok {
							configuredHosts[ip] = true
						}
					}
				}
			}
		}
	}

	// Check each expected host
	for _, expectedHost := range t.Hosts {
		if !configuredHosts[expectedHost] {
			issues = append(issues, fmt.Sprintf("Logging host %s not configured for VRF %s", expectedHost, t.VRF))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Logging hosts issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"expected_hosts":   t.Hosts,
			"configured_hosts": configuredHosts,
			"vrf":              t.VRF,
		}
	}

	return result, nil
}

func (t *VerifyLoggingHosts) ValidateInput(input any) error {
	if len(t.Hosts) == 0 {
		return fmt.Errorf("at least one logging host must be specified")
	}

	for i, host := range t.Hosts {
		// Validate IP address format
		if ip := net.ParseIP(host); ip == nil {
			return fmt.Errorf("host at index %d has invalid IP address: %s", i, host)
		}

		// Ensure it's IPv4 (syslog typically uses IPv4)
		if net.ParseIP(host).To4() == nil {
			return fmt.Errorf("host at index %d must be an IPv4 address: %s", i, host)
		}
	}

	return nil
}

// VerifyLoggingLogsGeneration verifies if logs are generated.
//
// Expected Results:
//   - Success: The test will pass if a test log message is generated and can be found in recent logs.
//   - Failure: The test will fail if the test log message is not found in recent logs.
//   - Error: The test will report an error if log generation or retrieval fails.
//
// Examples:
//   - name: VerifyLoggingLogsGeneration with default severity
//     VerifyLoggingLogsGeneration:
//
//   - name: VerifyLoggingLogsGeneration with specific severity
//     VerifyLoggingLogsGeneration:
//       severity_level: "warning"
type VerifyLoggingLogsGeneration struct {
	test.BaseTest
	SeverityLevel string `yaml:"severity_level,omitempty" json:"severity_level,omitempty"`
}

func NewVerifyLoggingLogsGeneration(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingLogsGeneration{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingLogsGeneration",
			TestDescription: "Verify if logs are generated",
			TestCategories:  []string{"logging", "generation"},
		},
		SeverityLevel: "informational", // Default severity level
	}

	if inputs != nil {
		if severity, ok := inputs["severity_level"].(string); ok {
			t.SeverityLevel = severity
		}
	}

	return t, nil
}

func (t *VerifyLoggingLogsGeneration) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// First, generate a test log message
	testMessage := fmt.Sprintf("ANTA test log message - %s", t.Name())
	logCmd := device.Command{
		Template: fmt.Sprintf("send log level %s message \"%s\"", t.SeverityLevel, testMessage),
		Format:   "json",
		UseCache: false,
	}

	_, err := dev.Execute(ctx, logCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to generate test log message: %v", err)
		return result, nil
	}

	// Check if the log was generated by examining recent logs
	showLogCmd := device.Command{
		Template: "show logging last 10",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, showLogCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to retrieve recent logs: %v", err)
		return result, nil
	}

	issues := []string{}
	logFound := false

	if logData, ok := cmdResult.Output.(map[string]any); ok {
		// Check for log entries
		if logEntries, ok := logData["logs"].([]any); ok {
			for _, logEntry := range logEntries {
				if log, ok := logEntry.(map[string]any); ok {
					var logMessage string
					if message, ok := log["message"].(string); ok {
						logMessage = message
					} else if text, ok := log["text"].(string); ok {
						logMessage = text
					}

					if strings.Contains(logMessage, "ANTA test log message") {
						logFound = true
						break
					}
				}
			}
		}

		// Alternative structure - check messages
		if messages, ok := logData["messages"].([]any); ok {
			for _, messageData := range messages {
				if msg, ok := messageData.(string); ok {
					if strings.Contains(msg, "ANTA test log message") {
						logFound = true
						break
					}
				}
			}
		}
	}

	if !logFound {
		issues = append(issues, fmt.Sprintf("Test log message with severity '%s' was not found in recent logs", t.SeverityLevel))
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Log generation issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"severity_level": t.SeverityLevel,
			"test_message":   testMessage,
		}
	}

	return result, nil
}

func (t *VerifyLoggingLogsGeneration) ValidateInput(input any) error {
	validSeverityLevels := []string{
		"emergency", "alert", "critical", "error", "warning", "notice", "informational", "debug",
	}

	for _, validLevel := range validSeverityLevels {
		if t.SeverityLevel == validLevel {
			return nil
		}
	}

	return fmt.Errorf("invalid severity level '%s', must be one of: %v", t.SeverityLevel, validSeverityLevels)
}

// VerifyLoggingHostname verifies if logs are generated with the device FQDN.
//
// Expected Results:
//   - Success: The test will pass if generated logs contain the device hostname/FQDN.
//   - Failure: The test will fail if generated logs do not contain the device hostname.
//   - Error: The test will report an error if hostname cannot be determined or logs cannot be generated/retrieved.
//
// Examples:
//   - name: VerifyLoggingHostname with default severity
//     VerifyLoggingHostname:
//
//   - name: VerifyLoggingHostname with specific severity
//     VerifyLoggingHostname:
//       severity_level: "notice"
type VerifyLoggingHostname struct {
	test.BaseTest
	SeverityLevel string `yaml:"severity_level,omitempty" json:"severity_level,omitempty"`
}

func NewVerifyLoggingHostname(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingHostname{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingHostname",
			TestDescription: "Verify if logs are generated with the device FQDN",
			TestCategories:  []string{"logging", "hostname"},
		},
		SeverityLevel: "informational",
	}

	if inputs != nil {
		if severity, ok := inputs["severity_level"].(string); ok {
			t.SeverityLevel = severity
		}
	}

	return t, nil
}

func (t *VerifyLoggingHostname) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Get device hostname/FQDN
	hostnameCmd := device.Command{
		Template: "show hostname",
		Format:   "json",
		UseCache: false,
	}

	hostnameResult, err := dev.Execute(ctx, hostnameCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get device hostname: %v", err)
		return result, nil
	}

	var deviceHostname string
	if hostnameData, ok := hostnameResult.Output.(map[string]any); ok {
		if hostname, ok := hostnameData["hostname"].(string); ok {
			deviceHostname = hostname
		} else if fqdn, ok := hostnameData["fqdn"].(string); ok {
			deviceHostname = fqdn
		}
	}

	if deviceHostname == "" {
		result.Status = test.TestError
		result.Message = "Could not determine device hostname"
		return result, nil
	}

	// Generate test log and check for hostname
	testMessage := fmt.Sprintf("ANTA hostname test log - %s", t.Name())
	logCmd := device.Command{
		Template: fmt.Sprintf("send log level %s message \"%s\"", t.SeverityLevel, testMessage),
		Format:   "json",
		UseCache: false,
	}

	_, err = dev.Execute(ctx, logCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to generate test log: %v", err)
		return result, nil
	}

	// Check recent logs for hostname
	showLogCmd := device.Command{
		Template: "show logging last 10",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, showLogCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to retrieve recent logs: %v", err)
		return result, nil
	}

	issues := []string{}
	hostnameFound := false

	if logData, ok := cmdResult.Output.(map[string]any); ok {
		if logEntries, ok := logData["logs"].([]any); ok {
			for _, logEntry := range logEntries {
				if log, ok := logEntry.(map[string]any); ok {
					var logMessage string
					if message, ok := log["message"].(string); ok {
						logMessage = message
					} else if text, ok := log["text"].(string); ok {
						logMessage = text
					}

					if strings.Contains(logMessage, "ANTA hostname test log") &&
						strings.Contains(logMessage, deviceHostname) {
						hostnameFound = true
						break
					}
				}
			}
		}
	}

	if !hostnameFound {
		issues = append(issues, fmt.Sprintf("Logs do not contain device hostname '%s'", deviceHostname))
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Logging hostname issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"device_hostname": deviceHostname,
			"severity_level":  t.SeverityLevel,
		}
	}

	return result, nil
}

func (t *VerifyLoggingHostname) ValidateInput(input any) error {
	validSeverityLevels := []string{
		"emergency", "alert", "critical", "error", "warning", "notice", "informational", "debug",
	}

	for _, validLevel := range validSeverityLevels {
		if t.SeverityLevel == validLevel {
			return nil
		}
	}

	return fmt.Errorf("invalid severity level '%s', must be one of: %v", t.SeverityLevel, validSeverityLevels)
}

// VerifyLoggingTimestamp verifies if logs are generated with appropriate timestamps.
//
// Expected Results:
//   - Success: The test will pass if generated logs contain proper timestamp formatting.
//   - Failure: The test will fail if generated logs do not contain proper timestamps.
//   - Error: The test will report an error if logs cannot be generated or retrieved.
//
// Examples:
//   - name: VerifyLoggingTimestamp with default severity
//     VerifyLoggingTimestamp:
//
//   - name: VerifyLoggingTimestamp with specific severity
//     VerifyLoggingTimestamp:
//       severity_level: "debug"
type VerifyLoggingTimestamp struct {
	test.BaseTest
	SeverityLevel string `yaml:"severity_level,omitempty" json:"severity_level,omitempty"`
}

func NewVerifyLoggingTimestamp(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingTimestamp{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingTimestamp",
			TestDescription: "Verify if logs are generated with the appropriate timestamp",
			TestCategories:  []string{"logging", "timestamp"},
		},
		SeverityLevel: "informational",
	}

	if inputs != nil {
		if severity, ok := inputs["severity_level"].(string); ok {
			t.SeverityLevel = severity
		}
	}

	return t, nil
}

func (t *VerifyLoggingTimestamp) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Generate test log
	testMessage := fmt.Sprintf("ANTA timestamp test log - %s", t.Name())
	logCmd := device.Command{
		Template: fmt.Sprintf("send log level %s message \"%s\"", t.SeverityLevel, testMessage),
		Format:   "json",
		UseCache: false,
	}

	_, err := dev.Execute(ctx, logCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to generate test log: %v", err)
		return result, nil
	}

	// Check recent logs for proper timestamp format
	showLogCmd := device.Command{
		Template: "show logging last 10",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, showLogCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to retrieve recent logs: %v", err)
		return result, nil
	}

	issues := []string{}
	timestampFound := false

	// Common timestamp patterns
	timestampPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`),                    // ISO format: 2024-01-01T12:00:00
		regexp.MustCompile(`^[A-Za-z]{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),             // Syslog format: Jan  1 12:00:00
		regexp.MustCompile(`^\d{2}:\d{2}:\d{2}`),                                      // Time only: 12:00:00
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`),                 // Date time: 2024-01-01 12:00:00
		regexp.MustCompile(`^[A-Za-z]{3}\s+[A-Za-z]{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`), // Full format: Mon Jan  1 12:00:00
	}

	if logData, ok := cmdResult.Output.(map[string]any); ok {
		if logEntries, ok := logData["logs"].([]any); ok {
			for _, logEntry := range logEntries {
				if log, ok := logEntry.(map[string]any); ok {
					var logMessage string
					if message, ok := log["message"].(string); ok {
						logMessage = message
					} else if text, ok := log["text"].(string); ok {
						logMessage = text
					}

					if strings.Contains(logMessage, "ANTA timestamp test log") {
						// Check if log has a proper timestamp
						for _, pattern := range timestampPatterns {
							if pattern.MatchString(logMessage) {
								timestampFound = true
								break
							}
						}

						// Also check for timestamp field in structured logs
						if timestamp, ok := log["timestamp"].(string); ok && timestamp != "" {
							timestampFound = true
						}
						break
					}
				}
			}
		}
	}

	if !timestampFound {
		issues = append(issues, "Logs do not contain proper timestamp format")
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Logging timestamp issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"severity_level": t.SeverityLevel,
		}
	}

	return result, nil
}

func (t *VerifyLoggingTimestamp) ValidateInput(input any) error {
	validSeverityLevels := []string{
		"emergency", "alert", "critical", "error", "warning", "notice", "informational", "debug",
	}

	for _, validLevel := range validSeverityLevels {
		if t.SeverityLevel == validLevel {
			return nil
		}
	}

	return fmt.Errorf("invalid severity level '%s', must be one of: %v", t.SeverityLevel, validSeverityLevels)
}

// VerifyLoggingAccounting verifies if AAA accounting logs are generated.
//
// Expected Results:
//   - Success: The test will pass if AAA accounting is enabled and accounting logs are found.
//   - Failure: The test will fail if AAA accounting is not enabled or accounting logs are not found.
//   - Error: The test will report an error if AAA configuration or logs cannot be retrieved.
//
// Examples:
//   - name: VerifyLoggingAccounting
//     VerifyLoggingAccounting:
type VerifyLoggingAccounting struct {
	test.BaseTest
}

func NewVerifyLoggingAccounting(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingAccounting{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingAccounting",
			TestDescription: "Verify if AAA accounting logs are generated",
			TestCategories:  []string{"logging", "accounting"},
		},
	}

	return t, nil
}

func (t *VerifyLoggingAccounting) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Check AAA accounting configuration first
	aaaCmd := device.Command{
		Template: "show aaa accounting",
		Format:   "json",
		UseCache: false,
	}

	aaaResult, err := dev.Execute(ctx, aaaCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get AAA accounting configuration: %v", err)
		return result, nil
	}

	accountingEnabled := false
	if aaaData, ok := aaaResult.Output.(map[string]any); ok {
		if accounting, ok := aaaData["accounting"].(map[string]any); ok {
			if commands, ok := accounting["commands"].(map[string]any); ok {
				if enabled, ok := commands["enabled"].(bool); ok && enabled {
					accountingEnabled = true
				}
			}
			if exec, ok := accounting["exec"].(map[string]any); ok {
				if enabled, ok := exec["enabled"].(bool); ok && enabled {
					accountingEnabled = true
				}
			}
		}
	}

	if !accountingEnabled {
		result.Status = test.TestFailure
		result.Message = "AAA accounting is not enabled, cannot verify accounting logs"
		return result, nil
	}

	// Check recent logs for accounting entries
	showLogCmd := device.Command{
		Template: "show logging last 50",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, showLogCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to retrieve recent logs: %v", err)
		return result, nil
	}

	issues := []string{}
	accountingLogsFound := false

	if logData, ok := cmdResult.Output.(map[string]any); ok {
		if logEntries, ok := logData["logs"].([]any); ok {
			for _, logEntry := range logEntries {
				if log, ok := logEntry.(map[string]any); ok {
					var logMessage string
					if message, ok := log["message"].(string); ok {
						logMessage = message
					} else if text, ok := log["text"].(string); ok {
						logMessage = text
					}

					// Look for common AAA accounting log patterns
					accountingKeywords := []string{
						"ACCOUNTING",
						"AAA-ACCOUNTING",
						"accounting",
						"cmd=",
						"user=",
						"start",
						"stop",
					}

					for _, keyword := range accountingKeywords {
						if strings.Contains(logMessage, keyword) {
							accountingLogsFound = true
							break
						}
					}

					if accountingLogsFound {
						break
					}
				}
			}
		}
	}

	if !accountingLogsFound {
		issues = append(issues, "No AAA accounting logs found in recent log entries")
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("AAA accounting logging issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyLoggingAccounting) ValidateInput(input any) error {
	return nil
}

// VerifyLoggingErrors verifies there are no syslog messages with a severity of ERRORS or higher.
//
// Expected Results:
//   - Success: The test will pass if no error-level or higher severity log messages are found in the specified time window.
//   - Failure: The test will fail if error-level or higher severity log messages are found.
//   - Error: The test will report an error if recent logs cannot be retrieved.
//
// Examples:
//   - name: VerifyLoggingErrors with default time window
//     VerifyLoggingErrors:
//
//   - name: VerifyLoggingErrors with custom time window
//     VerifyLoggingErrors:
//       last_number_time_units: 2
//       time_unit: "hours"
//
//   - name: VerifyLoggingErrors check last day
//     VerifyLoggingErrors:
//       last_number_time_units: 1
//       time_unit: "day"
type VerifyLoggingErrors struct {
	test.BaseTest
	LastNumberTimeUnits int    `yaml:"last_number_time_units" json:"last_number_time_units"`
	TimeUnit            string `yaml:"time_unit" json:"time_unit"`
}

func NewVerifyLoggingErrors(inputs map[string]any) (test.Test, error) {
	t := &VerifyLoggingErrors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLoggingErrors",
			TestDescription: "Verify there are no syslog messages with a severity of ERRORS or higher",
			TestCategories:  []string{"logging", "errors"},
		},
		LastNumberTimeUnits: 1,      // Default: last 1 hour
		TimeUnit:            "hour", // Default time unit
	}

	if inputs != nil {
		if timeUnits, ok := inputs["last_number_time_units"].(float64); ok {
			t.LastNumberTimeUnits = int(timeUnits)
		} else if timeUnits, ok := inputs["last_number_time_units"].(int); ok {
			t.LastNumberTimeUnits = timeUnits
		}

		if timeUnit, ok := inputs["time_unit"].(string); ok {
			t.TimeUnit = timeUnit
		}
	}

	return t, nil
}

func (t *VerifyLoggingErrors) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Calculate the time range to check
	var logCount int
	switch strings.ToLower(t.TimeUnit) {
	case "second", "seconds":
		logCount = t.LastNumberTimeUnits * 10 // Rough estimate: 10 logs per second
	case "minute", "minutes":
		logCount = t.LastNumberTimeUnits * 60 // Rough estimate: 60 logs per minute
	case "hour", "hours":
		logCount = t.LastNumberTimeUnits * 100 // Rough estimate: 100 logs per hour
	case "day", "days":
		logCount = t.LastNumberTimeUnits * 1000 // Rough estimate: 1000 logs per day
	default:
		logCount = 100 // Default fallback
	}

	// Cap the log count to a reasonable maximum
	if logCount > 1000 {
		logCount = 1000
	}

	showLogCmd := device.Command{
		Template: fmt.Sprintf("show logging last %d", logCount),
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, showLogCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to retrieve recent logs: %v", err)
		return result, nil
	}

	issues := []string{}
	errorCount := 0
	criticalCount := 0
	alertCount := 0
	emergencyCount := 0

	// Calculate cutoff time
	var cutoffTime time.Time
	now := time.Now()
	switch strings.ToLower(t.TimeUnit) {
	case "second", "seconds":
		cutoffTime = now.Add(-time.Duration(t.LastNumberTimeUnits) * time.Second)
	case "minute", "minutes":
		cutoffTime = now.Add(-time.Duration(t.LastNumberTimeUnits) * time.Minute)
	case "hour", "hours":
		cutoffTime = now.Add(-time.Duration(t.LastNumberTimeUnits) * time.Hour)
	case "day", "days":
		cutoffTime = now.Add(-time.Duration(t.LastNumberTimeUnits) * 24 * time.Hour)
	default:
		cutoffTime = now.Add(-1 * time.Hour) // Default: 1 hour
	}

	if logData, ok := cmdResult.Output.(map[string]any); ok {
		if logEntries, ok := logData["logs"].([]any); ok {
			for _, logEntry := range logEntries {
				if log, ok := logEntry.(map[string]any); ok {
					// Check timestamp if available
					if timestampStr, ok := log["timestamp"].(string); ok {
						if logTime, err := time.Parse(time.RFC3339, timestampStr); err == nil {
							if logTime.Before(cutoffTime) {
								continue // Skip logs outside our time window
							}
						}
					}

					var logMessage, severityStr string
					if message, ok := log["message"].(string); ok {
						logMessage = message
					} else if text, ok := log["text"].(string); ok {
						logMessage = text
					}

					if severity, ok := log["severity"].(string); ok {
						severityStr = severity
					} else {
						// Try to extract severity from message
						severityStr = extractSeverityFromMessage(logMessage)
					}

					// Check for error-level messages
					severityLevel := parseSeverityLevel(severityStr)
					if severityLevel <= 3 { // Error level or higher (0=emergency, 1=alert, 2=critical, 3=error)
						switch severityLevel {
						case 0:
							emergencyCount++
						case 1:
							alertCount++
						case 2:
							criticalCount++
						case 3:
							errorCount++
						}
					}
				}
			}
		}
	}

	// Report findings
	totalErrorLevelLogs := emergencyCount + alertCount + criticalCount + errorCount

	if totalErrorLevelLogs > 0 {
		issues = append(issues, fmt.Sprintf("Found %d error-level or higher log messages in the last %d %s(s)",
			totalErrorLevelLogs, t.LastNumberTimeUnits, t.TimeUnit))

		if emergencyCount > 0 {
			issues = append(issues, fmt.Sprintf("Emergency messages: %d", emergencyCount))
		}
		if alertCount > 0 {
			issues = append(issues, fmt.Sprintf("Alert messages: %d", alertCount))
		}
		if criticalCount > 0 {
			issues = append(issues, fmt.Sprintf("Critical messages: %d", criticalCount))
		}
		if errorCount > 0 {
			issues = append(issues, fmt.Sprintf("Error messages: %d", errorCount))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Logging errors found: %v", issues)
	} else {
		result.Details = map[string]any{
			"time_range":             fmt.Sprintf("%d %s(s)", t.LastNumberTimeUnits, t.TimeUnit),
			"total_logs_checked":     logCount,
			"error_level_logs_found": totalErrorLevelLogs,
		}
	}

	return result, nil
}

func (t *VerifyLoggingErrors) ValidateInput(input any) error {
	if t.LastNumberTimeUnits <= 0 {
		return fmt.Errorf("last_number_time_units must be greater than 0")
	}

	validTimeUnits := []string{"second", "seconds", "minute", "minutes", "hour", "hours", "day", "days"}
	timeUnit := strings.ToLower(t.TimeUnit)
	for _, validUnit := range validTimeUnits {
		if timeUnit == validUnit {
			return nil
		}
	}

	return fmt.Errorf("invalid time_unit '%s', must be one of: %v", t.TimeUnit, validTimeUnits)
}

// Helper function to parse severity level from string to numeric value
func parseSeverityLevel(severity string) int {
	severity = strings.ToLower(severity)
	switch severity {
	case "emergency", "emerg":
		return 0
	case "alert":
		return 1
	case "critical", "crit":
		return 2
	case "error", "err":
		return 3
	case "warning", "warn":
		return 4
	case "notice":
		return 5
	case "informational", "info":
		return 6
	case "debug":
		return 7
	default:
		return 7 // Default to debug level if unknown
	}
}

// Helper function to extract severity from log message
func extractSeverityFromMessage(message string) string {
	message = strings.ToUpper(message)
	severityKeywords := []string{"EMERGENCY", "ALERT", "CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}

	for _, severity := range severityKeywords {
		if strings.Contains(message, severity) {
			return severity
		}
	}

	// Try to find numeric severity in message
	if strings.Contains(message, "%") {
		parts := strings.Split(message, "%")
		if len(parts) > 0 {
			// Look for pattern like "%DAEMON-3-ERROR"
			for _, part := range parts {
				if strings.Contains(part, "-") {
					subParts := strings.Split(part, "-")
					if len(subParts) >= 2 {
						if level, err := strconv.Atoi(subParts[len(subParts)-2]); err == nil {
							if level >= 0 && level <= 7 {
								severityNames := []string{"EMERGENCY", "ALERT", "CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}
								return severityNames[level]
							}
						}
					}
				}
			}
		}
	}

	return "INFO" // Default to info level
}