package hardware

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/platform"
	"github.com/gavmckee/go-anta/internal/test"
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
//       check_temp_sensors: true
//       failure_margin: 5.0  # degrees Celsius margin before overheat
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

	warnings := []string{}
	alerts := []string{}

	if tempData, ok := cmdResult.Output.(map[string]any); ok {
		if sensors, ok := tempData["tempSensors"].([]any); ok {
			for _, s := range sensors {
				if sensor, ok := s.(map[string]any); ok {
					sensorName := ""
					if name, ok := sensor["name"].(string); ok {
						sensorName = name
					}

					currentTemp := 0.0
					if temp, ok := sensor["currentTemperature"].(float64); ok {
						currentTemp = temp
					}

					maxTemp := 0.0
					if max, ok := sensor["maxTemperature"].(float64); ok {
						maxTemp = max
					}

					alertThreshold := 0.0
					if alert, ok := sensor["alertThreshold"].(float64); ok {
						alertThreshold = alert
					}

					overheatThreshold := 0.0  
					if overheat, ok := sensor["overheatThreshold"].(float64); ok {
						overheatThreshold = overheat
					}

					if alertThreshold > 0 && currentTemp >= alertThreshold {
						alerts = append(alerts, fmt.Sprintf("%s: current=%.1f°C, alert=%.1f°C", 
							sensorName, currentTemp, alertThreshold))
					}

					if overheatThreshold > 0 && currentTemp >= (overheatThreshold-t.FailureMargin) {
						warnings = append(warnings, fmt.Sprintf("%s: current=%.1f°C, overheat=%.1f°C (margin=%.1f°C)",
							sensorName, currentTemp, overheatThreshold, t.FailureMargin))
					}

					if maxTemp > 0 && currentTemp > maxTemp {
						alerts = append(alerts, fmt.Sprintf("%s: current=%.1f°C exceeds max=%.1f°C",
							sensorName, currentTemp, maxTemp))
					}
				}
			}
		}

		if systemStatus, ok := tempData["systemStatus"].(string); ok {
			if systemStatus != "temperatureOk" {
				alerts = append(alerts, fmt.Sprintf("System temperature status: %s", systemStatus))
			}
		}
	}

	if len(alerts) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Temperature alerts: %v", alerts)
	} else if len(warnings) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Temperature warnings: %v", warnings)
	}

	return result, nil
}

func (t *VerifyTemperature) ValidateInput(input any) error {
	if t.FailureMargin < 0 {
		return fmt.Errorf("failure margin cannot be negative")
	}
	return nil
}

