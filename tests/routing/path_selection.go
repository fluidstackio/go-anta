package routing

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyPathsHealth verifies the path and telemetry state of all paths under router path-selection.
//
// This test validates that all configured dynamic path selection (DPS) paths are in a healthy state
// by checking both their connection status and telemetry functionality.
//
// The test performs the following checks:
//  1. Verifies that at least one path is configured in the path-selection configuration.
//  2. Validates that all path states are in acceptable conditions ('IPsec established' or 'Resolved').
//  3. Confirms that telemetry state is 'active' for all paths to ensure monitoring is functional.
//
// Expected Results:
//   - Success: The test will pass if all paths have acceptable states and active telemetry.
//   - Failure: The test will fail if no paths are configured, any path has an unacceptable state, or telemetry is inactive.
//   - Error: The test will report an error if path-selection information cannot be retrieved.
//
// Examples:
//
//   - name: VerifyPathsHealth basic check
//     VerifyPathsHealth: {}
//
//   - name: VerifyPathsHealth comprehensive validation
//     VerifyPathsHealth:
//     # No parameters needed - validates all configured paths
type VerifyPathsHealth struct {
	test.BaseTest
}

func NewVerifyPathsHealth(inputs map[string]any) (test.Test, error) {
	t := &VerifyPathsHealth{
		BaseTest: test.BaseTest{
			TestName:        "VerifyPathsHealth",
			TestDescription: "Verify path and telemetry state of all path-selection paths",
			TestCategories:  []string{"routing", "path-selection"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyPathsHealth) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show path-selection paths",
		Format:   "json",
		UseCache: false,
		Revision: 1, // Using revision 1 as specified in Python implementation
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get path-selection paths: %v", err)
		return result, nil
	}

	var paths []PathInfo
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if pathData, ok := data["paths"].(map[string]any); ok {
			for pathName, pathInfo := range pathData {
				if info, ok := pathInfo.(map[string]any); ok {
					path := PathInfo{Name: pathName}

					if state, ok := info["state"].(string); ok {
						path.State = state
					}
					if telemetryState, ok := info["telemetryState"].(string); ok {
						path.TelemetryState = telemetryState
					}
					if sourceAddr, ok := info["sourceAddress"].(string); ok {
						path.SourceAddress = sourceAddr
					}
					if destAddr, ok := info["destinationAddress"].(string); ok {
						path.DestinationAddress = destAddr
					}
					if pathGroup, ok := info["pathGroup"].(string); ok {
						path.PathGroup = pathGroup
					}

					paths = append(paths, path)
				}
			}
		}
	}

	if len(paths) == 0 {
		result.Status = test.TestFailure
		result.Message = "No paths configured under router path-selection"
		return result, nil
	}

	failures := []string{}
	for _, path := range paths {
		// Check path state - acceptable states are 'IPsec established' or 'Resolved'
		if !t.isAcceptablePathState(path.State) {
			failures = append(failures, fmt.Sprintf("Path %s: unacceptable state '%s'", path.Name, path.State))
		}

		// Check telemetry state - must be 'active'
		if !strings.EqualFold(path.TelemetryState, "active") {
			failures = append(failures, fmt.Sprintf("Path %s: telemetry state '%s' (expected 'active')", path.Name, path.TelemetryState))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Path health failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyPathsHealth) isAcceptablePathState(state string) bool {
	acceptableStates := []string{
		"IPsec established",
		"ipsecEstablished", // Alternative format
		"Resolved",
		"routeResolved", // Alternative format
	}

	for _, acceptable := range acceptableStates {
		if strings.EqualFold(state, acceptable) {
			return true
		}
	}
	return false
}

func (t *VerifyPathsHealth) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifySpecificPath verifies the DPS path and telemetry state of a specific IPv4 peer.
//
// This test validates the dynamic path selection configuration for a specific peer by
// checking the path group, addresses, connection state, and telemetry status.
//
// The test performs the following checks:
//  1. Verifies that the specified peer and path group exist in the configuration.
//  2. Validates that the source and destination addresses match the expected values.
//  3. Confirms that the path state is acceptable ('ipsecEstablished' or 'routeResolved').
//  4. Ensures that telemetry state is 'active' for proper monitoring.
//
// Expected Results:
//   - Success: The test will pass if the specific path exists with correct configuration and healthy state.
//   - Failure: The test will fail if the path doesn't exist, addresses don't match, state is unacceptable, or telemetry is inactive.
//   - Error: The test will report an error if path-selection information cannot be retrieved.
//
// Examples:
//
//   - name: VerifySpecificPath for peer 10.1.1.1
//     VerifySpecificPath:
//     peer: "10.1.1.1"
//     path_group: "internet"
//     source_address: "10.0.0.1"
//     destination_address: "10.1.1.1"
//
//   - name: VerifySpecificPath for MPLS path
//     VerifySpecificPath:
//     peer: "192.168.1.100"
//     path_group: "mpls-primary"
//     source_address: "192.168.1.1"
//     destination_address: "192.168.1.100"
type VerifySpecificPath struct {
	test.BaseTest
	Peer               string `yaml:"peer" json:"peer"`
	PathGroup          string `yaml:"path_group" json:"path_group"`
	SourceAddress      string `yaml:"source_address" json:"source_address"`
	DestinationAddress string `yaml:"destination_address" json:"destination_address"`
}

func NewVerifySpecificPath(inputs map[string]any) (test.Test, error) {
	t := &VerifySpecificPath{
		BaseTest: test.BaseTest{
			TestName:        "VerifySpecificPath",
			TestDescription: "Verify DPS path and telemetry state of a specific IPv4 peer",
			TestCategories:  []string{"routing", "path-selection"},
		},
	}

	if inputs != nil {
		if peer, ok := inputs["peer"].(string); ok {
			t.Peer = peer
		}
		if pathGroup, ok := inputs["path_group"].(string); ok {
			t.PathGroup = pathGroup
		}
		if sourceAddr, ok := inputs["source_address"].(string); ok {
			t.SourceAddress = sourceAddr
		}
		if destAddr, ok := inputs["destination_address"].(string); ok {
			t.DestinationAddress = destAddr
		}
	}

	return t, nil
}

func (t *VerifySpecificPath) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show path-selection paths",
		Format:   "json",
		UseCache: false,
		Revision: 1,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get path-selection paths: %v", err)
		return result, nil
	}

	var targetPath *PathInfo
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if pathData, ok := data["paths"].(map[string]any); ok {
			for pathName, pathInfo := range pathData {
				if info, ok := pathInfo.(map[string]any); ok {
					path := PathInfo{Name: pathName}

					if state, ok := info["state"].(string); ok {
						path.State = state
					}
					if telemetryState, ok := info["telemetryState"].(string); ok {
						path.TelemetryState = telemetryState
					}
					if sourceAddr, ok := info["sourceAddress"].(string); ok {
						path.SourceAddress = sourceAddr
					}
					if destAddr, ok := info["destinationAddress"].(string); ok {
						path.DestinationAddress = destAddr
					}
					if pathGroup, ok := info["pathGroup"].(string); ok {
						path.PathGroup = pathGroup
					}

					// Check if this is the path we're looking for
					if t.matchesTargetPath(&path) {
						targetPath = &path
						break
					}
				}
			}
		}
	}

	if targetPath == nil {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Path not found for peer %s in path group %s", t.Peer, t.PathGroup)
		return result, nil
	}

	failures := []string{}

	// Validate source address
	if targetPath.SourceAddress != t.SourceAddress {
		failures = append(failures, fmt.Sprintf("Source address mismatch: expected %s, got %s", t.SourceAddress, targetPath.SourceAddress))
	}

	// Validate destination address
	if targetPath.DestinationAddress != t.DestinationAddress {
		failures = append(failures, fmt.Sprintf("Destination address mismatch: expected %s, got %s", t.DestinationAddress, targetPath.DestinationAddress))
	}

	// Check path state - acceptable states are 'ipsecEstablished' or 'routeResolved'
	if !t.isAcceptablePathState(targetPath.State) {
		failures = append(failures, fmt.Sprintf("Unacceptable path state: %s", targetPath.State))
	}

	// Check telemetry state - must be 'active'
	if !strings.EqualFold(targetPath.TelemetryState, "active") {
		failures = append(failures, fmt.Sprintf("Telemetry state: expected 'active', got '%s'", targetPath.TelemetryState))
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Specific path validation failures: %v", failures)
	}

	return result, nil
}

func (t *VerifySpecificPath) matchesTargetPath(path *PathInfo) bool {
	// Match by peer (destination address) and path group
	return path.DestinationAddress == t.Peer && path.PathGroup == t.PathGroup
}

func (t *VerifySpecificPath) isAcceptablePathState(state string) bool {
	acceptableStates := []string{
		"ipsecEstablished",
		"IPsec established", // Alternative format
		"routeResolved",
		"Resolved", // Alternative format
	}

	for _, acceptable := range acceptableStates {
		if strings.EqualFold(state, acceptable) {
			return true
		}
	}
	return false
}

func (t *VerifySpecificPath) ValidateInput(input any) error {
	if t.Peer == "" {
		return fmt.Errorf("peer must be specified")
	}
	if t.PathGroup == "" {
		return fmt.Errorf("path_group must be specified")
	}
	if t.SourceAddress == "" {
		return fmt.Errorf("source_address must be specified")
	}
	if t.DestinationAddress == "" {
		return fmt.Errorf("destination_address must be specified")
	}

	// Basic IP address format validation (simplified)
	if !t.isValidIPv4(t.Peer) {
		return fmt.Errorf("peer must be a valid IPv4 address")
	}
	if !t.isValidIPv4(t.SourceAddress) {
		return fmt.Errorf("source_address must be a valid IPv4 address")
	}
	if !t.isValidIPv4(t.DestinationAddress) {
		return fmt.Errorf("destination_address must be a valid IPv4 address")
	}

	return nil
}

func (t *VerifySpecificPath) isValidIPv4(ip string) bool {
	// Simple IPv4 validation - checks for basic format
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}
	return true
}

// PathInfo represents information about a path-selection path
type PathInfo struct {
	Name               string
	State              string
	TelemetryState     string
	SourceAddress      string
	DestinationAddress string
	PathGroup          string
}
