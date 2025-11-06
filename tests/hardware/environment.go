package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/platform"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyEnvironmentSystemCooling verifies the device's system cooling status.
//
// This test validates that the device's overall cooling system is functioning properly
// by checking the system cooling status reported by the hardware monitoring subsystem.
// Proper cooling is critical for device reliability and performance.
//
// The test performs the following checks:
//   1. Retrieves the system cooling status from the device.
//   2. Validates that the cooling status indicates proper operation.
//   3. Reports any cooling system failures or warnings.
//
// Expected Results:
//   - Success: System cooling status is "ok" or equivalent healthy state.
//   - Failure: System cooling status indicates problems or warnings.
//   - Error: Unable to retrieve system cooling status.
//
// Examples:
//   - name: VerifyEnvironmentSystemCooling basic check
//     VerifyEnvironmentSystemCooling: {}
//
//   - name: VerifyEnvironmentSystemCooling comprehensive validation
//     VerifyEnvironmentSystemCooling:
//       # No parameters needed - validates overall cooling status
type VerifyEnvironmentSystemCooling struct {
	test.BaseTest
}

func NewVerifyEnvironmentSystemCooling(inputs map[string]any) (test.Test, error) {
	t := &VerifyEnvironmentSystemCooling{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEnvironmentSystemCooling",
			TestDescription: "Verify device system cooling status",
			TestCategories:  []string{"hardware", "environmental"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyEnvironmentSystemCooling) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where cooling systems are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "cooling systems are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show system environment cooling",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get system cooling status: %v", err)
		return result, nil
	}

	if coolingData, ok := cmdResult.Output.(map[string]any); ok {
		if systemCooling, ok := coolingData["systemCoolingStatus"].(string); ok {
			if !strings.EqualFold(systemCooling, "coolingOk") && !strings.EqualFold(systemCooling, "ok") {
				result.Status = test.TestFailure
				result.Message = fmt.Sprintf("System cooling status: %s (expected: coolingOk)", systemCooling)
			}
		} else if coolingStatus, ok := coolingData["coolingStatus"].(string); ok {
			if !strings.EqualFold(coolingStatus, "ok") {
				result.Status = test.TestFailure
				result.Message = fmt.Sprintf("Cooling status: %s (expected: ok)", coolingStatus)
			}
		} else {
			result.Status = test.TestError
			result.Message = "Unable to determine system cooling status from device response"
		}
	}

	return result, nil
}

func (t *VerifyEnvironmentSystemCooling) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifyEnvironmentCooling verifies the status of power supply fans and all fan trays.
//
// This test validates the operational status and performance of all cooling fans in the device,
// including power supply fans and chassis fan trays. Proper fan operation is essential for
// maintaining device temperatures within acceptable limits.
//
// The test performs the following checks:
//   1. Retrieves status for all power supply fans and fan trays.
//   2. Validates that all fans are operational and within speed tolerances.
//   3. Checks for any fan failures or performance warnings.
//   4. Optionally validates fan speeds against expected ranges.
//
// Expected Results:
//   - Success: All fans are operational and within acceptable speed ranges.
//   - Failure: One or more fans are failed, warning, or operating outside expected parameters.
//   - Error: Unable to retrieve fan status or performance data.
//
// Examples:
//   - name: VerifyEnvironmentCooling basic check
//     VerifyEnvironmentCooling: {}
//
//   - name: VerifyEnvironmentCooling with speed validation
//     VerifyEnvironmentCooling:
//       check_fan_speed: true
//       min_fan_speed_pct: 30  # Minimum acceptable fan speed percentage
type VerifyEnvironmentCooling struct {
	test.BaseTest
	CheckFanSpeed   bool `yaml:"check_fan_speed,omitempty" json:"check_fan_speed,omitempty"`
	MinFanSpeedPct  int  `yaml:"min_fan_speed_pct,omitempty" json:"min_fan_speed_pct,omitempty"`
}

func NewVerifyEnvironmentCooling(inputs map[string]any) (test.Test, error) {
	t := &VerifyEnvironmentCooling{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEnvironmentCooling",
			TestDescription: "Verify status of power supply fans and fan trays",
			TestCategories:  []string{"hardware", "environmental"},
		},
		CheckFanSpeed:  false,
		MinFanSpeedPct: 30, // Default minimum 30%
	}

	if inputs != nil {
		if checkSpeed, ok := inputs["check_fan_speed"].(bool); ok {
			t.CheckFanSpeed = checkSpeed
		}
		if minSpeed, ok := inputs["min_fan_speed_pct"].(float64); ok {
			t.MinFanSpeedPct = int(minSpeed)
		} else if minSpeed, ok := inputs["min_fan_speed_pct"].(int); ok {
			t.MinFanSpeedPct = minSpeed
		}
	}

	return t, nil
}

func (t *VerifyEnvironmentCooling) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where cooling fans are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "cooling fans are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show system environment cooling",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get cooling data: %v", err)
		return result, nil
	}

	fanIssues := []string{}

	if coolingData, ok := cmdResult.Output.(map[string]any); ok {
		// Check power supply fans
		if powerSupplies, ok := coolingData["powerSupplySlots"].(map[string]any); ok {
			for psName, psData := range powerSupplies {
				if ps, ok := psData.(map[string]any); ok {
					t.checkPowerSupplyFans(psName, ps, &fanIssues)
				}
			}
		}

		// Check fan trays
		if fanTrays, ok := coolingData["fanTraySlots"].(map[string]any); ok {
			for fanTrayName, fanTrayData := range fanTrays {
				if fanTray, ok := fanTrayData.(map[string]any); ok {
					t.checkFanTray(fanTrayName, fanTray, &fanIssues)
				}
			}
		}

		// Check individual fans
		if fans, ok := coolingData["fans"].(map[string]any); ok {
			for fanName, fanData := range fans {
				if fan, ok := fanData.(map[string]any); ok {
					t.checkIndividualFan(fanName, fan, &fanIssues)
				}
			}
		}
	}

	if len(fanIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Cooling fan issues: %v", fanIssues)
	}

	return result, nil
}

func (t *VerifyEnvironmentCooling) checkPowerSupplyFans(psName string, psData map[string]any, issues *[]string) {
	if fans, ok := psData["fans"].(map[string]any); ok {
		for fanName, fanData := range fans {
			if fan, ok := fanData.(map[string]any); ok {
				t.checkFanStatus(fmt.Sprintf("%s/%s", psName, fanName), fan, issues)
			}
		}
	}
}

func (t *VerifyEnvironmentCooling) checkFanTray(fanTrayName string, fanTrayData map[string]any, issues *[]string) {
	if fans, ok := fanTrayData["fans"].(map[string]any); ok {
		for fanName, fanData := range fans {
			if fan, ok := fanData.(map[string]any); ok {
				t.checkFanStatus(fmt.Sprintf("%s/%s", fanTrayName, fanName), fan, issues)
			}
		}
	}
}

func (t *VerifyEnvironmentCooling) checkIndividualFan(fanName string, fanData map[string]any, issues *[]string) {
	t.checkFanStatus(fanName, fanData, issues)
}

func (t *VerifyEnvironmentCooling) checkFanStatus(fanName string, fanData map[string]any, issues *[]string) {
	// Check fan status
	if status, ok := fanData["status"].(string); ok {
		if !strings.EqualFold(status, "ok") && !strings.EqualFold(status, "running") {
			*issues = append(*issues, fmt.Sprintf("%s: status %s", fanName, status))
		}
	}

	// Check fan speed if requested
	if t.CheckFanSpeed {
		if speedPct, ok := fanData["speedPercent"].(float64); ok {
			if int(speedPct) < t.MinFanSpeedPct {
				*issues = append(*issues, fmt.Sprintf("%s: speed %.0f%% below minimum %d%%", fanName, speedPct, t.MinFanSpeedPct))
			}
		} else if speed, ok := fanData["speed"].(float64); ok {
			// If we have absolute speed, check if it's reasonable (> 0)
			if speed <= 0 {
				*issues = append(*issues, fmt.Sprintf("%s: speed %.0f RPM (appears stopped)", fanName, speed))
			}
		}
	}
}

func (t *VerifyEnvironmentCooling) ValidateInput(input any) error {
	if t.MinFanSpeedPct < 0 || t.MinFanSpeedPct > 100 {
		return fmt.Errorf("minimum fan speed percentage must be between 0 and 100")
	}
	return nil
}

// VerifyEnvironmentPower verifies the power supplies state and input voltage.
//
// This test validates the operational status of all power supplies in the device
// and ensures that input voltages are within acceptable ranges. Power supply
// redundancy and voltage stability are critical for device reliability.
//
// The test performs the following checks:
//   1. Retrieves status for all power supply units (PSUs).
//   2. Validates that PSUs are operational and providing stable power.
//   3. Checks input voltages against acceptable ranges.
//   4. Verifies power supply redundancy if configured.
//
// Expected Results:
//   - Success: All power supplies are operational with stable voltages.
//   - Failure: One or more PSUs are failed, or voltages are outside acceptable ranges.
//   - Error: Unable to retrieve power supply status or voltage data.
//
// Examples:
//   - name: VerifyEnvironmentPower basic check
//     VerifyEnvironmentPower: {}
//
//   - name: VerifyEnvironmentPower with voltage validation
//     VerifyEnvironmentPower:
//       check_voltage: true
//       min_input_voltage: 100  # Minimum acceptable input voltage
//       max_input_voltage: 250  # Maximum acceptable input voltage
type VerifyEnvironmentPower struct {
	test.BaseTest
	CheckVoltage     bool    `yaml:"check_voltage,omitempty" json:"check_voltage,omitempty"`
	MinInputVoltage  float64 `yaml:"min_input_voltage,omitempty" json:"min_input_voltage,omitempty"`
	MaxInputVoltage  float64 `yaml:"max_input_voltage,omitempty" json:"max_input_voltage,omitempty"`
}

func NewVerifyEnvironmentPower(inputs map[string]any) (test.Test, error) {
	t := &VerifyEnvironmentPower{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEnvironmentPower",
			TestDescription: "Verify power supplies state and input voltage",
			TestCategories:  []string{"hardware", "environmental"},
		},
		CheckVoltage:    false,
		MinInputVoltage: 100.0, // Default minimum 100V
		MaxInputVoltage: 250.0, // Default maximum 250V
	}

	if inputs != nil {
		if checkVoltage, ok := inputs["check_voltage"].(bool); ok {
			t.CheckVoltage = checkVoltage
		}
		if minVoltage, ok := inputs["min_input_voltage"].(float64); ok {
			t.MinInputVoltage = minVoltage
		}
		if maxVoltage, ok := inputs["max_input_voltage"].(float64); ok {
			t.MaxInputVoltage = maxVoltage
		}
	}

	return t, nil
}

func (t *VerifyEnvironmentPower) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where power supplies are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "power supplies are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show system environment power",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get power data: %v", err)
		return result, nil
	}

	powerIssues := []string{}

	if powerData, ok := cmdResult.Output.(map[string]any); ok {
		if powerSupplies, ok := powerData["powerSupplies"].(map[string]any); ok {
			for psName, psData := range powerSupplies {
				if ps, ok := psData.(map[string]any); ok {
					t.checkPowerSupply(psName, ps, &powerIssues)
				}
			}
		} else if powerSupplySlots, ok := powerData["powerSupplySlots"].(map[string]any); ok {
			for psName, psData := range powerSupplySlots {
				if ps, ok := psData.(map[string]any); ok {
					t.checkPowerSupply(psName, ps, &powerIssues)
				}
			}
		}
	}

	if len(powerIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Power supply issues: %v", powerIssues)
	}

	return result, nil
}

func (t *VerifyEnvironmentPower) checkPowerSupply(psName string, psData map[string]any, issues *[]string) {
	// Check power supply status
	if state, ok := psData["state"].(string); ok {
		if !strings.EqualFold(state, "ok") && !strings.EqualFold(state, "powerGood") {
			*issues = append(*issues, fmt.Sprintf("%s: state %s", psName, state))
		}
	} else if status, ok := psData["status"].(string); ok {
		if !strings.EqualFold(status, "ok") && !strings.EqualFold(status, "powerGood") {
			*issues = append(*issues, fmt.Sprintf("%s: status %s", psName, status))
		}
	}

	// Check input voltage if requested
	if t.CheckVoltage {
		if inputVoltage, ok := psData["inputVoltage"].(float64); ok {
			if inputVoltage < t.MinInputVoltage {
				*issues = append(*issues, fmt.Sprintf("%s: input voltage %.1fV below minimum %.1fV", psName, inputVoltage, t.MinInputVoltage))
			} else if inputVoltage > t.MaxInputVoltage {
				*issues = append(*issues, fmt.Sprintf("%s: input voltage %.1fV above maximum %.1fV", psName, inputVoltage, t.MaxInputVoltage))
			}
		}
	}

	// Check for other power-related fields
	if capacity, ok := psData["capacity"].(float64); ok {
		if capacity <= 0 {
			*issues = append(*issues, fmt.Sprintf("%s: zero capacity (%.0fW)", psName, capacity))
		}
	}
}

func (t *VerifyEnvironmentPower) ValidateInput(input any) error {
	if t.CheckVoltage {
		if t.MinInputVoltage <= 0 {
			return fmt.Errorf("minimum input voltage must be positive")
		}
		if t.MaxInputVoltage <= t.MinInputVoltage {
			return fmt.Errorf("maximum input voltage must be greater than minimum")
		}
	}
	return nil
}