package vxlan

import (
	"context"
	"fmt"
	"net"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyVxlanVtep verifies the presence of expected VXLAN Tunnel Endpoints (VTEPs).
//
// This test performs the following checks:
//  1. Retrieves the list of active VXLAN VTEPs from the device.
//  2. Validates that each expected VTEP peer is found and operational.
//  3. Supports multiple output formats (vteps, remoteVteps, vtepList).
//  4. Validates VTEP IP addresses are properly formatted IPv4 addresses.
//
// Expected Results:
//   - Success: All expected VTEP peers are found and operational on the device.
//   - Failure: One or more expected VTEP peers are not found or not operational.
//   - Error: Unable to retrieve VXLAN VTEP information from the device.
//
// Example YAML configuration:
//   - name: "VerifyVxlanVtep"
//     module: "vxlan"
//     inputs:
//       vteps:
//         - "192.168.1.1"
//         - "192.168.1.2"
//         - "192.168.1.3"

type VerifyVxlanVtep struct {
	test.BaseTest
	Vteps []string `yaml:"vteps" json:"vteps"`
}

func NewVerifyVxlanVtep(inputs map[string]any) (test.Test, error) {
	t := &VerifyVxlanVtep{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVxlanVtep",
			TestDescription: "Verify Vxlan1 VTEP peers",
			TestCategories:  []string{"vxlan", "vtep"},
		},
	}

	if inputs != nil {
		if vteps, ok := inputs["vteps"].([]any); ok {
			for _, v := range vteps {
				if vtep, ok := v.(string); ok {
					t.Vteps = append(t.Vteps, vtep)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyVxlanVtep) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show vxlan vtep",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get VXLAN VTEP information: %v", err)
		return result, nil
	}

	issues := []string{}
	deviceVteps := make(map[string]bool)

	vtepData, ok := cmdResult.Output.(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to parse VXLAN VTEP output"
		return result, nil
	}

	// Parse VTEP peers
	if vteps, ok := vtepData["vteps"].([]any); ok {
		for _, vtepInfo := range vteps {
			vtep, ok := vtepInfo.(map[string]any)
			if !ok {
				continue
			}

			if address, ok := vtep["address"].(string); ok {
				deviceVteps[address] = true
			} else if vtepAddr, ok := vtep["vtep"].(string); ok {
				deviceVteps[vtepAddr] = true
			}
		}
	}

	// Alternative structure - check remote VTEPs
	if remoteVteps, ok := vtepData["remoteVteps"].(map[string]any); ok {
		for vtepAddr, vtepInfo := range remoteVteps {
			vtepDetails, ok := vtepInfo.(map[string]any)
			if !ok {
				continue
			}

			// Check if VTEP is active/operational
			status, hasStatus := vtepDetails["status"].(string)
			if !hasStatus || status == "up" || status == "active" || status == "operational" {
				deviceVteps[vtepAddr] = true
			}
		}
	}

	// Check for VTEP list in different format
	if vtepList, ok := vtepData["vtepList"].([]any); ok {
		for _, vtepAddr := range vtepList {
			if addr, ok := vtepAddr.(string); ok {
				deviceVteps[addr] = true
			}
		}
	}

	// Check each expected VTEP
	for _, expectedVtep := range t.Vteps {
		if !deviceVteps[expectedVtep] {
			issues = append(issues, fmt.Sprintf("VTEP peer %s not found", expectedVtep))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("VXLAN VTEP issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"expected_vteps": t.Vteps,
			"device_vteps":   deviceVteps,
		}
	}

	return result, nil
}

func (t *VerifyVxlanVtep) ValidateInput(input any) error {
	if len(t.Vteps) == 0 {
		return fmt.Errorf("at least one VTEP peer must be specified")
	}

	for i, vtep := range t.Vteps {
		// Validate IP address format
		if ip := net.ParseIP(vtep); ip == nil {
			return fmt.Errorf("VTEP at index %d has invalid IP address: %s", i, vtep)
		}

		// Ensure it's IPv4
		if net.ParseIP(vtep).To4() == nil {
			return fmt.Errorf("VTEP at index %d must be an IPv4 address: %s", i, vtep)
		}
	}

	return nil
}