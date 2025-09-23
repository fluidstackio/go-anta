package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyTransceivers struct {
	test.BaseTest
	CheckManufacturer bool     `yaml:"check_manufacturer" json:"check_manufacturer"`
	Manufacturers     []string `yaml:"manufacturers,omitempty" json:"manufacturers,omitempty"`
	CheckTemperature  bool     `yaml:"check_temperature,omitempty" json:"check_temperature,omitempty"`
	CheckVoltage      bool     `yaml:"check_voltage,omitempty" json:"check_voltage,omitempty"`
}

func NewVerifyTransceivers(inputs map[string]interface{}) (test.Test, error) {
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
		if manufacturers, ok := inputs["manufacturers"].([]interface{}); ok {
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
	
	if transceiverData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if interfaces, ok := transceiverData["interfaces"].(map[string]interface{}); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]interface{}); ok {
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

func (t *VerifyTransceivers) validateTransceiver(intfName string, data map[string]interface{}, issues *[]string) error {
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

	if details, ok := data["details"].(map[string]interface{}); ok {
		if t.CheckTemperature {
			if tempData, ok := details["temperature"].(map[string]interface{}); ok {
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
			if voltageData, ok := details["voltage"].(map[string]interface{}); ok {
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

		if lanes, ok := details["lanes"].([]interface{}); ok {
			for i, lane := range lanes {
				if laneData, ok := lane.(map[string]interface{}); ok {
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

func (t *VerifyTransceivers) ValidateInput(input interface{}) error {
	return nil
}