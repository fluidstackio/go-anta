package interfaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyInterfacesStatus verifies that specified interfaces are in the expected status.
//
// Expected Results:
//   - Success: The test will pass if all specified interfaces are in the expected interface and protocol status.
//   - Failure: The test will fail if any interface is not in the expected state or does not exist.
//   - Error: The test will report an error if interface status cannot be retrieved.
//
// Examples:
//   - name: VerifyInterfacesStatus with specific states
//     VerifyInterfacesStatus:
//       interfaces:
//         - name: "Ethernet1/1"
//           status: "up"
//           protocol: "up"
//         - name: "Ethernet2/1"
//           status: "adminDown"
//
//   - name: VerifyInterfacesStatus protocol only
//     VerifyInterfacesStatus:
//       interfaces:
//         - name: "Management1"
//           protocol: "up"
type VerifyInterfacesStatus struct {
	test.BaseTest
	Interfaces []InterfaceStatus `yaml:"interfaces" json:"interfaces"`
}

type InterfaceStatus struct {
	Name     string `yaml:"name" json:"name"`
	Status   string `yaml:"status,omitempty" json:"status,omitempty"`         // up, down, notPresent
	Protocol string `yaml:"protocol,omitempty" json:"protocol,omitempty"`     // up, down, notPresent, lowerLayerDown
}

func NewVerifyInterfacesStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifyInterfacesStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifyInterfacesStatus",
			TestDescription: "Verify interface status and line protocol status",
			TestCategories:  []string{"interfaces", "status"},
		},
	}

	if inputs != nil {
		if interfaces, ok := inputs["interfaces"].([]any); ok {
			for _, i := range interfaces {
				if intfMap, ok := i.(map[string]any); ok {
					intf := InterfaceStatus{
						Status:   "up", // Default expectation
						Protocol: "up", // Default expectation
					}

					if name, ok := intfMap["name"].(string); ok {
						intf.Name = name
					}
					if status, ok := intfMap["status"].(string); ok {
						intf.Status = strings.ToLower(status)
					}
					if protocol, ok := intfMap["protocol"].(string); ok {
						intf.Protocol = strings.ToLower(protocol)
					}

					t.Interfaces = append(t.Interfaces, intf)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyInterfacesStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Interfaces) == 0 {
		result.Status = test.TestError
		result.Message = "No interfaces configured for status verification"
		return result, nil
	}

	cmd := device.Command{
		Template: "show interfaces description",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get interface descriptions: %v", err)
		return result, nil
	}

	deviceInterfaces := make(map[string]InterfaceInfo)

	if intfData, ok := cmdResult.Output.(map[string]any); ok {
		if descriptions, ok := intfData["interfaceDescriptions"].(map[string]any); ok {
			for intfName, intfInfo := range descriptions {
				if info, ok := intfInfo.(map[string]any); ok {
					deviceIntf := InterfaceInfo{
						Name: intfName,
					}

					if status, ok := info["interfaceStatus"].(string); ok {
						deviceIntf.Status = strings.ToLower(status)
					}
					if protocol, ok := info["lineProtocolStatus"].(string); ok {
						deviceIntf.Protocol = strings.ToLower(protocol)
					}
					if desc, ok := info["description"].(string); ok {
						deviceIntf.Description = desc
					}

					deviceInterfaces[intfName] = deviceIntf
				}
			}
		}
	}

	failures := []string{}

	for _, expectedIntf := range t.Interfaces {
		deviceIntf, found := deviceInterfaces[expectedIntf.Name]

		if !found {
			failures = append(failures, fmt.Sprintf("Interface %s not found", expectedIntf.Name))
			continue
		}

		// Check interface status
		if expectedIntf.Status != "" && deviceIntf.Status != expectedIntf.Status {
			failures = append(failures, fmt.Sprintf("Interface %s: expected status '%s', got '%s'",
				expectedIntf.Name, expectedIntf.Status, deviceIntf.Status))
		}

		// Check line protocol status
		if expectedIntf.Protocol != "" && deviceIntf.Protocol != expectedIntf.Protocol {
			failures = append(failures, fmt.Sprintf("Interface %s: expected protocol '%s', got '%s'",
				expectedIntf.Name, expectedIntf.Protocol, deviceIntf.Protocol))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Interface status failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyInterfacesStatus) ValidateInput(input any) error {
	if len(t.Interfaces) == 0 {
		return fmt.Errorf("at least one interface must be specified")
	}

	for i, intf := range t.Interfaces {
		if intf.Name == "" {
			return fmt.Errorf("interface at index %d has no name", i)
		}
	}

	return nil
}

type InterfaceInfo struct {
	Name        string
	Status      string
	Protocol    string
	Description string
}