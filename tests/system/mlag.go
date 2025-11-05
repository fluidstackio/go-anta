package system

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyMlagStatus verifies the overall MLAG (Multi-Chassis Link Aggregation) health configuration.
//
// This test validates the operational status of MLAG by checking the negotiation status,
// local interface status, and peer link status to ensure proper redundancy operation.
//
// The test performs the following checks:
//   1. Verifies that MLAG is enabled on the device.
//   2. Validates that the MLAG state is 'active'.
//   3. Confirms that negotiation status is 'connected'.
//   4. Checks that local interface status is operational.
//   5. Ensures peer link status is healthy.
//
// Expected Results:
//   - Success: The test will pass if MLAG is active with proper negotiation and interface states.
//   - Failure: The test will fail if MLAG state is not active, negotiation is not connected, or interfaces are unhealthy.
//   - Error: The test will report an error if MLAG information cannot be retrieved.
//   - Skipped: The test will be skipped if MLAG is disabled on the device.
//
// Examples:
//   - name: VerifyMlagStatus basic check
//     VerifyMlagStatus: {}
//
//   - name: VerifyMlagStatus comprehensive validation
//     VerifyMlagStatus:
//       # No parameters needed - validates overall MLAG health
type VerifyMlagStatus struct {
	test.BaseTest
}

func NewVerifyMlagStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifyMlagStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMlagStatus",
			TestDescription: "Verify overall MLAG health configuration",
			TestCategories:  []string{"system", "mlag"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyMlagStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show mlag",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get MLAG status: %v", err)
		return result, nil
	}

	var mlagInfo MlagInfo
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if state, ok := data["state"].(string); ok {
			mlagInfo.State = state
		}
		if negStatus, ok := data["negotiationStatus"].(string); ok {
			mlagInfo.NegotiationStatus = negStatus
		}
		if localIntfStatus, ok := data["localIntfStatus"].(string); ok {
			mlagInfo.LocalIntfStatus = localIntfStatus
		}
		if peerLinkStatus, ok := data["peerLinkStatus"].(string); ok {
			mlagInfo.PeerLinkStatus = peerLinkStatus
		}
	}

	// Check if MLAG is disabled
	if strings.EqualFold(mlagInfo.State, "disabled") || mlagInfo.State == "" {
		result.Status = test.TestSkipped
		result.Message = "MLAG is disabled"
		return result, nil
	}

	failures := []string{}

	// Check MLAG state
	if !strings.EqualFold(mlagInfo.State, "active") {
		failures = append(failures, fmt.Sprintf("MLAG state is '%s', expected 'active'", mlagInfo.State))
	}

	// Check negotiation status
	if !strings.EqualFold(mlagInfo.NegotiationStatus, "connected") {
		failures = append(failures, fmt.Sprintf("Negotiation status is '%s', expected 'connected'", mlagInfo.NegotiationStatus))
	}

	// Check local interface status
	if mlagInfo.LocalIntfStatus != "" && !strings.EqualFold(mlagInfo.LocalIntfStatus, "up") {
		failures = append(failures, fmt.Sprintf("Local interface status is '%s', expected 'up'", mlagInfo.LocalIntfStatus))
	}

	// Check peer link status
	if mlagInfo.PeerLinkStatus != "" && !strings.EqualFold(mlagInfo.PeerLinkStatus, "up") {
		failures = append(failures, fmt.Sprintf("Peer link status is '%s', expected 'up'", mlagInfo.PeerLinkStatus))
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("MLAG status failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyMlagStatus) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifyMlagInterfaces verifies that no MLAG interfaces are in inactive or active-partial state.
//
// This test ensures that all MLAG ports are functioning properly by checking for ports
// that are either inactive or in an active-partial state, which could indicate problems
// with the MLAG configuration or connectivity.
//
// The test performs the following checks:
//   1. Verifies that MLAG is enabled and active on the device.
//   2. Examines all MLAG interface states.
//   3. Identifies any interfaces in 'inactive' or 'active-partial' states.
//   4. Reports failures for problematic interface states.
//
// Expected Results:
//   - Success: The test will pass if all MLAG interfaces are in proper active states.
//   - Failure: The test will fail if any MLAG interfaces are inactive or active-partial.
//   - Error: The test will report an error if MLAG interface information cannot be retrieved.
//   - Skipped: The test will be skipped if MLAG is disabled on the device.
//
// Examples:
//   - name: VerifyMlagInterfaces basic check
//     VerifyMlagInterfaces: {}
//
//   - name: VerifyMlagInterfaces comprehensive validation
//     VerifyMlagInterfaces:
//       # No parameters needed - validates all MLAG interface states
type VerifyMlagInterfaces struct {
	test.BaseTest
}

func NewVerifyMlagInterfaces(inputs map[string]any) (test.Test, error) {
	t := &VerifyMlagInterfaces{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMlagInterfaces",
			TestDescription: "Verify no MLAG interfaces are inactive or active-partial",
			TestCategories:  []string{"system", "mlag"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyMlagInterfaces) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show mlag detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get MLAG detail: %v", err)
		return result, nil
	}

	data, ok := cmdResult.Output.(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to parse MLAG output"
		return result, nil
	}

	mlagState, _ := data["state"].(string)

	interfaces := make(map[string]MlagInterfaceInfo)
	intfs, ok := data["interfaces"].(map[string]any)
	if ok {
		for intfName, intfData := range intfs {
			intfInfo, ok := intfData.(map[string]any)
			if !ok {
				continue
			}

			info := MlagInterfaceInfo{Name: intfName}
			if status, ok := intfInfo["status"].(string); ok {
				info.Status = status
			}
			if state, ok := intfInfo["state"].(string); ok {
				info.State = state
			}
			interfaces[intfName] = info
		}
	}

	// Check if MLAG is disabled
	if strings.EqualFold(mlagState, "disabled") || mlagState == "" {
		result.Status = test.TestSkipped
		result.Message = "MLAG is disabled"
		return result, nil
	}

	failures := []string{}
	for intfName, intfInfo := range interfaces {
		// Check for inactive or active-partial states
		if strings.EqualFold(intfInfo.Status, "inactive") ||
			strings.EqualFold(intfInfo.Status, "active-partial") ||
			strings.EqualFold(intfInfo.State, "inactive") ||
			strings.EqualFold(intfInfo.State, "active-partial") {
			failures = append(failures, fmt.Sprintf("Interface %s has problematic state: status='%s', state='%s'", intfName, intfInfo.Status, intfInfo.State))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("MLAG interface failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyMlagInterfaces) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifyMlagConfigSanity verifies MLAG configuration consistency between peers.
//
// This test checks for configuration inconsistencies in the MLAG setup that could
// lead to operational issues, including global configuration mismatches and
// interface-specific configuration problems.
//
// The test performs the following checks:
//   1. Verifies that MLAG is active on the device.
//   2. Examines configuration sanity results from the EOS system.
//   3. Identifies any global or interface configuration inconsistencies.
//   4. Reports specific configuration mismatches between MLAG peers.
//
// Expected Results:
//   - Success: The test will pass if no configuration inconsistencies are detected.
//   - Failure: The test will fail if any global or interface configuration mismatches are found.
//   - Error: The test will report an error if configuration sanity information cannot be retrieved.
//   - Skipped: The test will be skipped if MLAG is not active on the device.
//
// Examples:
//   - name: VerifyMlagConfigSanity basic check
//     VerifyMlagConfigSanity: {}
//
//   - name: VerifyMlagConfigSanity comprehensive validation
//     VerifyMlagConfigSanity:
//       # No parameters needed - validates configuration consistency
type VerifyMlagConfigSanity struct {
	test.BaseTest
}

func NewVerifyMlagConfigSanity(inputs map[string]any) (test.Test, error) {
	t := &VerifyMlagConfigSanity{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMlagConfigSanity",
			TestDescription: "Verify MLAG configuration consistency",
			TestCategories:  []string{"system", "mlag"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyMlagConfigSanity) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// First check MLAG status
	statusCmd := device.Command{
		Template: "show mlag",
		Format:   "json",
		UseCache: false,
	}

	statusResult, err := dev.Execute(ctx, statusCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get MLAG status: %v", err)
		return result, nil
	}

	var mlagState string
	if data, ok := statusResult.Output.(map[string]any); ok {
		if state, ok := data["state"].(string); ok {
			mlagState = state
		}
	}

	// Check if MLAG is not active
	if !strings.EqualFold(mlagState, "active") {
		result.Status = test.TestSkipped
		result.Message = "MLAG is not active"
		return result, nil
	}

	// Now check config sanity
	sanityCmd := device.Command{
		Template: "show mlag config-sanity",
		Format:   "json",
		UseCache: false,
	}

	sanityResult, err := dev.Execute(ctx, sanityCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get MLAG config-sanity: %v", err)
		return result, nil
	}

	data, ok := sanityResult.Output.(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to parse MLAG config-sanity output"
		return result, nil
	}

	failures := []string{}

	// Check global config sanity
	globalSanity, ok := data["globalSanity"].(map[string]any)
	if ok {
		for configKey, configStatus := range globalSanity {
			statusMap, ok := configStatus.(map[string]any)
			if !ok {
				continue
			}

			if consistent, ok := statusMap["consistent"].(bool); ok && !consistent {
				failures = append(failures, fmt.Sprintf("Global config inconsistency: %s", configKey))
			}
		}
	}

	// Check interface config sanity
	intfSanity, ok := data["interfaceSanity"].(map[string]any)
	if ok {
		for intfName, intfStatus := range intfSanity {
			statusMap, ok := intfStatus.(map[string]any)
			if !ok {
				continue
			}

			if consistent, ok := statusMap["consistent"].(bool); ok && !consistent {
				failures = append(failures, fmt.Sprintf("Interface %s config inconsistency", intfName))
			}
		}
	}

	// Check for any specific sanity issues
	if issues, ok := data["issues"].([]any); ok && len(issues) > 0 {
		for _, issue := range issues {
			if issueStr, ok := issue.(string); ok {
				failures = append(failures, fmt.Sprintf("Config sanity issue: %s", issueStr))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("MLAG config sanity failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyMlagConfigSanity) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifyMlagReloadDelay verifies MLAG reload delay configuration parameters.
//
// This test validates the reload delay settings for both MLAG and non-MLAG ports,
// which control how long the system waits before bringing up ports after a reload
// to prevent network loops and ensure proper MLAG synchronization.
//
// The test performs the following checks:
//   1. Verifies that MLAG is enabled on the device.
//   2. Validates the configured reload delay for MLAG ports.
//   3. Checks the configured reload delay for non-MLAG ports.
//   4. Compares actual delays against expected values if specified.
//
// Expected Results:
//   - Success: The test will pass if reload delay configurations match expected values.
//   - Failure: The test will fail if reload delays don't match the specified requirements.
//   - Error: The test will report an error if reload delay information cannot be retrieved.
//   - Skipped: The test will be skipped if MLAG is disabled on the device.
//
// Examples:
//   - name: VerifyMlagReloadDelay with specific delays
//     VerifyMlagReloadDelay:
//       reload_delay: 300
//       reload_delay_non_mlag: 330
//
//   - name: VerifyMlagReloadDelay basic check
//     VerifyMlagReloadDelay:
//       reload_delay: 240
type VerifyMlagReloadDelay struct {
	test.BaseTest
	ReloadDelay        *int `yaml:"reload_delay,omitempty" json:"reload_delay,omitempty"`
	ReloadDelayNonMlag *int `yaml:"reload_delay_non_mlag,omitempty" json:"reload_delay_non_mlag,omitempty"`
}

func NewVerifyMlagReloadDelay(inputs map[string]any) (test.Test, error) {
	t := &VerifyMlagReloadDelay{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMlagReloadDelay",
			TestDescription: "Verify MLAG reload delay configuration",
			TestCategories:  []string{"system", "mlag"},
		},
	}

	if inputs != nil {
		if reloadDelay, ok := inputs["reload_delay"]; ok {
			if delay, ok := reloadDelay.(float64); ok {
				delayInt := int(delay)
				t.ReloadDelay = &delayInt
			} else if delay, ok := reloadDelay.(int); ok {
				t.ReloadDelay = &delay
			}
		}

		if reloadDelayNonMlag, ok := inputs["reload_delay_non_mlag"]; ok {
			if delay, ok := reloadDelayNonMlag.(float64); ok {
				delayInt := int(delay)
				t.ReloadDelayNonMlag = &delayInt
			} else if delay, ok := reloadDelayNonMlag.(int); ok {
				t.ReloadDelayNonMlag = &delay
			}
		}
	}

	return t, nil
}

func (t *VerifyMlagReloadDelay) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show mlag detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get MLAG detail: %v", err)
		return result, nil
	}

	var mlagState string
	var actualReloadDelay, actualReloadDelayNonMlag int

	if data, ok := cmdResult.Output.(map[string]any); ok {
		if state, ok := data["state"].(string); ok {
			mlagState = state
		}

		if reloadDelay, ok := data["reloadDelay"].(float64); ok {
			actualReloadDelay = int(reloadDelay)
		} else if reloadDelay, ok := data["reloadDelay"].(string); ok {
			if delay, err := strconv.Atoi(reloadDelay); err == nil {
				actualReloadDelay = delay
			}
		}

		if reloadDelayNonMlag, ok := data["reloadDelayNonMlag"].(float64); ok {
			actualReloadDelayNonMlag = int(reloadDelayNonMlag)
		} else if reloadDelayNonMlag, ok := data["reloadDelayNonMlag"].(string); ok {
			if delay, err := strconv.Atoi(reloadDelayNonMlag); err == nil {
				actualReloadDelayNonMlag = delay
			}
		}
	}

	// Check if MLAG is disabled
	if strings.EqualFold(mlagState, "disabled") || mlagState == "" {
		result.Status = test.TestSkipped
		result.Message = "MLAG is disabled"
		return result, nil
	}

	failures := []string{}

	// Check reload delay if specified
	if t.ReloadDelay != nil && actualReloadDelay != *t.ReloadDelay {
		failures = append(failures, fmt.Sprintf("MLAG reload delay: expected %d, got %d", *t.ReloadDelay, actualReloadDelay))
	}

	// Check non-MLAG reload delay if specified
	if t.ReloadDelayNonMlag != nil && actualReloadDelayNonMlag != *t.ReloadDelayNonMlag {
		failures = append(failures, fmt.Sprintf("Non-MLAG reload delay: expected %d, got %d", *t.ReloadDelayNonMlag, actualReloadDelayNonMlag))
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("MLAG reload delay failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyMlagReloadDelay) ValidateInput(input any) error {
	if t.ReloadDelay != nil && *t.ReloadDelay < 0 {
		return fmt.Errorf("reload_delay must be non-negative")
	}
	if t.ReloadDelayNonMlag != nil && *t.ReloadDelayNonMlag < 0 {
		return fmt.Errorf("reload_delay_non_mlag must be non-negative")
	}
	return nil
}

// VerifyMlagDualPrimary verifies MLAG dual-primary detection and recovery configuration.
//
// This test validates the dual-primary detection mechanism which prevents split-brain
// scenarios in MLAG configurations by ensuring proper detection delays, recovery
// settings, and error handling parameters.
//
// The test performs the following checks:
//   1. Verifies that MLAG is enabled on the device.
//   2. Validates dual-primary detection delay configuration.
//   3. Checks recovery delay settings for dual-primary scenarios.
//   4. Confirms error handling and interface disabling behavior.
//   5. Validates primary priority settings if configured.
//
// Expected Results:
//   - Success: The test will pass if dual-primary detection parameters match expected values.
//   - Failure: The test will fail if detection or recovery settings don't match requirements.
//   - Error: The test will report an error if dual-primary configuration cannot be retrieved.
//   - Skipped: The test will be skipped if MLAG is disabled on the device.
//
// Examples:
//   - name: VerifyMlagDualPrimary with detection delay
//     VerifyMlagDualPrimary:
//       detection_delay: 200
//       recovery_delay: 3600
//       errdisabled: true
//
//   - name: VerifyMlagDualPrimary with priority
//     VerifyMlagDualPrimary:
//       detection_delay: 120
//       primary_priority: 100
type VerifyMlagDualPrimary struct {
	test.BaseTest
	DetectionDelay  *int  `yaml:"detection_delay,omitempty" json:"detection_delay,omitempty"`
	RecoveryDelay   *int  `yaml:"recovery_delay,omitempty" json:"recovery_delay,omitempty"`
	Errdisabled     *bool `yaml:"errdisabled,omitempty" json:"errdisabled,omitempty"`
	PrimaryPriority *int  `yaml:"primary_priority,omitempty" json:"primary_priority,omitempty"`
}

func NewVerifyMlagDualPrimary(inputs map[string]any) (test.Test, error) {
	t := &VerifyMlagDualPrimary{
		BaseTest: test.BaseTest{
			TestName:        "VerifyMlagDualPrimary",
			TestDescription: "Verify MLAG dual-primary detection configuration",
			TestCategories:  []string{"system", "mlag"},
		},
	}

	if inputs != nil {
		if detectionDelay, ok := inputs["detection_delay"]; ok {
			if delay, ok := detectionDelay.(float64); ok {
				delayInt := int(delay)
				t.DetectionDelay = &delayInt
			} else if delay, ok := detectionDelay.(int); ok {
				t.DetectionDelay = &delay
			}
		}

		if recoveryDelay, ok := inputs["recovery_delay"]; ok {
			if delay, ok := recoveryDelay.(float64); ok {
				delayInt := int(delay)
				t.RecoveryDelay = &delayInt
			} else if delay, ok := recoveryDelay.(int); ok {
				t.RecoveryDelay = &delay
			}
		}

		if errdisabled, ok := inputs["errdisabled"].(bool); ok {
			t.Errdisabled = &errdisabled
		}

		if primaryPriority, ok := inputs["primary_priority"]; ok {
			if priority, ok := primaryPriority.(float64); ok {
				priorityInt := int(priority)
				t.PrimaryPriority = &priorityInt
			} else if priority, ok := primaryPriority.(int); ok {
				t.PrimaryPriority = &priority
			}
		}
	}

	return t, nil
}

func (t *VerifyMlagDualPrimary) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show mlag detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get MLAG detail: %v", err)
		return result, nil
	}

	var mlagState string
	var dualPrimaryConfig DualPrimaryConfig

	if data, ok := cmdResult.Output.(map[string]any); ok {
		if state, ok := data["state"].(string); ok {
			mlagState = state
		}

		if dualPrimary, ok := data["dualPrimary"].(map[string]any); ok {
			if detectionDelay, ok := dualPrimary["detectionDelay"].(float64); ok {
				dualPrimaryConfig.DetectionDelay = int(detectionDelay)
			}
			if recoveryDelay, ok := dualPrimary["recoveryDelay"].(float64); ok {
				dualPrimaryConfig.RecoveryDelay = int(recoveryDelay)
			}
			if errdisabled, ok := dualPrimary["errdisabled"].(bool); ok {
				dualPrimaryConfig.Errdisabled = errdisabled
			}
			if priority, ok := dualPrimary["primaryPriority"].(float64); ok {
				dualPrimaryConfig.PrimaryPriority = int(priority)
			}
		}
	}

	// Check if MLAG is disabled
	if strings.EqualFold(mlagState, "disabled") || mlagState == "" {
		result.Status = test.TestSkipped
		result.Message = "MLAG is disabled"
		return result, nil
	}

	failures := []string{}

	// Check detection delay if specified
	if t.DetectionDelay != nil && dualPrimaryConfig.DetectionDelay != *t.DetectionDelay {
		failures = append(failures, fmt.Sprintf("Dual-primary detection delay: expected %d, got %d", *t.DetectionDelay, dualPrimaryConfig.DetectionDelay))
	}

	// Check recovery delay if specified
	if t.RecoveryDelay != nil && dualPrimaryConfig.RecoveryDelay != *t.RecoveryDelay {
		failures = append(failures, fmt.Sprintf("Dual-primary recovery delay: expected %d, got %d", *t.RecoveryDelay, dualPrimaryConfig.RecoveryDelay))
	}

	// Check errdisabled setting if specified
	if t.Errdisabled != nil && dualPrimaryConfig.Errdisabled != *t.Errdisabled {
		failures = append(failures, fmt.Sprintf("Dual-primary errdisabled: expected %v, got %v", *t.Errdisabled, dualPrimaryConfig.Errdisabled))
	}

	// Check primary priority if specified
	if t.PrimaryPriority != nil && dualPrimaryConfig.PrimaryPriority != *t.PrimaryPriority {
		failures = append(failures, fmt.Sprintf("Primary priority: expected %d, got %d", *t.PrimaryPriority, dualPrimaryConfig.PrimaryPriority))
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("MLAG dual-primary failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyMlagDualPrimary) ValidateInput(input any) error {
	if t.DetectionDelay != nil && *t.DetectionDelay < 0 {
		return fmt.Errorf("detection_delay must be non-negative")
	}
	if t.RecoveryDelay != nil && *t.RecoveryDelay < 0 {
		return fmt.Errorf("recovery_delay must be non-negative")
	}
	if t.PrimaryPriority != nil && (*t.PrimaryPriority < 0 || *t.PrimaryPriority > 65535) {
		return fmt.Errorf("primary_priority must be between 0 and 65535")
	}
	return nil
}

// Supporting data structures

type MlagInfo struct {
	State              string
	NegotiationStatus  string
	LocalIntfStatus    string
	PeerLinkStatus     string
}

type MlagInterfaceInfo struct {
	Name   string
	Status string
	State  string
}

type DualPrimaryConfig struct {
	DetectionDelay  int
	RecoveryDelay   int
	Errdisabled     bool
	PrimaryPriority int
}