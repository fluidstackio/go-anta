package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/platform"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyTransceivers verifies the health and compliance of optical transceivers.
//
// This test performs the following checks for each transceiver:
//  1. Validates manufacturer compliance against an approved list if specified.
//  2. Monitors transceiver temperature against high alarm and warning thresholds.
//  3. Monitors supply voltage against high and low alarm thresholds.
//  4. Checks receive power levels for adequate signal strength.
//
// Expected Results:
//   - Success: All transceivers pass health checks and compliance requirements.
//   - Failure: Transceivers exceed temperature/voltage thresholds, have low RX power, or use unauthorized manufacturers.
//   - Error: Unable to retrieve transceiver inventory or health data.
//
// Example YAML configuration:
//   - name: "VerifyTransceivers"
//     module: "hardware"
//     inputs:
//       check_manufacturer: true
//       manufacturers: ["Arista Networks", "Finisar"]
//       check_temperature: true
//       check_voltage: true
type VerifyTransceivers struct {
	test.BaseTest
	CheckManufacturer bool     `yaml:"check_manufacturer" json:"check_manufacturer"`
	Manufacturers     []string `yaml:"manufacturers,omitempty" json:"manufacturers,omitempty"`
	CheckTemperature  bool     `yaml:"check_temperature,omitempty" json:"check_temperature,omitempty"`
	CheckVoltage      bool     `yaml:"check_voltage,omitempty" json:"check_voltage,omitempty"`
}

func NewVerifyTransceivers(inputs map[string]any) (test.Test, error) {
	t := &VerifyTransceivers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTransceivers",
			TestDescription: "Verify transceiver inventory and health",
			TestCategories:  []string{"hardware", "optics"},
		},
		CheckManufacturer: true,
		CheckTemperature:  true,
		CheckVoltage:      true,
	}

	if inputs != nil {
		if check, ok := inputs["check_manufacturer"].(bool); ok {
			t.CheckManufacturer = check
		}
		if manufacturers, ok := inputs["manufacturers"].([]any); ok {
			for _, m := range manufacturers {
				if mfg, ok := m.(string); ok {
					t.Manufacturers = append(t.Manufacturers, mfg)
				}
			}
		}
		if check, ok := inputs["check_temperature"].(bool); ok {
			t.CheckTemperature = check
		}
		if check, ok := inputs["check_voltage"].(bool); ok {
			t.CheckVoltage = check
		}
	}

	return t, nil
}

func (t *VerifyTransceivers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical transceivers are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical transceivers are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show interfaces transceiver",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get transceiver data: %v", err)
		return result, nil
	}

	issues := []string{}
	
	if transceiverData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := transceiverData["interfaces"].(map[string]any); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]any); ok {
					if err := t.validateTransceiver(intfName, intf, &issues); err != nil {
						issues = append(issues, fmt.Sprintf("%s: %v", intfName, err))
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Transceiver issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyTransceivers) validateTransceiver(intfName string, data map[string]any, issues *[]string) error {
	if t.CheckManufacturer && len(t.Manufacturers) > 0 {
		if vendor, ok := data["vendorName"].(string); ok {
			valid := false
			for _, allowedMfg := range t.Manufacturers {
				if strings.Contains(strings.ToLower(vendor), strings.ToLower(allowedMfg)) {
					valid = true
					break
				}
			}
			if !valid {
				*issues = append(*issues, fmt.Sprintf("%s: unauthorized manufacturer '%s'", intfName, vendor))
			}
		}
	}

	if details, ok := data["details"].(map[string]any); ok {
		if t.CheckTemperature {
			if tempData, ok := details["temperature"].(map[string]any); ok {
				if temp, ok := tempData["temp"].(float64); ok {
					if highAlarm, ok := tempData["highAlarm"].(float64); ok {
						if temp >= highAlarm {
							*issues = append(*issues, fmt.Sprintf("%s: temperature %.1f°C exceeds high alarm %.1f°C", 
								intfName, temp, highAlarm))
						}
					}
					if highWarn, ok := tempData["highWarn"].(float64); ok {
						if temp >= highWarn {
							*issues = append(*issues, fmt.Sprintf("%s: temperature %.1f°C exceeds high warning %.1f°C",
								intfName, temp, highWarn))
						}
					}
				}
			}
		}

		if t.CheckVoltage {
			if voltageData, ok := details["voltage"].(map[string]any); ok {
				if voltage, ok := voltageData["voltage"].(float64); ok {
					if highAlarm, ok := voltageData["highAlarm"].(float64); ok {
						if voltage >= highAlarm {
							*issues = append(*issues, fmt.Sprintf("%s: voltage %.2fV exceeds high alarm %.2fV",
								intfName, voltage, highAlarm))
						}
					}
					if lowAlarm, ok := voltageData["lowAlarm"].(float64); ok {
						if voltage <= lowAlarm {
							*issues = append(*issues, fmt.Sprintf("%s: voltage %.2fV below low alarm %.2fV",
								intfName, voltage, lowAlarm))
						}
					}
				}
			}
		}

		if lanes, ok := details["lanes"].([]any); ok {
			for i, lane := range lanes {
				if laneData, ok := lane.(map[string]any); ok {
					if t.CheckTemperature {
						if rxPower, ok := laneData["rxPower"].(float64); ok {
							if rxPower < -30 {
								*issues = append(*issues, fmt.Sprintf("%s lane %d: low RX power %.2f dBm", intfName, i, rxPower))
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (t *VerifyTransceivers) ValidateInput(input any) error {
	return nil
}

// VerifyTransceiversManufacturers verifies if all transceivers come from approved manufacturers.
//
// This test validates that all optical transceivers in the device are sourced from
// manufacturers on an approved list, ensuring compliance with organizational standards
// and avoiding potential compatibility or support issues.
//
// The test performs the following checks:
//   1. Retrieves the complete transceiver inventory from the device.
//   2. Extracts manufacturer information for each transceiver.
//   3. Validates each manufacturer against the approved list.
//   4. Reports any transceivers from unauthorized manufacturers.
//
// Expected Results:
//   - Success: All transceivers are from approved manufacturers.
//   - Failure: One or more transceivers are from unauthorized manufacturers.
//   - Error: Unable to retrieve transceiver inventory or manufacturer data.
//
// Examples:
//   - name: VerifyTransceiversManufacturers strict validation
//     VerifyTransceiversManufacturers:
//       manufacturers: ["Arista Networks", "Finisar", "JDSU"]
//
//   - name: VerifyTransceiversManufacturers with additional vendors
//     VerifyTransceiversManufacturers:
//       manufacturers: ["Arista Networks", "Finisar", "JDSU", "Mellanox", "Intel"]
type VerifyTransceiversManufacturers struct {
	test.BaseTest
	Manufacturers []string `yaml:"manufacturers" json:"manufacturers"`
}

func NewVerifyTransceiversManufacturers(inputs map[string]any) (test.Test, error) {
	t := &VerifyTransceiversManufacturers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTransceiversManufacturers",
			TestDescription: "Verify all transceivers come from approved manufacturers",
			TestCategories:  []string{"hardware", "optics"},
		},
	}

	if inputs != nil {
		if manufacturers, ok := inputs["manufacturers"].([]any); ok {
			for _, mfg := range manufacturers {
				if mfgStr, ok := mfg.(string); ok {
					t.Manufacturers = append(t.Manufacturers, mfgStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyTransceiversManufacturers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical transceivers are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical transceivers are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show interfaces transceiver",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get transceiver data: %v", err)
		return result, nil
	}

	unauthorizedTransceivers := []string{}

	if transceiverData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := transceiverData["interfaces"].(map[string]any); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]any); ok {
					if mfgInfo, ok := intf["vendorName"].(string); ok {
						if !t.isApprovedManufacturer(mfgInfo) {
							unauthorizedTransceivers = append(unauthorizedTransceivers, fmt.Sprintf("%s: %s", intfName, mfgInfo))
						}
					} else if mfgInfo, ok := intf["manufacturer"].(string); ok {
						if !t.isApprovedManufacturer(mfgInfo) {
							unauthorizedTransceivers = append(unauthorizedTransceivers, fmt.Sprintf("%s: %s", intfName, mfgInfo))
						}
					}
				}
			}
		}
	}

	if len(unauthorizedTransceivers) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Unauthorized transceiver manufacturers found: %v", unauthorizedTransceivers)
	}

	return result, nil
}

func (t *VerifyTransceiversManufacturers) isApprovedManufacturer(manufacturer string) bool {
	for _, approved := range t.Manufacturers {
		if strings.EqualFold(manufacturer, approved) {
			return true
		}
	}
	return false
}

func (t *VerifyTransceiversManufacturers) ValidateInput(input any) error {
	if len(t.Manufacturers) == 0 {
		return fmt.Errorf("at least one approved manufacturer must be specified")
	}
	return nil
}

// VerifyTransceiversTemperature verifies if all transceivers are operating at acceptable temperatures.
//
// This test monitors transceiver temperature sensors to ensure they are operating within
// safe thermal limits. High temperatures can indicate poor ventilation, component aging,
// or environmental issues that could lead to performance degradation or failure.
//
// The test performs the following checks:
//   1. Retrieves temperature data for all transceivers.
//   2. Compares current temperatures against high alarm and warning thresholds.
//   3. Applies configurable margin to warning thresholds for early detection.
//   4. Reports transceivers approaching or exceeding thermal limits.
//
// Expected Results:
//   - Success: All transceivers are operating within acceptable temperature ranges.
//   - Failure: One or more transceivers exceed temperature thresholds or approach thermal limits.
//   - Error: Unable to retrieve transceiver temperature data.
//
// Examples:
//   - name: VerifyTransceiversTemperature standard check
//     VerifyTransceiversTemperature: {}
//
//   - name: VerifyTransceiversTemperature with custom margin
//     VerifyTransceiversTemperature:
//       temp_warning_margin: 10.0  # degrees Celsius margin before warning threshold
type VerifyTransceiversTemperature struct {
	test.BaseTest
	TempWarningMargin float64 `yaml:"temp_warning_margin,omitempty" json:"temp_warning_margin,omitempty"`
}

func NewVerifyTransceiversTemperature(inputs map[string]any) (test.Test, error) {
	t := &VerifyTransceiversTemperature{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTransceiversTemperature",
			TestDescription: "Verify all transceivers are operating at acceptable temperatures",
			TestCategories:  []string{"hardware", "optics", "environmental"},
		},
		TempWarningMargin: 5.0, // Default 5°C margin
	}

	if inputs != nil {
		if margin, ok := inputs["temp_warning_margin"].(float64); ok {
			t.TempWarningMargin = margin
		}
	}

	return t, nil
}

func (t *VerifyTransceiversTemperature) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical transceivers are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical transceivers are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show interfaces transceiver temperature",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get transceiver temperature data: %v", err)
		return result, nil
	}

	temperatureIssues := []string{}

	if tempData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := tempData["interfaces"].(map[string]any); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]any); ok {
					if err := t.checkTransceiverTemperature(intfName, intf, &temperatureIssues); err != nil {
						temperatureIssues = append(temperatureIssues, fmt.Sprintf("%s: %v", intfName, err))
					}
				}
			}
		}
	}

	if len(temperatureIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Transceiver temperature issues: %v", temperatureIssues)
	}

	return result, nil
}

func (t *VerifyTransceiversTemperature) checkTransceiverTemperature(intfName string, data map[string]any, issues *[]string) error {
	// Try different possible field names for temperature data
	tempFields := []string{"temperature", "temp", "currentTemp"}
	alarmFields := []string{"highAlarmThreshold", "tempHighAlarm", "highAlarm"}
	warningFields := []string{"highWarningThreshold", "tempHighWarning", "highWarning"}

	var currentTemp, highAlarm, highWarning float64
	var tempFound, alarmFound, warningFound bool

	// Extract current temperature
	for _, field := range tempFields {
		if temp, ok := data[field].(float64); ok {
			currentTemp = temp
			tempFound = true
			break
		}
	}

	// Extract high alarm threshold
	for _, field := range alarmFields {
		if alarm, ok := data[field].(float64); ok {
			highAlarm = alarm
			alarmFound = true
			break
		}
	}

	// Extract high warning threshold
	for _, field := range warningFields {
		if warning, ok := data[field].(float64); ok {
			highWarning = warning
			warningFound = true
			break
		}
	}

	if !tempFound {
		return fmt.Errorf("temperature data not available")
	}

	// Check against high alarm threshold
	if alarmFound && currentTemp >= highAlarm {
		*issues = append(*issues, fmt.Sprintf("%s: temperature %.1f°C exceeds high alarm threshold %.1f°C", intfName, currentTemp, highAlarm))
	}

	// Check against high warning threshold with margin
	if warningFound {
		warningWithMargin := highWarning - t.TempWarningMargin
		if currentTemp >= warningWithMargin {
			*issues = append(*issues, fmt.Sprintf("%s: temperature %.1f°C approaching warning threshold %.1f°C (margin: %.1f°C)", intfName, currentTemp, highWarning, t.TempWarningMargin))
		}
	}

	return nil
}

func (t *VerifyTransceiversTemperature) ValidateInput(input any) error {
	if t.TempWarningMargin < 0 {
		return fmt.Errorf("temperature warning margin cannot be negative")
	}
	return nil
}

