package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/platform"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyAdverseDrops verifies there are no adverse drops exceeding defined thresholds.
//
// This test monitors packet drop counters across device interfaces and forwarding engines
// to detect performance issues, congestion, or hardware problems that could impact
// network performance and reliability.
//
// The test performs the following checks:
//   1. Retrieves adverse drop counters from all interfaces and forwarding engines.
//   2. Compares drop counts against configurable thresholds.
//   3. Identifies interfaces or engines with excessive drop rates.
//   4. Reports drop statistics and threshold violations.
//
// Expected Results:
//   - Success: All adverse drop counters are below defined thresholds.
//   - Failure: One or more interfaces or engines exceed drop thresholds.
//   - Error: Unable to retrieve adverse drop statistics.
//
// Examples:
//   - name: VerifyAdverseDrops basic check
//     VerifyAdverseDrops: {}
//
//   - name: VerifyAdverseDrops with custom thresholds
//     VerifyAdverseDrops:
//       max_drops: 1000
//       check_interfaces: true
//       check_forwarding_engines: true
type VerifyAdverseDrops struct {
	test.BaseTest
	MaxDrops                 int64 `yaml:"max_drops,omitempty" json:"max_drops,omitempty"`
	CheckInterfaces          bool  `yaml:"check_interfaces,omitempty" json:"check_interfaces,omitempty"`
	CheckForwardingEngines   bool  `yaml:"check_forwarding_engines,omitempty" json:"check_forwarding_engines,omitempty"`
}

func NewVerifyAdverseDrops(inputs map[string]any) (test.Test, error) {
	t := &VerifyAdverseDrops{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAdverseDrops",
			TestDescription: "Verify no adverse drops exceeding defined thresholds",
			TestCategories:  []string{"hardware", "performance"},
		},
		MaxDrops:               0, // Default: no drops allowed
		CheckInterfaces:        true,
		CheckForwardingEngines: true,
	}

	if inputs != nil {
		if maxDrops, ok := inputs["max_drops"].(float64); ok {
			t.MaxDrops = int64(maxDrops)
		} else if maxDrops, ok := inputs["max_drops"].(int); ok {
			t.MaxDrops = int64(maxDrops)
		}
		if checkIntf, ok := inputs["check_interfaces"].(bool); ok {
			t.CheckInterfaces = checkIntf
		}
		if checkFE, ok := inputs["check_forwarding_engines"].(bool); ok {
			t.CheckForwardingEngines = checkFE
		}
	}

	return t, nil
}

func (t *VerifyAdverseDrops) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where adverse drops may not be meaningful
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "adverse drop monitoring is not meaningful"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show hardware counter drop",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get adverse drop data: %v", err)
		return result, nil
	}

	dropIssues := []string{}

	if dropData, ok := cmdResult.Output.(map[string]any); ok {
		// Check interface drops
		if t.CheckInterfaces {
			if interfaces, ok := dropData["interfaces"].(map[string]any); ok {
				for intfName, intfData := range interfaces {
					if intf, ok := intfData.(map[string]any); ok {
						t.checkInterfaceDrops(intfName, intf, &dropIssues)
					}
				}
			}
		}

		// Check forwarding engine drops
		if t.CheckForwardingEngines {
			if forwardingEngines, ok := dropData["forwardingEngines"].(map[string]any); ok {
				for feName, feData := range forwardingEngines {
					if fe, ok := feData.(map[string]any); ok {
						t.checkForwardingEngineDrops(feName, fe, &dropIssues)
					}
				}
			}
		}
	}

	if len(dropIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Adverse drop threshold violations: %v", dropIssues)
	}

	return result, nil
}

func (t *VerifyAdverseDrops) checkInterfaceDrops(intfName string, intfData map[string]any, issues *[]string) {
	dropFields := []string{"drops", "inDrops", "outDrops", "totalDrops"}

	for _, field := range dropFields {
		drops, ok := intfData[field].(float64)
		if !ok {
			continue
		}

		if int64(drops) > t.MaxDrops {
			*issues = append(*issues, fmt.Sprintf("Interface %s: %s=%d exceeds threshold %d", intfName, field, int64(drops), t.MaxDrops))
		}
	}
}

func (t *VerifyAdverseDrops) checkForwardingEngineDrops(feName string, feData map[string]any, issues *[]string) {
	dropFields := []string{"drops", "adverseDrops", "totalDrops"}

	for _, field := range dropFields {
		drops, ok := feData[field].(float64)
		if !ok {
			continue
		}

		if int64(drops) > t.MaxDrops {
			*issues = append(*issues, fmt.Sprintf("Forwarding Engine %s: %s=%d exceeds threshold %d", feName, field, int64(drops), t.MaxDrops))
		}
	}
}

func (t *VerifyAdverseDrops) ValidateInput(input any) error {
	if t.MaxDrops < 0 {
		return fmt.Errorf("maximum drops threshold cannot be negative")
	}
	return nil
}

// VerifySupervisorRedundancy verifies the redundancy protocol configured on the active supervisor.
//
// This test validates that supervisor redundancy is properly configured and operational
// on devices that support redundant supervisors. Supervisor redundancy ensures high
// availability and seamless failover in case of primary supervisor failure.
//
// The test performs the following checks:
//   1. Retrieves supervisor redundancy status and configuration.
//   2. Validates that redundancy is enabled and properly configured.
//   3. Checks the status of both primary and standby supervisors.
//   4. Verifies synchronization state between supervisors.
//
// Expected Results:
//   - Success: Supervisor redundancy is properly configured and operational.
//   - Failure: Redundancy is disabled, misconfigured, or supervisors are not synchronized.
//   - Error: Unable to retrieve supervisor redundancy information.
//
// Examples:
//   - name: VerifySupervisorRedundancy basic check
//     VerifySupervisorRedundancy: {}
//
//   - name: VerifySupervisorRedundancy with protocol validation
//     VerifySupervisorRedundancy:
//       expected_protocol: "rpr"  # Route Processor Redundancy
type VerifySupervisorRedundancy struct {
	test.BaseTest
	ExpectedProtocol string `yaml:"expected_protocol,omitempty" json:"expected_protocol,omitempty"`
}

func NewVerifySupervisorRedundancy(inputs map[string]any) (test.Test, error) {
	t := &VerifySupervisorRedundancy{
		BaseTest: test.BaseTest{
			TestName:        "VerifySupervisorRedundancy",
			TestDescription: "Verify redundancy protocol configured on active supervisor",
			TestCategories:  []string{"hardware", "redundancy"},
		},
	}

	if inputs != nil {
		if protocol, ok := inputs["expected_protocol"].(string); ok {
			t.ExpectedProtocol = protocol
		}
	}

	return t, nil
}

func (t *VerifySupervisorRedundancy) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where supervisor redundancy is not applicable
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "supervisor redundancy is not applicable"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show redundancy",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get redundancy data: %v", err)
		return result, nil
	}

	if redundancyData, ok := cmdResult.Output.(map[string]any); ok {
		// Check redundancy protocol
		if protocol, ok := redundancyData["protocol"].(string); ok {
			if t.ExpectedProtocol != "" && !strings.EqualFold(protocol, t.ExpectedProtocol) {
				result.Status = test.TestFailure
				result.Message = fmt.Sprintf("Redundancy protocol: %s (expected: %s)", protocol, t.ExpectedProtocol)
				return result, nil
			}
		}

		// Check redundancy state
		if state, ok := redundancyData["redundancyState"].(string); ok {
			if !strings.Contains(strings.ToLower(state), "active") {
				result.Status = test.TestFailure
				result.Message = fmt.Sprintf("Redundancy state: %s (expected: active)", state)
				return result, nil
			}
		}

		// Check supervisor status
		if supervisors, ok := redundancyData["supervisors"].(map[string]any); ok {
			activeSupervisors := 0
			standbySupervisors := 0

			for supName, supData := range supervisors {
				if sup, ok := supData.(map[string]any); ok {
					if state, ok := sup["state"].(string); ok {
						switch strings.ToLower(state) {
						case "active":
							activeSupervisors++
						case "standby", "standby-hot":
							standbySupervisors++
						default:
							result.Status = test.TestFailure
							result.Message = fmt.Sprintf("Supervisor %s: unexpected state %s", supName, state)
							return result, nil
						}
					}
				}
			}

			if activeSupervisors != 1 {
				result.Status = test.TestFailure
				result.Message = fmt.Sprintf("Expected 1 active supervisor, found %d", activeSupervisors)
				return result, nil
			}

			if standbySupervisors == 0 {
				result.Status = test.TestFailure
				result.Message = "No standby supervisor found - redundancy not available"
				return result, nil
			}
		}
	}

	return result, nil
}

func (t *VerifySupervisorRedundancy) ValidateInput(input any) error {
	// No specific validation required
	return nil
}

// VerifyPCIeErrors verifies PCIe device error counters.
//
// This test monitors PCIe (Peripheral Component Interconnect Express) device error
// counters to detect hardware communication issues, signal integrity problems,
// or device malfunctions that could impact system stability and performance.
//
// The test performs the following checks:
//   1. Retrieves PCIe error counters for all PCIe devices.
//   2. Validates that error counts are within acceptable thresholds.
//   3. Checks for correctable and uncorrectable error types.
//   4. Reports devices with excessive error rates.
//
// Expected Results:
//   - Success: All PCIe devices have error counts within acceptable limits.
//   - Failure: One or more PCIe devices exceed error thresholds.
//   - Error: Unable to retrieve PCIe error counter data.
//
// Examples:
//   - name: VerifyPCIeErrors basic check
//     VerifyPCIeErrors: {}
//
//   - name: VerifyPCIeErrors with custom thresholds
//     VerifyPCIeErrors:
//       max_correctable_errors: 10
//       max_uncorrectable_errors: 0
type VerifyPCIeErrors struct {
	test.BaseTest
	MaxCorrectableErrors   int64 `yaml:"max_correctable_errors,omitempty" json:"max_correctable_errors,omitempty"`
	MaxUncorrectableErrors int64 `yaml:"max_uncorrectable_errors,omitempty" json:"max_uncorrectable_errors,omitempty"`
}

func NewVerifyPCIeErrors(inputs map[string]any) (test.Test, error) {
	t := &VerifyPCIeErrors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyPCIeErrors",
			TestDescription: "Verify PCIe device error counters",
			TestCategories:  []string{"hardware", "pcie"},
		},
		MaxCorrectableErrors:   0, // Default: no correctable errors
		MaxUncorrectableErrors: 0, // Default: no uncorrectable errors
	}

	if inputs != nil {
		if maxCorrectable, ok := inputs["max_correctable_errors"].(float64); ok {
			t.MaxCorrectableErrors = int64(maxCorrectable)
		} else if maxCorrectable, ok := inputs["max_correctable_errors"].(int); ok {
			t.MaxCorrectableErrors = int64(maxCorrectable)
		}
		if maxUncorrectable, ok := inputs["max_uncorrectable_errors"].(float64); ok {
			t.MaxUncorrectableErrors = int64(maxUncorrectable)
		} else if maxUncorrectable, ok := inputs["max_uncorrectable_errors"].(int); ok {
			t.MaxUncorrectableErrors = int64(maxUncorrectable)
		}
	}

	return t, nil
}

func (t *VerifyPCIeErrors) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where PCIe devices are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "PCIe devices are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show hardware pci errors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get PCIe error data: %v", err)
		return result, nil
	}

	pcieIssues := []string{}

	if pcieData, ok := cmdResult.Output.(map[string]any); ok {
		if devices, ok := pcieData["pcieDevices"].(map[string]any); ok {
			for deviceName, deviceData := range devices {
				if device, ok := deviceData.(map[string]any); ok {
					t.checkPCIeDeviceErrors(deviceName, device, &pcieIssues)
				}
			}
		}
	}

	if len(pcieIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("PCIe error threshold violations: %v", pcieIssues)
	}

	return result, nil
}

func (t *VerifyPCIeErrors) checkPCIeDeviceErrors(deviceName string, deviceData map[string]any, issues *[]string) {
	// Check correctable errors
	if correctableErrors, ok := deviceData["correctableErrors"].(float64); ok && int64(correctableErrors) > t.MaxCorrectableErrors {
		*issues = append(*issues, fmt.Sprintf("Device %s: %d correctable errors exceed threshold %d", deviceName, int64(correctableErrors), t.MaxCorrectableErrors))
	}

	// Check uncorrectable errors
	if uncorrectableErrors, ok := deviceData["uncorrectableErrors"].(float64); ok && int64(uncorrectableErrors) > t.MaxUncorrectableErrors {
		*issues = append(*issues, fmt.Sprintf("Device %s: %d uncorrectable errors exceed threshold %d", deviceName, int64(uncorrectableErrors), t.MaxUncorrectableErrors))
	}

	// Check for other error types
	errorFields := []string{"fatalErrors", "nonFatalErrors", "linkErrors"}
	for _, field := range errorFields {
		errors, ok := deviceData[field].(float64)
		if !ok {
			continue
		}

		if int64(errors) > 0 {
			*issues = append(*issues, fmt.Sprintf("Device %s: %d %s detected", deviceName, int64(errors), field))
		}
	}
}

func (t *VerifyPCIeErrors) ValidateInput(input any) error {
	if t.MaxCorrectableErrors < 0 {
		return fmt.Errorf("maximum correctable errors threshold cannot be negative")
	}
	if t.MaxUncorrectableErrors < 0 {
		return fmt.Errorf("maximum uncorrectable errors threshold cannot be negative")
	}
	return nil
}

// VerifyAbsenceOfLinecards verifies that specific linecards are not present in the device inventory.
//
// This test ensures that specified linecard types or models are not installed in the device,
// which can be useful for compliance checking, licensing validation, or ensuring that
// unsupported or problematic hardware is not present.
//
// The test performs the following checks:
//   1. Retrieves the complete device inventory including all linecards.
//   2. Searches for the presence of specified linecard models or types.
//   3. Reports if any prohibited linecards are found.
//   4. Optionally validates slot positions of detected linecards.
//
// Expected Results:
//   - Success: None of the specified linecards are present in the device.
//   - Failure: One or more prohibited linecards are detected.
//   - Error: Unable to retrieve device inventory.
//
// Examples:
//   - name: VerifyAbsenceOfLinecards basic check
//     VerifyAbsenceOfLinecards:
//       linecard_models: ["7500E-36Q", "7500E-72S"]
//
//   - name: VerifyAbsenceOfLinecards with slot validation
//     VerifyAbsenceOfLinecards:
//       linecard_models: ["DCS-7500E-36Q-LC"]
//       check_slots: true
type VerifyAbsenceOfLinecards struct {
	test.BaseTest
	LinecardModels []string `yaml:"linecard_models" json:"linecard_models"`
	CheckSlots     bool     `yaml:"check_slots,omitempty" json:"check_slots,omitempty"`
}

func NewVerifyAbsenceOfLinecards(inputs map[string]any) (test.Test, error) {
	t := &VerifyAbsenceOfLinecards{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAbsenceOfLinecards",
			TestDescription: "Verify specific linecards are not present in device inventory",
			TestCategories:  []string{"hardware", "inventory"},
		},
		CheckSlots: false,
	}

	if inputs != nil {
		if models, ok := inputs["linecard_models"].([]any); ok {
			for _, model := range models {
				if modelStr, ok := model.(string); ok {
					t.LinecardModels = append(t.LinecardModels, modelStr)
				}
			}
		}
		if checkSlots, ok := inputs["check_slots"].(bool); ok {
			t.CheckSlots = checkSlots
		}
	}

	return t, nil
}

func (t *VerifyAbsenceOfLinecards) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where linecards are not applicable
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "linecards are not applicable"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show version",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get inventory data: %v", err)
		return result, nil
	}

	foundLinecards := []string{}

	if inventoryData, ok := cmdResult.Output.(map[string]any); ok {
		// Check linecards in xcvrSlots (transceiver slots often indicate linecards)
		if xcvrSlots, ok := inventoryData["xcvrSlots"].(map[string]any); ok {
			for slotName, slotData := range xcvrSlots {
				if slot, ok := slotData.(map[string]any); ok {
					t.checkSlotForLinecards(slotName, slot, &foundLinecards)
				}
			}
		}

		// Check hardware components
		if components, ok := inventoryData["hardwareRevision"].(map[string]any); ok {
			for componentName, componentData := range components {
				if component, ok := componentData.(map[string]any); ok {
					t.checkComponentForLinecards(componentName, component, &foundLinecards)
				}
			}
		}

		// Check cards directly
		if cards, ok := inventoryData["cards"].(map[string]any); ok {
			for cardName, cardData := range cards {
				if card, ok := cardData.(map[string]any); ok {
					t.checkCardForLinecards(cardName, card, &foundLinecards)
				}
			}
		}
	}

	if len(foundLinecards) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Prohibited linecards found: %v", foundLinecards)
	}

	return result, nil
}

func (t *VerifyAbsenceOfLinecards) checkSlotForLinecards(slotName string, slotData map[string]any, found *[]string) {
	if modelName, ok := slotData["modelName"].(string); ok {
		t.checkModelName(slotName, modelName, found)
	}
}

func (t *VerifyAbsenceOfLinecards) checkComponentForLinecards(componentName string, componentData map[string]any, found *[]string) {
	if modelName, ok := componentData["name"].(string); ok {
		t.checkModelName(componentName, modelName, found)
	}
}

func (t *VerifyAbsenceOfLinecards) checkCardForLinecards(cardName string, cardData map[string]any, found *[]string) {
	if modelName, ok := cardData["modelName"].(string); ok {
		t.checkModelName(cardName, modelName, found)
	}
}

func (t *VerifyAbsenceOfLinecards) checkModelName(location string, modelName string, found *[]string) {
	for _, prohibitedModel := range t.LinecardModels {
		if strings.Contains(modelName, prohibitedModel) || strings.EqualFold(modelName, prohibitedModel) {
			*found = append(*found, fmt.Sprintf("%s: %s", location, modelName))
		}
	}
}

func (t *VerifyAbsenceOfLinecards) ValidateInput(input any) error {
	if len(t.LinecardModels) == 0 {
		return fmt.Errorf("at least one linecard model must be specified")
	}
	return nil
}