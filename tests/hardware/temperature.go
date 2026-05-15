package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/platform"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyTemperature verifies that system temperature sensors are within acceptable ranges.
//
// This test performs the following checks:
//  1. Monitors all temperature sensors on the device.
//  2. Checks current temperatures against alert and overheat thresholds.
//  3. Validates system temperature status is "temperatureOk".
//  4. Applies configurable failure margin to overheat thresholds.
//
// Expected Results:
//   - Success: All temperature sensors are within normal operating ranges.
//   - Failure: Temperature sensors exceed alert thresholds or approach overheat limits.
//   - Error: Unable to retrieve temperature sensor data.
//
// Example YAML configuration:
//   - name: "VerifyTemperature"
//     module: "hardware"
//     inputs:
//     check_temp_sensors: true
//     failure_margin: 5.0  # degrees Celsius margin before overheat
type VerifyTemperature struct {
	test.BaseTest
	CheckTempSensors bool    `yaml:"check_temp_sensors" json:"check_temp_sensors"`
	FailureMargin    float64 `yaml:"failure_margin,omitempty" json:"failure_margin,omitempty"`
}

func NewVerifyTemperature(inputs map[string]any) (test.Test, error) {
	t := &VerifyTemperature{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTemperature",
			TestDescription: "Verify system temperature is within acceptable range",
			TestCategories:  []string{"hardware", "environmental"},
		},
		CheckTempSensors: true,
		FailureMargin:    5.0,
	}

	if inputs != nil {
		if check, ok := inputs["check_temp_sensors"].(bool); ok {
			t.CheckTempSensors = check
		}
		if margin, ok := inputs["failure_margin"].(float64); ok {
			t.FailureMargin = margin
		} else if margin, ok := inputs["failure_margin"].(int); ok {
			t.FailureMargin = float64(margin)
		}
	}

	return t, nil
}

// TempSensorReport is the structured record for one temperature
// sensor, surfaced in TestResult.Details. JSON keys mirror what
// pkg/reporter's temperatures block reads.
type TempSensorReport struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Container   string  `json:"container,omitempty"`
	CurrentC    float64 `json:"current_c"`
	MaxC        float64 `json:"max_c,omitempty"`
	OverheatC   float64 `json:"overheat_c,omitempty"`
	CriticalC   float64 `json:"critical_c,omitempty"`
	Status      string  `json:"status"`
	InAlert     bool    `json:"in_alert,omitempty"`
}

func (t *VerifyTemperature) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical temperature sensors are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical temperature sensors are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show system environment temperature",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get temperature data: %v", err)
		return result, nil
	}

	tempData, err := test.AsMap(cmdResult.Output)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Unexpected temperature output: %v", err)
		return result, nil
	}

	var sensors []TempSensorReport
	warnings := []string{}
	alerts := []string{}

	walk := func(container string, raw any) {
		walkContainer(raw, func(_ string, m map[string]any) {
			s := tempSensorRecord(container, m)
			sensors = append(sensors, s)
			t.checkSensor(s, &warnings, &alerts)
		})
	}

	// Top-level chassis sensors.
	walk("chassis", tempData["tempSensors"])
	// PSU-embedded sensors (always present on devices with PSUs).
	walkContainer(tempData["powerSupplySlots"], func(slotName string, slot map[string]any) {
		walk("PSU"+slotName, slot["tempSensors"])
	})
	// Linecard-embedded sensors (chassis platforms).
	walkContainer(tempData["cardSlots"], func(slotName string, slot map[string]any) {
		walk("Card"+slotName, slot["tempSensors"])
	})

	systemStatus, _ := tempData["systemStatus"].(string)
	if systemStatus != "" && systemStatus != "temperatureOk" {
		alerts = append(alerts, fmt.Sprintf("System temperature status: %s", systemStatus))
	}

	details := map[string]any{
		"sensor_count":  len(sensors),
		"temperatures":  sensors,
		"system_status": systemStatus,
	}
	if v, ok := tempData["ambientThreshold"].(float64); ok {
		details["ambient_threshold_c"] = v
	}
	hottest := hottestSensor(sensors)
	if hottest != nil {
		details["hottest_sensor"] = hottest.Name
		details["hottest_c"] = hottest.CurrentC
	}
	if len(alerts) > 0 {
		details["alerts"] = alerts
	}
	if len(warnings) > 0 {
		details["warnings"] = warnings
	}
	if len(alerts)+len(warnings) > 0 {
		issues := append([]string{}, alerts...)
		issues = append(issues, warnings...)
		details["issues"] = issues
	}
	result.Details = details

	switch {
	case len(sensors) == 0:
		result.Status = test.TestError
		result.Message = "No temperature sensors found (unexpected on a physical platform)"
	case len(alerts) > 0:
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Temperature alerts: %v", alerts)
	case len(warnings) > 0:
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Temperature warnings: %v", warnings)
	default:
		msg := fmt.Sprintf("%d sensors, all Ok", len(sensors))
		if hottest != nil {
			msg += fmt.Sprintf(" (hottest: %s at %.1f°C)", hottest.Name, hottest.CurrentC)
		}
		result.Message = msg
	}

	return result, nil
}

// tempSensorRecord captures a single sensor's reading and thresholds.
// Falls back to alternative field names where EOS varies.
func tempSensorRecord(container string, m map[string]any) TempSensorReport {
	s := TempSensorReport{Container: container}
	if v, ok := m["name"].(string); ok {
		s.Name = v
	}
	if v, ok := m["description"].(string); ok {
		s.Description = v
	}
	if v, ok := m["currentTemperature"].(float64); ok {
		s.CurrentC = v
	}
	if v, ok := m["maxTemperature"].(float64); ok {
		s.MaxC = v
	}
	if v, ok := m["overheatThreshold"].(float64); ok {
		s.OverheatC = v
	}
	if v, ok := m["criticalThreshold"].(float64); ok {
		s.CriticalC = v
	} else if v, ok := m["alertThreshold"].(float64); ok {
		// Legacy field name on some platforms.
		s.CriticalC = v
	}
	if v, ok := m["hwStatus"].(string); ok {
		s.Status = v
	} else if v, ok := m["status"].(string); ok {
		s.Status = v
	}
	if v, ok := m["inAlertState"].(bool); ok {
		s.InAlert = v
	}
	return s
}

// checkSensor produces alert/warning issues for one sensor. A sensor
// is in alert when current ≥ criticalThreshold or hwStatus isn't ok;
// it's in warning when current ≥ overheatThreshold − failureMargin.
func (t *VerifyTemperature) checkSensor(s TempSensorReport, warnings, alerts *[]string) {
	label := s.Name
	if s.Container != "" {
		label = s.Container + "/" + s.Name
	}
	if s.Status != "" && !strings.EqualFold(s.Status, "ok") {
		*alerts = append(*alerts, fmt.Sprintf("%s: status %s", label, s.Status))
	}
	if s.InAlert {
		*alerts = append(*alerts, fmt.Sprintf("%s: inAlertState", label))
	}
	if s.CriticalC > 0 && s.CurrentC >= s.CriticalC {
		*alerts = append(*alerts, fmt.Sprintf("%s: current=%.1f°C ≥ critical=%.1f°C", label, s.CurrentC, s.CriticalC))
	}
	if s.OverheatC > 0 && s.CurrentC >= (s.OverheatC-t.FailureMargin) {
		*warnings = append(*warnings, fmt.Sprintf("%s: current=%.1f°C ≥ overheat=%.1f°C − margin=%.1f°C", label, s.CurrentC, s.OverheatC, t.FailureMargin))
	}
}

func hottestSensor(sensors []TempSensorReport) *TempSensorReport {
	if len(sensors) == 0 {
		return nil
	}
	idx := 0
	for i := 1; i < len(sensors); i++ {
		if sensors[i].CurrentC > sensors[idx].CurrentC {
			idx = i
		}
	}
	return &sensors[idx]
}

func (t *VerifyTemperature) ValidateInput(input any) error {
	if t.FailureMargin < 0 {
		return fmt.Errorf("failure margin cannot be negative")
	}
	return nil
}
