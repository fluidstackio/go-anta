package vxlan

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
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

	intfData, err := test.AsMap(cmdResult.Output)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Unexpected Vxlan1 output: %v", err)
		return result, nil
	}
	interfaces, ok := intfData["interfaces"].(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "Vxlan1 output missing 'interfaces' field"
		return result, nil
	}
	vxlan1, ok := interfaces["Vxlan1"].(map[string]any)
	if !ok {
		result.Status = test.TestFailure
		result.Message = "Vxlan1 interface not found"
		return result, nil
	}

	var interfaceStatus, lineProtocol string
	if status, ok := vxlan1["interfaceStatus"].(string); ok {
		interfaceStatus = strings.ToLower(status)
	}
	if protocol, ok := vxlan1["lineProtocolStatus"].(string); ok {
		lineProtocol = strings.ToLower(protocol)
	}

	if interfaceStatus != "up" && interfaceStatus != "connected" {
		issues = append(issues, fmt.Sprintf("Vxlan1 interface status is '%s', expected 'up'", interfaceStatus))
	}
	if lineProtocol != "up" {
		issues = append(issues, fmt.Sprintf("Vxlan1 line protocol status is '%s', expected 'up'", lineProtocol))
	}

	if len(issues) == 0 {
		result.Details = map[string]any{
			"interface_status":     interfaceStatus,
			"line_protocol_status": lineProtocol,
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