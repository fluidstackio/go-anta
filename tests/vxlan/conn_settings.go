package vxlan

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyVxlan1ConnSettings verifies the VXLAN connection settings including source interface and UDP port.
//
// This test performs the following checks:
//  1. Retrieves the Vxlan1 interface configuration from the device.
//  2. Validates that the source interface matches the expected configuration.
//  3. Verifies that the UDP port matches the expected value (defaults to 4789).
//  4. Supports both sourceInterface and srcInterface field names.
//
// Expected Results:
//   - Success: Both source interface and UDP port match the expected configuration.
//   - Failure: Source interface or UDP port doesn't match the expected values.
//   - Error: Unable to retrieve Vxlan1 interface configuration from the device.
//
// Example YAML configuration:
//   - name: "VerifyVxlan1ConnSettings"
//     module: "vxlan"
//     inputs:
//       source_interface: "Loopback0"
//       udp_port: 4789

type VerifyVxlan1ConnSettings struct {
	test.BaseTest
	SourceInterface string `yaml:"source_interface" json:"source_interface"`
	UdpPort         int    `yaml:"udp_port" json:"udp_port"`
}

func NewVerifyVxlan1ConnSettings(inputs map[string]any) (test.Test, error) {
	t := &VerifyVxlan1ConnSettings{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVxlan1ConnSettings",
			TestDescription: "Verify Vxlan1 source interface and UDP port",
			TestCategories:  []string{"vxlan", "configuration"},
		},
		UdpPort: 4789, // Default VXLAN UDP port
	}

	if inputs != nil {
		if sourceIntf, ok := inputs["source_interface"].(string); ok {
			t.SourceInterface = sourceIntf
		}
		if udpPort, ok := inputs["udp_port"].(float64); ok {
			t.UdpPort = int(udpPort)
		} else if udpPort, ok := inputs["udp_port"].(int); ok {
			t.UdpPort = udpPort
		}
	}

	return t, nil
}

func (t *VerifyVxlan1ConnSettings) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
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
		result.Message = fmt.Sprintf("Failed to get Vxlan1 interface configuration: %v", err)
		return result, nil
	}

	issues := []string{}

	if intfData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := intfData["interfaces"].(map[string]any); ok {
			if vxlan1, ok := interfaces["Vxlan1"].(map[string]any); ok {
				var deviceSourceIntf string
				var deviceUdpPort int

				// Check source interface
				if sourceIntf, ok := vxlan1["sourceInterface"].(string); ok {
					deviceSourceIntf = sourceIntf
				} else if srcIntf, ok := vxlan1["srcInterface"].(string); ok {
					deviceSourceIntf = srcIntf
				}

				// Check UDP port
				if port, ok := vxlan1["udpPort"].(float64); ok {
					deviceUdpPort = int(port)
				} else if port, ok := vxlan1["port"].(float64); ok {
					deviceUdpPort = int(port)
				}

				// Validate source interface
				if t.SourceInterface != "" && deviceSourceIntf != t.SourceInterface {
					issues = append(issues, fmt.Sprintf("Vxlan1 source interface is '%s', expected '%s'",
						deviceSourceIntf, t.SourceInterface))
				}

				// Validate UDP port
				if deviceUdpPort != t.UdpPort {
					issues = append(issues, fmt.Sprintf("Vxlan1 UDP port is %d, expected %d",
						deviceUdpPort, t.UdpPort))
				}

				if len(issues) == 0 {
					result.Details = map[string]any{
						"source_interface": deviceSourceIntf,
						"udp_port":         deviceUdpPort,
					}
				}
			} else {
				result.Status = test.TestError
				result.Message = "Vxlan1 interface not found"
				return result, nil
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Vxlan1 connection settings issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyVxlan1ConnSettings) ValidateInput(input any) error {
	if t.SourceInterface == "" && t.UdpPort == 0 {
		return fmt.Errorf("either source_interface or udp_port must be specified")
	}

	if t.UdpPort != 0 && (t.UdpPort < 1024 || t.UdpPort > 65335) {
		return fmt.Errorf("udp_port must be between 1024 and 65335")
	}

	return nil
}