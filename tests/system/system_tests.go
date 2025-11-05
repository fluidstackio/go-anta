package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyEOSVersion verifies the EOS software version meets requirements.
//
// Expected Results:
//   - Success: The test will pass if the device EOS version matches one of the provided versions,
//     or if the version is greater than or equal to the provided minimum version.
//   - Failure: The test will fail if the device version is not in the provided list or is below
//     the minimum required version.
//   - Error: The test will report an error if the device version cannot be determined.
//
// Examples:
//   - name: VerifyEOSVersion with allowed versions
//     VerifyEOSVersion:
//       versions:
//         - "4.28.1F"
//         - "4.28.2F"
//
//   - name: VerifyEOSVersion with minimum version
//     VerifyEOSVersion:
//       minimum_version: "4.28.0F"
type VerifyEOSVersion struct {
	test.BaseTest
	MinimumVersion string   `yaml:"minimum_version,omitempty" json:"minimum_version,omitempty"`
	Versions       []string `yaml:"versions,omitempty" json:"versions,omitempty"`
}

func NewVerifyEOSVersion(inputs map[string]any) (test.Test, error) {
	t := &VerifyEOSVersion{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEOSVersion",
			TestDescription: "Verify EOS software version meets requirements",
			TestCategories:  []string{"system", "software"},
		},
	}

	if inputs != nil {
		if minVer, ok := inputs["minimum_version"].(string); ok {
			t.MinimumVersion = minVer
		}
		if versions, ok := inputs["versions"].([]any); ok {
			for _, v := range versions {
				if version, ok := v.(string); ok {
					t.Versions = append(t.Versions, version)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyEOSVersion) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show version",
		Format:   "json",
		UseCache: true,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get version information: %v", err)
		return result, nil
	}

	currentVersion := ""
	if versionData, ok := cmdResult.Output.(map[string]any); ok {
		if version, ok := versionData["version"].(string); ok {
			currentVersion = version
		}
	}

	if currentVersion == "" {
		result.Status = test.TestError
		result.Message = "Could not determine current EOS version"
		return result, nil
	}

	if len(t.Versions) > 0 {
		found := false
		for _, allowedVersion := range t.Versions {
			if currentVersion == allowedVersion {
				found = true
				break
			}
		}
		if !found {
			result.Status = test.TestFailure
			result.Message = fmt.Sprintf("Version %s not in allowed list: %v", currentVersion, t.Versions)
			return result, nil
		}
	}

	if t.MinimumVersion != "" {
		if !isVersionGreaterOrEqual(currentVersion, t.MinimumVersion) {
			result.Status = test.TestFailure
			result.Message = fmt.Sprintf("Version %s is below minimum required version %s",
				currentVersion, t.MinimumVersion)
			return result, nil
		}
	}

	result.Details = map[string]string{
		"current_version": currentVersion,
	}

	return result, nil
}

func (t *VerifyEOSVersion) ValidateInput(input any) error {
	if t.MinimumVersion == "" && len(t.Versions) == 0 {
		return fmt.Errorf("either minimum_version or versions list must be specified")
	}
	return nil
}

func isVersionGreaterOrEqual(current, minimum string) bool {
	currentParts := parseVersion(current)
	minimumParts := parseVersion(minimum)

	for i := 0; i < len(minimumParts); i++ {
		if i >= len(currentParts) {
			return false
		}
		if currentParts[i] > minimumParts[i] {
			return true
		}
		if currentParts[i] < minimumParts[i] {
			return false
		}
	}
	return true
}

func parseVersion(version string) []int {
	version = strings.TrimSuffix(version, "F")
	version = strings.TrimSuffix(version, "M")

	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))

	for _, part := range parts {
		var num int
		fmt.Sscanf(part, "%d", &num)
		result = append(result, num)
	}

	return result
}

// VerifyUptime verifies the device uptime meets minimum requirements.
//
// Expected Results:
//   - Success: The test will pass if the device uptime is higher than the provided minimum value.
//   - Failure: The test will fail if the device uptime is lower than the provided minimum value.
//   - Error: The test will report an error if the device uptime cannot be determined.
//
// Examples:
//   - name: VerifyUptime with 1 day minimum
//     VerifyUptime:
//       minimum_uptime: 86400  # 24 hours in seconds
//
//   - name: VerifyUptime with 1 hour minimum
//     VerifyUptime:
//       minimum_uptime: 3600   # 1 hour in seconds (default)
type VerifyUptime struct {
	test.BaseTest
	MinimumUptime int64 `yaml:"minimum_uptime" json:"minimum_uptime"`
}

func NewVerifyUptime(inputs map[string]any) (test.Test, error) {
	t := &VerifyUptime{
		BaseTest: test.BaseTest{
			TestName:        "VerifyUptime",
			TestDescription: "Verify system uptime meets minimum requirements",
			TestCategories:  []string{"system", "availability"},
		},
		MinimumUptime: 3600,
	}

	if inputs != nil {
		if uptime, ok := inputs["minimum_uptime"].(float64); ok {
			t.MinimumUptime = int64(uptime)
		} else if uptime, ok := inputs["minimum_uptime"].(int); ok {
			t.MinimumUptime = int64(uptime)
		}
	}

	return t, nil
}

func (t *VerifyUptime) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show version",
		Format:   "json",
		UseCache: true,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get uptime information: %v", err)
		return result, nil
	}

	var uptimeSeconds int64
	if versionData, ok := cmdResult.Output.(map[string]any); ok {
		if uptime, ok := versionData["uptime"].(float64); ok {
			uptimeSeconds = int64(uptime)
		}
	}

	if uptimeSeconds == 0 {
		result.Status = test.TestError
		result.Message = "Could not determine system uptime"
		return result, nil
	}

	uptimeHours := uptimeSeconds / 3600
	uptimeDays := uptimeHours / 24

	if uptimeSeconds < t.MinimumUptime {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Uptime %d seconds (%.1f hours) is below minimum %d seconds",
			uptimeSeconds, float64(uptimeSeconds)/3600, t.MinimumUptime)
	} else {
		result.Details = map[string]any{
			"uptime_seconds": uptimeSeconds,
			"uptime_hours":   uptimeHours,
			"uptime_days":    uptimeDays,
		}
	}

	return result, nil
}

func (t *VerifyUptime) ValidateInput(input any) error {
	if t.MinimumUptime < 0 {
		return fmt.Errorf("minimum uptime cannot be negative")
	}
	return nil
}

// VerifyNTP verifies the NTP synchronization status and server configuration.
//
// Expected Results:
//   - Success: The test will pass if all specified NTP servers are configured and synchronized
//     according to the expected synchronization state and stratum level.
//   - Failure: The test will fail if any NTP server is missing, has incorrect synchronization state,
//     or has a different stratum level than expected.
//   - Error: The test will report an error if NTP associations cannot be retrieved.
//
// Examples:
//   - name: VerifyNTP with specific servers
//     VerifyNTP:
//       servers:
//         - server: "pool.ntp.org"
//           synchronized: true
//           stratum: 2
//         - server: "time.google.com"
//           synchronized: true
type VerifyNTP struct {
	test.BaseTest
	Servers []NTPServer `yaml:"servers" json:"servers"`
}

type NTPServer struct {
	Server       string `yaml:"server" json:"server"`
	Synchronized bool   `yaml:"synchronized" json:"synchronized"`
	Stratum      int    `yaml:"stratum,omitempty" json:"stratum,omitempty"`
}

type NTPAssociation struct {
	PeerAddress string
	Condition   string
	Stratum     int
}

func NewVerifyNTP(inputs map[string]any) (test.Test, error) {
	t := &VerifyNTP{
		BaseTest: test.BaseTest{
			TestName:        "VerifyNTP",
			TestDescription: "Verify NTP synchronization status",
			TestCategories:  []string{"system", "time"},
		},
	}

	if inputs != nil {
		if servers, ok := inputs["servers"].([]any); ok {
			for _, s := range servers {
				if serverMap, ok := s.(map[string]any); ok {
					server := NTPServer{
						Synchronized: true,
					}
					if addr, ok := serverMap["server"].(string); ok {
						server.Server = addr
					}
					if sync, ok := serverMap["synchronized"].(bool); ok {
						server.Synchronized = sync
					}
					if stratum, ok := serverMap["stratum"].(float64); ok {
						server.Stratum = int(stratum)
					} else if stratum, ok := serverMap["stratum"].(int); ok {
						server.Stratum = stratum
					}
					t.Servers = append(t.Servers, server)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyNTP) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show ntp associations",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get NTP associations: %v", err)
		return result, nil
	}

	issues := []string{}
	ntpServers := make(map[string]NTPAssociation)

	if ntpData, ok := cmdResult.Output.(map[string]any); ok {
		if peers, ok := ntpData["peers"].(map[string]any); ok {
			// EOS structure: peers is a map with server names as keys
			for serverName, peerData := range peers {
				if peer, ok := peerData.(map[string]any); ok {
					assoc := NTPAssociation{}
					assoc.PeerAddress = serverName // Use the key as the server name

					if condition, ok := peer["condition"].(string); ok {
						assoc.Condition = condition
					}
					if stratum, ok := peer["stratumLevel"].(float64); ok {
						assoc.Stratum = int(stratum)
					} else if stratum, ok := peer["stratumLevel"].(int); ok {
						assoc.Stratum = stratum
					}

					ntpServers[serverName] = assoc
				}
			}
		}
	}

	if len(t.Servers) == 0 && len(ntpServers) == 0 {
		result.Status = test.TestFailure
		result.Message = "No NTP servers configured"
		return result, nil
	}

	for _, expectedServer := range t.Servers {
		found := false
		for addr, assoc := range ntpServers {
			if strings.Contains(addr, expectedServer.Server) || strings.Contains(expectedServer.Server, addr) {
				found = true

				isSynchronized := strings.Contains(assoc.Condition, "sys.peer") ||
					strings.Contains(assoc.Condition, "candidate")

				if expectedServer.Synchronized && !isSynchronized {
					issues = append(issues, fmt.Sprintf("Server %s is not synchronized (condition: %s)",
						expectedServer.Server, assoc.Condition))
				} else if !expectedServer.Synchronized && isSynchronized {
					issues = append(issues, fmt.Sprintf("Server %s is unexpectedly synchronized",
						expectedServer.Server))
				}

				if expectedServer.Stratum > 0 && assoc.Stratum != expectedServer.Stratum {
					issues = append(issues, fmt.Sprintf("Server %s: expected stratum %d, got %d",
						expectedServer.Server, expectedServer.Stratum, assoc.Stratum))
				}

				break
			}
		}

		if !found {
			issues = append(issues, fmt.Sprintf("NTP server %s not found", expectedServer.Server))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("NTP issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyNTP) ValidateInput(input any) error {
	for i, server := range t.Servers {
		if server.Server == "" {
			return fmt.Errorf("NTP server at index %d has no address", i)
		}
	}
	return nil
}

// VerifyDNSResolution verifies that DNS name resolution works correctly for specified FQDNs and servers.
//
// Expected Results:
//   - Success: The test will pass if all specified DNS servers are configured and all FQDNs
//     can be resolved successfully.
//   - Failure: The test will fail if any DNS server is not configured or any FQDN fails to resolve.
//   - Error: The test will report an error if DNS configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyDNSResolution with servers and FQDNs
//     VerifyDNSResolution:
//       servers:
//         - "8.8.8.8"
//         - "8.8.4.4"
//       fqdn:
//         - "www.google.com"
//         - "www.arista.com"
//
//   - name: VerifyDNSResolution with FQDNs only
//     VerifyDNSResolution:
//       fqdn:
//         - "github.com"
type VerifyDNSResolution struct {
	test.BaseTest
	Servers []string `yaml:"servers" json:"servers"`
	FQDN    []string `yaml:"fqdn" json:"fqdn"`
}

func NewVerifyDNSResolution(inputs map[string]any) (test.Test, error) {
	t := &VerifyDNSResolution{
		BaseTest: test.BaseTest{
			TestName:        "VerifyDNSResolution",
			TestDescription: "Verify DNS name resolution works correctly",
			TestCategories:  []string{"system", "network"},
		},
	}

	if inputs != nil {
		if servers, ok := inputs["servers"].([]any); ok {
			for _, s := range servers {
				if server, ok := s.(string); ok {
					t.Servers = append(t.Servers, server)
				}
			}
		}

		if fqdns, ok := inputs["fqdn"].([]any); ok {
			for _, f := range fqdns {
				if fqdn, ok := f.(string); ok {
					t.FQDN = append(t.FQDN, fqdn)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyDNSResolution) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}

	// Check DNS servers configuration
	if len(t.Servers) > 0 {
		cmd := device.Command{
			Template: "show ip name-server",
			Format:   "json",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			result.Status = test.TestError
			result.Message = fmt.Sprintf("Failed to get DNS servers: %v", err)
			return result, nil
		}

		configuredServers := make(map[string]bool)
		if dnsData, ok := cmdResult.Output.(map[string]any); ok {
			if servers, ok := dnsData["nameServerConfigs"].([]any); ok {
				for _, s := range servers {
					if serverData, ok := s.(map[string]any); ok {
						if addr, ok := serverData["ipAddr"].(string); ok {
							configuredServers[addr] = true
						}
					}
				}
			}
		}

		// Check if all expected servers are configured
		for _, expectedServer := range t.Servers {
			if !configuredServers[expectedServer] {
				issues = append(issues, fmt.Sprintf("DNS server %s is not configured", expectedServer))
			}
		}
	}

	// Test DNS resolution for each FQDN using ping (which uses DNS resolution)
	for _, fqdn := range t.FQDN {
		cmd := device.Command{
			Template: fmt.Sprintf("ping %s repeat 1", fqdn),
			Format:   "json",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to resolve FQDN %s: %v", fqdn, err))
			continue
		}

		// Check if the FQDN was resolved (if ping output shows successful resolution, DNS worked)
		if pingResult, ok := cmdResult.Output.(map[string]any); ok {
			if messages, ok := pingResult["messages"].([]any); ok && len(messages) > 0 {
				if message, ok := messages[0].(string); ok {
					// If we see "PING" followed by any successful resolution, DNS worked
					// The FQDN might resolve to a CNAME, so we look for successful ping attempts
					if strings.Contains(message, "PING ") && (strings.Contains(message, " bytes from ") || strings.Contains(message, "1 received")) {
						// DNS resolution successful - we got an IP and could ping it
						continue
					} else if strings.Contains(message, "PING ") && strings.Contains(message, "Destination Host Unreachable") {
						// DNS resolved (we got the PING line) but host is unreachable - that's still DNS success
						continue
					} else {
						issues = append(issues, fmt.Sprintf("Failed to resolve FQDN %s via DNS", fqdn))
					}
				}
			} else {
				issues = append(issues, fmt.Sprintf("No response when trying to resolve FQDN %s", fqdn))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("DNS resolution issues: %v", issues)
	} else if len(t.Servers) == 0 && len(t.FQDN) == 0 {
		result.Status = test.TestFailure
		result.Message = "No DNS servers or FQDNs specified for testing"
	}

	return result, nil
}

func (t *VerifyDNSResolution) ValidateInput(input any) error {
	if len(t.Servers) == 0 && len(t.FQDN) == 0 {
		return fmt.Errorf("at least one DNS server or FQDN must be specified")
	}
	return nil
}

// VerifyReloadCause verifies the last reload cause of the device.
//
// Expected Results:
//   - Success: The test will pass if there is no reload cause, or if the last reload cause
//     matches one of the provided allowed causes.
//   - Failure: The test will fail if the last reload cause was NOT one of the provided allowed causes.
//   - Error: The test will report an error if the reload cause cannot be determined.
//
// Examples:
//   - name: VerifyReloadCause with allowed causes
//     VerifyReloadCause:
//       allowed_causes:
//         - "USER"
//         - "FPGA"
//         - "ZTP"
//
//   - name: VerifyReloadCause with default causes
//     VerifyReloadCause:  # Uses default allowed causes: USER, FPGA
type VerifyReloadCause struct {
	test.BaseTest
	AllowedCauses []string `yaml:"allowed_causes,omitempty" json:"allowed_causes,omitempty"`
}

func NewVerifyReloadCause(inputs map[string]any) (test.Test, error) {
	t := &VerifyReloadCause{
		BaseTest: test.BaseTest{
			TestName:        "VerifyReloadCause",
			TestDescription: "Verify the last reload cause of the device",
			TestCategories:  []string{"system", "reload"},
		},
		AllowedCauses: []string{"USER", "FPGA"}, // Default allowed causes
	}

	if inputs != nil {
		if causes, ok := inputs["allowed_causes"].([]any); ok {
			t.AllowedCauses = nil // Clear defaults
			for _, c := range causes {
				if cause, ok := c.(string); ok {
					t.AllowedCauses = append(t.AllowedCauses, strings.ToUpper(cause))
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyReloadCause) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show reload cause",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get reload cause: %v", err)
		return result, nil
	}

	var reloadCause string
	if reloadData, ok := cmdResult.Output.(map[string]any); ok {
		if kernelCrash, ok := reloadData["kernelCrashData"].(map[string]any); ok {
			if description, ok := kernelCrash["description"].(string); ok {
				reloadCause = description
			}
		}

		// If no kernel crash data, check for other reload cause fields
		if reloadCause == "" {
			if resetCauses, ok := reloadData["resetCauses"].([]any); ok && len(resetCauses) > 0 {
				if resetCause, ok := resetCauses[0].(map[string]any); ok {
					if description, ok := resetCause["description"].(string); ok {
						reloadCause = description
					} else if reason, ok := resetCause["reason"].(string); ok {
						reloadCause = reason
					}
				}
			}
		}
	}

	if reloadCause == "" {
		result.Status = test.TestError
		result.Message = "Could not determine reload cause"
		return result, nil
	}

	// Check if the reload cause is allowed
	reloadCauseUpper := strings.ToUpper(reloadCause)
	allowed := false
	for _, allowedCause := range t.AllowedCauses {
		if strings.Contains(reloadCauseUpper, strings.ToUpper(allowedCause)) {
			allowed = true
			break
		}
	}

	if !allowed {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Reload cause '%s' is not in allowed list: %v",
			reloadCause, t.AllowedCauses)
	} else {
		result.Details = map[string]any{
			"reload_cause": reloadCause,
		}
	}

	return result, nil
}

func (t *VerifyReloadCause) ValidateInput(input any) error {
	if len(t.AllowedCauses) == 0 {
		return fmt.Errorf("at least one allowed reload cause must be specified")
	}
	return nil
}

// VerifyCoredump verifies there are no core dump files on the device.
//
// Expected Results:
//   - Success: The test will pass if there are no core dump files in the system.
//   - Failure: The test will fail if core dump files are found.
//   - Error: The test will report an error if core dump information cannot be retrieved.
//
// Notes:
//   - This test will NOT check for minidump files generated by certain agents, as these
//     are expected diagnostic files.
//   - Both application core dumps and kernel core dumps are checked.
//
// Examples:
//   - name: VerifyCoredump
//     VerifyCoredump:
type VerifyCoredump struct {
	test.BaseTest
}

func NewVerifyCoredump(inputs map[string]any) (test.Test, error) {
	t := &VerifyCoredump{
		BaseTest: test.BaseTest{
			TestName:        "VerifyCoredump",
			TestDescription: "Verify there are no core dump files",
			TestCategories:  []string{"system", "stability"},
		},
	}

	return t, nil
}

func (t *VerifyCoredump) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show system coredump",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get coredump information: %v", err)
		return result, nil
	}

	issues := []string{}

	if coredumpData, ok := cmdResult.Output.(map[string]any); ok {
		if coreFiles, ok := coredumpData["coreFiles"].([]any); ok {
			for _, coreFile := range coreFiles {
				if fileInfo, ok := coreFile.(map[string]any); ok {
					if filename, ok := fileInfo["filename"].(string); ok {
						// Skip minidumps as they are expected diagnostic files
						if !strings.Contains(filename, "minidump") {
							issues = append(issues, fmt.Sprintf("Core dump file found: %s", filename))
						}
					}
				}
			}
		}

		// Also check for kernel core dumps
		if kernelCores, ok := coredumpData["kernelCoreFiles"].([]any); ok {
			for _, coreFile := range kernelCores {
				if fileInfo, ok := coreFile.(map[string]any); ok {
					if filename, ok := fileInfo["filename"].(string); ok {
						issues = append(issues, fmt.Sprintf("Kernel core dump file found: %s", filename))
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Core dump issues found: %v", issues)
	}

	return result, nil
}

func (t *VerifyCoredump) ValidateInput(input any) error {
	return nil
}

// VerifyAgentLogs verifies there are no agent crash reports on the device.
//
// Expected Results:
//   - Success: The test will pass if there are no agent crash logs or crashed agents.
//   - Failure: The test will fail if agent crashes are detected or agents are in crashed/restarting state.
//   - Error: The test will report an error if agent logs cannot be retrieved.
//
// Examples:
//   - name: VerifyAgentLogs
//     VerifyAgentLogs:
type VerifyAgentLogs struct {
	test.BaseTest
}

func NewVerifyAgentLogs(inputs map[string]any) (test.Test, error) {
	t := &VerifyAgentLogs{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAgentLogs",
			TestDescription: "Verify there are no agent crash reports",
			TestCategories:  []string{"system", "stability"},
		},
	}

	return t, nil
}

func (t *VerifyAgentLogs) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show agent logs crash",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get agent crash logs: %v", err)
		return result, nil
	}

	issues := []string{}

	if logData, ok := cmdResult.Output.(map[string]any); ok {
		if crashLogs, ok := logData["crashLogs"].([]any); ok && len(crashLogs) > 0 {
			for _, crashLog := range crashLogs {
				if logInfo, ok := crashLog.(map[string]any); ok {
					var agentName, crashTime string
					if agent, ok := logInfo["agentName"].(string); ok {
						agentName = agent
					}
					if timestamp, ok := logInfo["crashTime"].(string); ok {
						crashTime = timestamp
					}

					issues = append(issues, fmt.Sprintf("Agent crash found: %s at %s", agentName, crashTime))
				}
			}
		}

		// Also check for recent agent restarts which might indicate crashes
		if agents, ok := logData["agents"].(map[string]any); ok {
			for agentName, agentData := range agents {
				if agent, ok := agentData.(map[string]any); ok {
					if status, ok := agent["status"].(string); ok {
						if status == "crashed" || status == "restarting" {
							issues = append(issues, fmt.Sprintf("Agent %s is in %s state", agentName, status))
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Agent issues found: %v", issues)
	}

	return result, nil
}

func (t *VerifyAgentLogs) ValidateInput(input any) error {
	return nil
}

// VerifyCPUUtilization verifies that CPU utilization is below the threshold.
//
// Expected Results:
//   - Success: The test will pass if the CPU utilization is below 75%.
//   - Failure: The test will fail if the CPU utilization exceeds 75% or load average indicates high CPU stress.
//   - Error: The test will report an error if CPU utilization cannot be determined.
//
// Examples:
//   - name: VerifyCPUUtilization
//     VerifyCPUUtilization:
type VerifyCPUUtilization struct {
	test.BaseTest
}

func NewVerifyCPUUtilization(inputs map[string]any) (test.Test, error) {
	t := &VerifyCPUUtilization{
		BaseTest: test.BaseTest{
			TestName:        "VerifyCPUUtilization",
			TestDescription: "Verify CPU utilization is below 75%",
			TestCategories:  []string{"system", "performance"},
		},
	}

	return t, nil
}

func (t *VerifyCPUUtilization) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show processes summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get CPU utilization: %v", err)
		return result, nil
	}

	const maxCPUUtilization = 75.0

	if processData, ok := cmdResult.Output.(map[string]any); ok {
		if summary, ok := processData["summary"].(map[string]any); ok {
			// Check CPU idle percentage
			if cpuInfo, ok := summary["cpuInfo"].(map[string]any); ok {
				if idle, ok := cpuInfo["%Cpu(s)_id"].(float64); ok {
					utilizationPercent := 100.0 - idle

					if utilizationPercent > maxCPUUtilization {
						result.Status = test.TestFailure
						result.Message = fmt.Sprintf("CPU utilization %.2f%% exceeds threshold of %.2f%%",
							utilizationPercent, maxCPUUtilization)
						return result, nil
					}

					result.Details = map[string]any{
						"cpu_utilization_percent": utilizationPercent,
						"cpu_idle_percent":        idle,
						"threshold_percent":       maxCPUUtilization,
					}
					return result, nil
				}
			}

			// Fallback: Check 1-minute load average if CPU idle is not available
			if loadAvg, ok := summary["loadAvg"].(map[string]any); ok {
				if oneMin, ok := loadAvg["1min"].(float64); ok {
					// Rough estimation: load average > 0.75 indicates high CPU usage
					if oneMin > 0.75 {
						result.Status = test.TestFailure
						result.Message = fmt.Sprintf("High CPU load detected: 1-minute load average %.2f indicates CPU stress", oneMin)
						return result, nil
					}

					result.Details = map[string]any{
						"load_average_1min": oneMin,
						"threshold":         0.75,
					}
					return result, nil
				}
			}
		}
	}

	result.Status = test.TestError
	result.Message = "Could not determine CPU utilization from system output"
	return result, nil
}

func (t *VerifyCPUUtilization) ValidateInput(input any) error {
	return nil
}

// VerifyMemoryUtilization verifies that memory utilization is below the threshold.
//
// Expected Results:
//   - Success: The test will pass if the memory utilization is below 75%.
//   - Failure: The test will fail if the memory utilization exceeds 75%.
//   - Error: The test will report an error if memory utilization cannot be determined.
//
// Examples:
//   - name: VerifyMemoryUtilization
//     VerifyMemoryUtilization:
type VerifyMemoryUtilization struct {
	test.BaseTest
}

func NewVerifyMemoryUtilization(inputs map[string]any) (test.Test, error) {
	t := &VerifyMemoryUtilization{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMemoryUtilization",
			TestDescription: "Verify memory utilization is below 75%",
			TestCategories:  []string{"system", "performance"},
		},
	}

	return t, nil
}

func (t *VerifyMemoryUtilization) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show processes summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get memory utilization: %v", err)
		return result, nil
	}

	const maxMemoryUtilization = 75.0

	if processData, ok := cmdResult.Output.(map[string]any); ok {
		if summary, ok := processData["summary"].(map[string]any); ok {
			if memInfo, ok := summary["memInfo"].(map[string]any); ok {
				var totalMem, usedMem, freeMem float64
				var memUtilization float64

				// Try to get total and free memory
				if total, ok := memInfo["total"].(float64); ok {
					totalMem = total
				}
				if free, ok := memInfo["free"].(float64); ok {
					freeMem = free
				}
				if used, ok := memInfo["used"].(float64); ok {
					usedMem = used
				}

				// Calculate utilization
				if totalMem > 0 && freeMem >= 0 {
					memUtilization = ((totalMem - freeMem) / totalMem) * 100
				} else if totalMem > 0 && usedMem > 0 {
					memUtilization = (usedMem / totalMem) * 100
				}

				if memUtilization == 0 {
					// Try alternative fields
					if utilization, ok := memInfo["utilization"].(float64); ok {
						memUtilization = utilization
					}
				}

				if memUtilization > 0 {
					if memUtilization > maxMemoryUtilization {
						result.Status = test.TestFailure
						result.Message = fmt.Sprintf("Memory utilization %.2f%% exceeds threshold of %.2f%%",
							memUtilization, maxMemoryUtilization)
						return result, nil
					}

					result.Details = map[string]any{
						"memory_utilization_percent": memUtilization,
						"total_memory":               totalMem,
						"free_memory":                freeMem,
						"used_memory":                usedMem,
						"threshold_percent":          maxMemoryUtilization,
					}
					return result, nil
				}
			}
		}
	}

	result.Status = test.TestError
	result.Message = "Could not determine memory utilization from system output"
	return result, nil
}

func (t *VerifyMemoryUtilization) ValidateInput(input any) error {
	return nil
}

// VerifyFileSystemUtilization verifies that no filesystem partition exceeds disk space threshold.
//
// Expected Results:
//   - Success: The test will pass if no filesystem partition utilization exceeds 75%.
//   - Failure: The test will fail if any filesystem partition utilization exceeds 75%.
//   - Error: The test will report an error if disk usage information cannot be retrieved.
//
// Examples:
//   - name: VerifyFileSystemUtilization
//     VerifyFileSystemUtilization:
type VerifyFileSystemUtilization struct {
	test.BaseTest
}

func NewVerifyFileSystemUtilization(inputs map[string]any) (test.Test, error) {
	t := &VerifyFileSystemUtilization{
		BaseTest: test.BaseTest{
			TestName:        "VerifyFileSystemUtilization",
			TestDescription: "Verify no partition is utilizing more than 75% of its disk space",
			TestCategories:  []string{"system", "storage"},
		},
	}

	return t, nil
}

func (t *VerifyFileSystemUtilization) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show disk usage",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get disk usage: %v", err)
		return result, nil
	}

	const maxUtilization = 75.0
	issues := []string{}

	if diskData, ok := cmdResult.Output.(map[string]any); ok {
		if filesystems, ok := diskData["filesystems"].(map[string]any); ok {
			for mountPoint, fsData := range filesystems {
				if fs, ok := fsData.(map[string]any); ok {
					var utilization float64
					var totalSize, usedSize, availSize float64

					if total, ok := fs["total"].(float64); ok {
						totalSize = total
					}
					if used, ok := fs["used"].(float64); ok {
						usedSize = used
					}
					if avail, ok := fs["available"].(float64); ok {
						availSize = avail
					}

					// Calculate utilization percentage
					if totalSize > 0 && usedSize >= 0 {
						utilization = (usedSize / totalSize) * 100
					} else if totalSize > 0 && availSize >= 0 {
						utilization = ((totalSize - availSize) / totalSize) * 100
					}

					// Check if utilization exceeds threshold
					if utilization > maxUtilization {
						issues = append(issues, fmt.Sprintf("Filesystem %s utilization %.2f%% exceeds threshold %.2f%% (Used: %.0f, Total: %.0f)",
							mountPoint, utilization, maxUtilization, usedSize, totalSize))
					}
				}
			}
		}

		// Also check for disk partitions if filesystems data is not available
		if len(issues) == 0 {
			if partitions, ok := diskData["partitions"].([]any); ok {
				for _, partition := range partitions {
					if part, ok := partition.(map[string]any); ok {
						var utilization float64
						var mountPoint string

						if mount, ok := part["mountPoint"].(string); ok {
							mountPoint = mount
						}
						if util, ok := part["utilization"].(float64); ok {
							utilization = util
						} else if used, ok := part["used"].(float64); ok {
							if total, ok := part["total"].(float64); ok && total > 0 {
								utilization = (used / total) * 100
							}
						}

						if utilization > maxUtilization {
							issues = append(issues, fmt.Sprintf("Partition %s utilization %.2f%% exceeds threshold %.2f%%",
								mountPoint, utilization, maxUtilization))
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Filesystem utilization issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyFileSystemUtilization) ValidateInput(input any) error {
	return nil
}

// VerifyFlashUtilization verifies that the flash drive has sufficient free space.
//
// Expected Results:
//   - Success: The test will pass if flash drive utilization is below the specified threshold (default 70%).
//   - Failure: The test will fail if flash drive utilization exceeds the threshold.
//   - Error: The test will report an error if flash storage information cannot be retrieved.
//
// Examples:
//   - name: VerifyFlashUtilization with default threshold
//     VerifyFlashUtilization:
//
//   - name: VerifyFlashUtilization with custom threshold and peer check
//     VerifyFlashUtilization:
//       max_utilization: 80.0
//       check_peer_supervisor: true
type VerifyFlashUtilization struct {
	test.BaseTest
	MaxUtilization      float64 `yaml:"max_utilization,omitempty" json:"max_utilization,omitempty"`
	CheckPeerSupervisor bool    `yaml:"check_peer_supervisor,omitempty" json:"check_peer_supervisor,omitempty"`
}

func NewVerifyFlashUtilization(inputs map[string]any) (test.Test, error) {
	t := &VerifyFlashUtilization{
		BaseTest: test.BaseTest{
			TestName:        "VerifyFlashUtilization",
			TestDescription: "Verify the free space on the flash drive is sufficient",
			TestCategories:  []string{"system", "storage"},
		},
		MaxUtilization:      70.0, // Default 70% threshold
		CheckPeerSupervisor: false,
	}

	if inputs != nil {
		if maxUtil, ok := inputs["max_utilization"].(float64); ok {
			t.MaxUtilization = maxUtil
		}
		if checkPeer, ok := inputs["check_peer_supervisor"].(bool); ok {
			t.CheckPeerSupervisor = checkPeer
		}
	}

	return t, nil
}

func (t *VerifyFlashUtilization) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show disk space",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get flash disk usage: %v", err)
		return result, nil
	}

	issues := []string{}

	if diskData, ok := cmdResult.Output.(map[string]any); ok {
		// Check the main flash storage
		if storage, ok := diskData["storage"].(map[string]any); ok {
			for deviceName, deviceData := range storage {
				if device, ok := deviceData.(map[string]any); ok {
					// Look for flash drives (typically /mnt/flash, /mnt/drive, etc.)
					if mountPoint, ok := device["mountPoint"].(string); ok {
						if mountPoint == "/mnt/flash" || mountPoint == "/mnt/drive" ||
							deviceName == "flash:" || deviceName == "system:" {

							var utilization float64
							var totalSpace, usedSpace, freeSpace float64

							if total, ok := device["total"].(float64); ok {
								totalSpace = total
							}
							if used, ok := device["used"].(float64); ok {
								usedSpace = used
							}
							if free, ok := device["free"].(float64); ok {
								freeSpace = free
							}

							// Calculate utilization
							if totalSpace > 0 {
								if usedSpace > 0 {
									utilization = (usedSpace / totalSpace) * 100
								} else if freeSpace > 0 {
									utilization = ((totalSpace - freeSpace) / totalSpace) * 100
								}
							}

							if utilization > t.MaxUtilization {
								issues = append(issues, fmt.Sprintf("Flash device %s utilization %.2f%% exceeds threshold %.2f%% (Used: %.0f, Total: %.0f)",
									deviceName, utilization, t.MaxUtilization, usedSpace, totalSpace))
							}
						}
					}
				}
			}
		}

		// Check peer supervisor if requested (for dual-supervisor systems)
		if t.CheckPeerSupervisor {
			if peer, ok := diskData["peerStorage"].(map[string]any); ok {
				for deviceName, deviceData := range peer {
					if device, ok := deviceData.(map[string]any); ok {
						if mountPoint, ok := device["mountPoint"].(string); ok {
							if mountPoint == "/mnt/flash" || mountPoint == "/mnt/drive" {
								var utilization float64
								var totalSpace, usedSpace, freeSpace float64

								if total, ok := device["total"].(float64); ok {
									totalSpace = total
								}
								if used, ok := device["used"].(float64); ok {
									usedSpace = used
								}
								if free, ok := device["free"].(float64); ok {
									freeSpace = free
								}

								// Calculate utilization
								if totalSpace > 0 {
									if usedSpace > 0 {
										utilization = (usedSpace / totalSpace) * 100
									} else if freeSpace > 0 {
										utilization = ((totalSpace - freeSpace) / totalSpace) * 100
									}
								}

								if utilization > t.MaxUtilization {
									issues = append(issues, fmt.Sprintf("Peer supervisor flash device %s utilization %.2f%% exceeds threshold %.2f%%",
										deviceName, utilization, t.MaxUtilization))
								}
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Flash utilization issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyFlashUtilization) ValidateInput(input any) error {
	if t.MaxUtilization <= 0 || t.MaxUtilization > 100 {
		return fmt.Errorf("max_utilization must be between 0 and 100")
	}
	return nil
}

// VerifyMaintenance verifies the device is not currently under or entering maintenance mode.
//
// Expected Results:
//   - Success: The test will pass if the device is not in maintenance mode and no maintenance is scheduled.
//   - Failure: The test will fail if the device is in maintenance mode, entering maintenance, or has active scheduled maintenance.
//   - Error: The test will report an error if maintenance status cannot be determined.
//
// Examples:
//   - name: VerifyMaintenance
//     VerifyMaintenance:
type VerifyMaintenance struct {
	test.BaseTest
}

func NewVerifyMaintenance(inputs map[string]any) (test.Test, error) {
	t := &VerifyMaintenance{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMaintenance",
			TestDescription: "Verify device is not currently under or entering maintenance",
			TestCategories:  []string{"system", "maintenance"},
		},
	}

	return t, nil
}

func (t *VerifyMaintenance) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show maintenance",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get maintenance status: %v", err)
		return result, nil
	}

	issues := []string{}

	if maintenanceData, ok := cmdResult.Output.(map[string]any); ok {
		// Check overall maintenance status
		if status, ok := maintenanceData["status"].(string); ok {
			if status == "maintenance" || status == "entering-maintenance" {
				issues = append(issues, fmt.Sprintf("Device is in %s mode", status))
			}
		}

		// Check maintenance mode state
		if maintenanceMode, ok := maintenanceData["maintenanceMode"].(bool); ok && maintenanceMode {
			issues = append(issues, "Device maintenance mode is enabled")
		}

		// Check for scheduled maintenance
		if scheduled, ok := maintenanceData["scheduledMaintenance"].(map[string]any); ok {
			if active, ok := scheduled["active"].(bool); ok && active {
				var startTime, endTime string
				if start, ok := scheduled["startTime"].(string); ok {
					startTime = start
				}
				if end, ok := scheduled["endTime"].(string); ok {
					endTime = end
				}
				issues = append(issues, fmt.Sprintf("Scheduled maintenance is active from %s to %s", startTime, endTime))
			}
		}

		// Check for maintenance units (protocols or features under maintenance)
		if units, ok := maintenanceData["units"].([]any); ok && len(units) > 0 {
			for _, unit := range units {
				if unitInfo, ok := unit.(map[string]any); ok {
					if name, ok := unitInfo["name"].(string); ok {
						if state, ok := unitInfo["state"].(string); ok {
							if state == "maintenance" || state == "entering-maintenance" {
								issues = append(issues, fmt.Sprintf("Unit %s is in %s state", name, state))
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Maintenance issues detected: %v", issues)
	}

	return result, nil
}

func (t *VerifyMaintenance) ValidateInput(input any) error {
	return nil
}