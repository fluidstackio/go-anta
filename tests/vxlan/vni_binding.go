package vxlan

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyVxlanVniBinding verifies the VNI-VLAN and VNI-VRF bindings of the Vxlan1 interface.
//
// This test performs the following checks:
//  1. Retrieves the VNI bindings from the Vxlan1 interface configuration.
//  2. Validates that each expected VNI is bound to the correct VLAN or VRF.
//  3. Supports both VLAN ID (integer) and VRF name (string) bindings.
//  4. Checks multiple binding formats (vlanToVniMap, vrfToVniMap, vniBindings).
//
// Expected Results:
//   - Success: All expected VNI bindings match the device configuration.
//   - Failure: VNI binding not found or bound to incorrect VLAN/VRF.
//   - Error: Unable to retrieve Vxlan1 interface information from the device.
//
// Example YAML configuration:
//   - name: "VerifyVxlanVniBinding"
//     module: "vxlan"
//     inputs:
//       bindings:
//         "10001": 100      # VNI 10001 bound to VLAN 100
//         "10002": 200      # VNI 10002 bound to VLAN 200
//         "20001": "VRF_A"  # VNI 20001 bound to VRF_A
//         "20002": "VRF_B"  # VNI 20002 bound to VRF_B

type VerifyVxlanVniBinding struct {
	test.BaseTest
	Bindings map[string]any `yaml:"bindings" json:"bindings"`
}

func NewVerifyVxlanVniBinding(inputs map[string]any) (test.Test, error) {
	t := &VerifyVxlanVniBinding{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVxlanVniBinding",
			TestDescription: "Verify the VNI-VLAN, VNI-VRF bindings of the Vxlan1 interface",
			TestCategories:  []string{"vxlan", "binding"},
		},
		Bindings: make(map[string]any),
	}

	if inputs != nil {
		if bindings, ok := inputs["bindings"].(map[string]any); ok {
			t.Bindings = bindings
		}
	}

	return t, nil
}

func (t *VerifyVxlanVniBinding) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show interfaces Vxlan1",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get Vxlan1 interface bindings: %v", err)
		return result, nil
	}

	issues := []string{}
	deviceBindings := make(map[string]any)

	if intfData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := intfData["interfaces"].(map[string]any); ok {
			if vxlan1, ok := interfaces["Vxlan1"].(map[string]any); ok {
				// Parse VLAN bindings
				if vlanBindings, ok := vxlan1["vlanToVniMap"].(map[string]any); ok {
					for vlanStr, vniData := range vlanBindings {
						if vni, ok := vniData.(float64); ok {
							vniStr := strconv.Itoa(int(vni))
							vlanID, _ := strconv.Atoi(vlanStr)
							deviceBindings[vniStr] = vlanID
						}
					}
				}

				// Parse VRF bindings
				if vrfBindings, ok := vxlan1["vrfToVniMap"].(map[string]any); ok {
					for vrfName, vniData := range vrfBindings {
						if vni, ok := vniData.(float64); ok {
							vniStr := strconv.Itoa(int(vni))
							deviceBindings[vniStr] = vrfName
						}
					}
				}

				// Alternative structure - check vniBindings
				if vniBindings, ok := vxlan1["vniBindings"].(map[string]any); ok {
					for vniStr, bindingData := range vniBindings {
						if binding, ok := bindingData.(map[string]any); ok {
							if vlanID, ok := binding["vlan"].(float64); ok {
								deviceBindings[vniStr] = int(vlanID)
							} else if vrfName, ok := binding["vrf"].(string); ok {
								deviceBindings[vniStr] = vrfName
							}
						}
					}
				}
			} else {
				result.Status = test.TestError
				result.Message = "Vxlan1 interface not found"
				return result, nil
			}
		}
	}

	// Check each expected binding
	for vniStr, expectedBinding := range t.Bindings {
		deviceBinding, found := deviceBindings[vniStr]

		if !found {
			issues = append(issues, fmt.Sprintf("VNI %s binding not found on device", vniStr))
			continue
		}

		// Compare bindings based on type
		switch expectedValue := expectedBinding.(type) {
		case float64:
			// Expected VLAN ID
			expectedVlan := int(expectedValue)
			if deviceVlan, ok := deviceBinding.(int); ok {
				if deviceVlan != expectedVlan {
					issues = append(issues, fmt.Sprintf("VNI %s bound to VLAN %d, expected VLAN %d",
						vniStr, deviceVlan, expectedVlan))
				}
			} else {
				issues = append(issues, fmt.Sprintf("VNI %s is not bound to a VLAN (found: %v, expected VLAN %d)",
					vniStr, deviceBinding, expectedVlan))
			}
		case int:
			// Expected VLAN ID
			if deviceVlan, ok := deviceBinding.(int); ok {
				if deviceVlan != expectedValue {
					issues = append(issues, fmt.Sprintf("VNI %s bound to VLAN %d, expected VLAN %d",
						vniStr, deviceVlan, expectedValue))
				}
			} else {
				issues = append(issues, fmt.Sprintf("VNI %s is not bound to a VLAN (found: %v, expected VLAN %d)",
					vniStr, deviceBinding, expectedValue))
			}
		case string:
			// Expected VRF name
			if deviceVrf, ok := deviceBinding.(string); ok {
				if deviceVrf != expectedValue {
					issues = append(issues, fmt.Sprintf("VNI %s bound to VRF '%s', expected VRF '%s'",
						vniStr, deviceVrf, expectedValue))
				}
			} else {
				issues = append(issues, fmt.Sprintf("VNI %s is not bound to a VRF (found: %v, expected VRF '%s')",
					vniStr, deviceBinding, expectedValue))
			}
		default:
			issues = append(issues, fmt.Sprintf("VNI %s has unsupported binding type: %T", vniStr, expectedValue))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("VXLAN VNI binding issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"expected_bindings": t.Bindings,
			"device_bindings":   deviceBindings,
		}
	}

	return result, nil
}

func (t *VerifyVxlanVniBinding) ValidateInput(input any) error {
	if len(t.Bindings) == 0 {
		return fmt.Errorf("at least one VNI binding must be specified")
	}

	for vniStr, binding := range t.Bindings {
		// Validate VNI format
		if vni, err := strconv.Atoi(vniStr); err != nil || vni < 1 || vni > 16777215 {
			return fmt.Errorf("invalid VNI '%s' (must be 1-16777215)", vniStr)
		}

		// Validate binding value
		switch bindingValue := binding.(type) {
		case float64:
			vlan := int(bindingValue)
			if vlan < 1 || vlan > 4094 {
				return fmt.Errorf("VNI %s: invalid VLAN ID %d (must be 1-4094)", vniStr, vlan)
			}
		case int:
			if bindingValue < 1 || bindingValue > 4094 {
				return fmt.Errorf("VNI %s: invalid VLAN ID %d (must be 1-4094)", vniStr, bindingValue)
			}
		case string:
			if bindingValue == "" {
				return fmt.Errorf("VNI %s: VRF name cannot be empty", vniStr)
			}
		default:
			return fmt.Errorf("VNI %s: binding must be VLAN ID (number) or VRF name (string)", vniStr)
		}
	}

	return nil
}