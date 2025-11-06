package vlan

import (
	"context"
	"fmt"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyVlanInternalPolicy verifies the VLAN internal allocation policy and range configuration.
//
// This test performs the following checks:
//  1. Verifies the VLAN internal allocation policy is set to the expected value (ascending or descending).
//  2. Validates the VLAN internal allocation range matches the specified start and end VLAN IDs.
//  3. Ensures all currently allocated internal VLANs fall within the expected range.
//
// Expected Results:
//   - Success: Internal policy matches expected value, range is correctly configured, and all allocated VLANs are within range.
//   - Failure: Policy mismatch, incorrect range configuration, or allocated VLANs outside expected range.
//   - Error: Unable to retrieve VLAN internal usage information from the device.
//
// Example YAML configuration:
//   - name: "VerifyVlanInternalPolicy"
//     module: "vlan"
//     inputs:
//       policy: "ascending"      # Optional: "ascending" or "descending" (default: "ascending")
//       start_vlan_id: 1006      # Optional: Start of internal VLAN range (default: 1006)
//       end_vlan_id: 4094        # Optional: End of internal VLAN range (default: 4094)

type VerifyVlanInternalPolicy struct {
	test.BaseTest
	Policy      string `yaml:"policy" json:"policy"`
	StartVlanID int    `yaml:"start_vlan_id" json:"start_vlan_id"`
	EndVlanID   int    `yaml:"end_vlan_id" json:"end_vlan_id"`
}

func NewVerifyVlanInternalPolicy(inputs map[string]any) (test.Test, error) {
	t := &VerifyVlanInternalPolicy{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVlanInternalPolicy",
			TestDescription: "Verify VLAN internal allocation policy is ascending or descending and VLANs are within specified range",
			TestCategories:  []string{"vlan", "configuration"},
		},
		Policy:      "ascending", // Default policy
		StartVlanID: 1006,        // Default start VLAN ID
		EndVlanID:   4094,        // Default end VLAN ID
	}

	if inputs != nil {
		if policy, ok := inputs["policy"].(string); ok {
			if policy == "ascending" || policy == "descending" {
				t.Policy = policy
			}
		}
		if startVlan, ok := inputs["start_vlan_id"].(float64); ok {
			t.StartVlanID = int(startVlan)
		} else if startVlan, ok := inputs["start_vlan_id"].(int); ok {
			t.StartVlanID = startVlan
		}
		if endVlan, ok := inputs["end_vlan_id"].(float64); ok {
			t.EndVlanID = int(endVlan)
		} else if endVlan, ok := inputs["end_vlan_id"].(int); ok {
			t.EndVlanID = endVlan
		}
	}

	return t, nil
}

func (t *VerifyVlanInternalPolicy) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show vlan internal usage",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get VLAN internal usage: %v", err)
		return result, nil
	}

	issues := []string{}

	if vlanData, ok := cmdResult.Output.(map[string]any); ok {
		// Check internal VLAN policy
		if policy, ok := vlanData["internalVlanPolicy"].(string); ok {
			if policy != t.Policy {
				issues = append(issues, fmt.Sprintf("VLAN internal policy is '%s', expected '%s'", policy, t.Policy))
			}
		} else {
			issues = append(issues, "Could not determine VLAN internal policy")
		}

		// Check internal VLAN range
		if internalVlans, ok := vlanData["internalVlanRange"].(map[string]any); ok {
			var actualStart, actualEnd int

			if start, ok := internalVlans["startVlanId"].(float64); ok {
				actualStart = int(start)
			} else if start, ok := internalVlans["start"].(float64); ok {
				actualStart = int(start)
			}

			if end, ok := internalVlans["endVlanId"].(float64); ok {
				actualEnd = int(end)
			} else if end, ok := internalVlans["end"].(float64); ok {
				actualEnd = int(end)
			}

			if actualStart != 0 && actualStart != t.StartVlanID {
				issues = append(issues, fmt.Sprintf("VLAN internal range start is %d, expected %d", actualStart, t.StartVlanID))
			}

			if actualEnd != 0 && actualEnd != t.EndVlanID {
				issues = append(issues, fmt.Sprintf("VLAN internal range end is %d, expected %d", actualEnd, t.EndVlanID))
			}
		}

		// Check current internal VLAN allocations are within range
		if allocations, ok := vlanData["internalVlanAllocations"].([]any); ok {
			for _, allocation := range allocations {
				if alloc, ok := allocation.(map[string]any); ok {
					if vlanID, ok := alloc["vlanId"].(float64); ok {
						vlanIDInt := int(vlanID)
						if vlanIDInt < t.StartVlanID || vlanIDInt > t.EndVlanID {
							issues = append(issues, fmt.Sprintf("Internal VLAN %d is outside expected range %d-%d",
								vlanIDInt, t.StartVlanID, t.EndVlanID))
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("VLAN internal policy issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"policy":        t.Policy,
			"start_vlan_id": t.StartVlanID,
			"end_vlan_id":   t.EndVlanID,
		}
	}

	return result, nil
}

func (t *VerifyVlanInternalPolicy) ValidateInput(input any) error {
	if t.Policy != "ascending" && t.Policy != "descending" {
		return fmt.Errorf("policy must be 'ascending' or 'descending'")
	}
	if t.StartVlanID < 1 || t.StartVlanID > 4094 {
		return fmt.Errorf("start_vlan_id must be between 1 and 4094")
	}
	if t.EndVlanID < 1 || t.EndVlanID > 4094 {
		return fmt.Errorf("end_vlan_id must be between 1 and 4094")
	}
	if t.StartVlanID >= t.EndVlanID {
		return fmt.Errorf("start_vlan_id must be less than end_vlan_id")
	}
	return nil
}