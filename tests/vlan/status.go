package vlan

import (
	"context"
	"fmt"

	"github.com/fluidstack/go-anta/pkg/device"
	"github.com/fluidstack/go-anta/pkg/test"
)

// VerifyVlanStatus verifies the administrative status and names of specified VLANs.
//
// This test performs the following checks:
//  1. Verifies that all specified VLANs exist in the device configuration.
//  2. Validates that each VLAN has the expected administrative status (active/suspend).
//  3. Optionally validates VLAN names match expected values if specified.
//
// Expected Results:
//   - Success: All specified VLANs exist with correct status and names (if specified).
//   - Failure: Missing VLANs, incorrect status, or name mismatches.
//   - Error: Unable to retrieve VLAN information from the device.
//
// Example YAML configuration:
//   - name: "VerifyVlanStatus"
//     module: "vlan"
//     inputs:
//       vlans:                   # Required: List of VLANs to verify
//         - id: 100
//           status: "active"     # Optional: Expected status (default: "active")
//           name: "VLAN100"      # Optional: Expected VLAN name
//         - id: 200
//           status: "suspend"

type VerifyVlanStatus struct {
	test.BaseTest
	Vlans []VlanStatus `yaml:"vlans" json:"vlans"`
}

type VlanStatus struct {
	ID     int    `yaml:"id" json:"id"`
	Status string `yaml:"status,omitempty" json:"status,omitempty"`
	Name   string `yaml:"name,omitempty" json:"name,omitempty"`
}

func NewVerifyVlanStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifyVlanStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVlanStatus",
			TestDescription: "Verify the administrative status of specified VLANs",
			TestCategories:  []string{"vlan", "status"},
		},
	}

	if inputs != nil {
		if vlans, ok := inputs["vlans"].([]any); ok {
			for _, v := range vlans {
				if vlanMap, ok := v.(map[string]any); ok {
					vlan := VlanStatus{
						Status: "active", // Default expected status
					}

					if id, ok := vlanMap["id"].(float64); ok {
						vlan.ID = int(id)
					} else if id, ok := vlanMap["id"].(int); ok {
						vlan.ID = id
					}

					if status, ok := vlanMap["status"].(string); ok {
						vlan.Status = status
					}

					if name, ok := vlanMap["name"].(string); ok {
						vlan.Name = name
					}

					if vlan.ID > 0 {
						t.Vlans = append(t.Vlans, vlan)
					}
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyVlanStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show vlan",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get VLAN information: %v", err)
		return result, nil
	}

	issues := []string{}
	deviceVlans := make(map[int]DeviceVlan)

	if vlanData, ok := cmdResult.Output.(map[string]any); ok {
		if vlans, ok := vlanData["vlans"].(map[string]any); ok {
			for vlanIDStr, vlanInfo := range vlans {
				if vlan, ok := vlanInfo.(map[string]any); ok {
					deviceVlan := DeviceVlan{}

					// Parse VLAN ID from string key
					var vlanID int
					fmt.Sscanf(vlanIDStr, "%d", &vlanID)
					deviceVlan.ID = vlanID

					if status, ok := vlan["status"].(string); ok {
						deviceVlan.Status = status
					}

					if name, ok := vlan["name"].(string); ok {
						deviceVlan.Name = name
					}

					// Handle dynamic status field
					if dynamic, ok := vlan["dynamic"].(bool); ok {
						deviceVlan.Dynamic = dynamic
					}

					deviceVlans[vlanID] = deviceVlan
				}
			}
		}
	}

	// Check each expected VLAN
	for _, expectedVlan := range t.Vlans {
		deviceVlan, found := deviceVlans[expectedVlan.ID]

		if !found {
			issues = append(issues, fmt.Sprintf("VLAN %d not found on device", expectedVlan.ID))
			continue
		}

		// Check status
		if expectedVlan.Status != "" && deviceVlan.Status != expectedVlan.Status {
			issues = append(issues, fmt.Sprintf("VLAN %d status is '%s', expected '%s'",
				expectedVlan.ID, deviceVlan.Status, expectedVlan.Status))
		}

		// Check name if specified
		if expectedVlan.Name != "" && deviceVlan.Name != expectedVlan.Name {
			issues = append(issues, fmt.Sprintf("VLAN %d name is '%s', expected '%s'",
				expectedVlan.ID, deviceVlan.Name, expectedVlan.Name))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("VLAN status issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"checked_vlans": len(t.Vlans),
			"device_vlans":  len(deviceVlans),
		}
	}

	return result, nil
}

func (t *VerifyVlanStatus) ValidateInput(input any) error {
	if len(t.Vlans) == 0 {
		return fmt.Errorf("at least one VLAN must be specified")
	}

	for i, vlan := range t.Vlans {
		if vlan.ID < 1 || vlan.ID > 4094 {
			return fmt.Errorf("VLAN at index %d has invalid ID %d (must be 1-4094)", i, vlan.ID)
		}
		if vlan.Status != "" && vlan.Status != "active" && vlan.Status != "suspend" {
			return fmt.Errorf("VLAN %d has invalid status '%s' (must be 'active' or 'suspend')", vlan.ID, vlan.Status)
		}
	}

	return nil
}

type DeviceVlan struct {
	ID      int
	Status  string
	Name    string
	Dynamic bool
}