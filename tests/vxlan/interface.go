package vxlan

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyVxlan1Interface verifies the operational status of the Vxlan1 interface.
//
// This test performs the following checks:
//  1. Verifies that the Vxlan1 interface exists in the device configuration.
//  2. Validates that the interface status is 'up' or 'connected'.
//  3. Validates that the line protocol status is 'up'.
//
// Expected Results:
//   - Success: Vxlan1 interface is present and both interface and line protocol are 'up'.
//   - Failure: Interface is missing, interface status is not 'up', or line protocol is not 'up'.
//   - Error: Unable to retrieve interface information from the device.
//
// Example YAML configuration:
//   - name: "VerifyVxlan1Interface"
//     module: "vxlan"
//     # No additional inputs required

type VerifyVxlan1Interface struct {
	test.BaseTest
}

func NewVerifyVxlan1Interface(inputs map[string]any) (test.Test, error) {
	t := &VerifyVxlan1Interface{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVxlan1Interface",
			TestDescription: "Verify the Vxlan1 interface status",
			TestCategories:  []string{"vxlan", "interface"},
		},
	}

	return t, nil
}

func (t *VerifyVxlan1Interface) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
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
		result.Message = fmt.Sprintf("Failed to get Vxlan1 interface status: %v", err)
		return result, nil
	}

	issues := []string{}

	if intfData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := intfData["interfaces"].(map[string]any); ok {
			if vxlan1, ok := interfaces["Vxlan1"].(map[string]any); ok {
				var interfaceStatus, lineProtocol string

				if status, ok := vxlan1["interfaceStatus"].(string); ok {
					interfaceStatus = strings.ToLower(status)
				}

				if protocol, ok := vxlan1["lineProtocolStatus"].(string); ok {
					lineProtocol = strings.ToLower(protocol)
				}

				// Check interface status
				if interfaceStatus != "up" && interfaceStatus != "connected" {
					issues = append(issues, fmt.Sprintf("Vxlan1 interface status is '%s', expected 'up'", interfaceStatus))
				}

				// Check line protocol status
				if lineProtocol != "up" {
					issues = append(issues, fmt.Sprintf("Vxlan1 line protocol status is '%s', expected 'up'", lineProtocol))
				}

				if len(issues) == 0 {
					result.Details = map[string]any{
						"interface_status":     interfaceStatus,
						"line_protocol_status": lineProtocol,
					}
				}
			} else {
				issues = append(issues, "Vxlan1 interface not found")
			}
		} else {
			issues = append(issues, "No interfaces data found in command output")
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Vxlan1 interface issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyVxlan1Interface) ValidateInput(input any) error {
	return nil
}