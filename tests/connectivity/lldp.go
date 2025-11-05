package connectivity

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyLLDPNeighbors verifies the connection status of the specified LLDP (Link Layer Discovery Protocol) neighbors.
//
// Expected Results:
//   - Success: The test will pass if all provided LLDP neighbors are present and correctly connected.
//   - Failure: The test will fail if the provided LLDP neighbor is not found or system name/port does not match expected information.
//   - Error: The test will report an error if LLDP neighbor information cannot be retrieved.
//
// Examples:
//
//   - name: VerifyLLDPNeighbors with specific neighbors
//     VerifyLLDPNeighbors:
//     interfaces:
//
//   - interface: "Ethernet1"
//     neighbor_device: "spine1"
//     neighbor_port: "Ethernet1"
//
//   - interface: "Ethernet2"
//     neighbor_device: "spine2"
//     neighbor_port: "Ethernet1"
//
//   - name: VerifyLLDPNeighbors device name only
//     VerifyLLDPNeighbors:
//     interfaces:
//
//   - interface: "Management1"
//     neighbor_device: "mgmt-switch"
type VerifyLLDPNeighbors struct {
	test.BaseTest
	Interfaces []LLDPInterface `yaml:"interfaces" json:"interfaces"`
}

type LLDPInterface struct {
	Interface      string `yaml:"interface" json:"interface"`
	NeighborDevice string `yaml:"neighbor_device" json:"neighbor_device"`
	NeighborPort   string `yaml:"neighbor_port" json:"neighbor_port"`
}

func NewVerifyLLDPNeighbors(inputs map[string]any) (test.Test, error) {
	t := &VerifyLLDPNeighbors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLLDPNeighbors",
			TestDescription: "Verify LLDP neighbors on specified interfaces",
			TestCategories:  []string{"connectivity", "lldp"},
		},
	}

	if inputs != nil {
		if interfaces, ok := inputs["interfaces"].([]any); ok {
			for _, i := range interfaces {
				if intfMap, ok := i.(map[string]any); ok {
					intf := LLDPInterface{}
					if name, ok := intfMap["interface"].(string); ok {
						intf.Interface = name
					}
					if neighbor, ok := intfMap["neighbor_device"].(string); ok {
						intf.NeighborDevice = neighbor
					}
					if port, ok := intfMap["neighbor_port"].(string); ok {
						intf.NeighborPort = port
					}
					t.Interfaces = append(t.Interfaces, intf)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyLLDPNeighbors) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Interfaces) == 0 {
		result.Status = test.TestError
		result.Message = "No interfaces configured for LLDP verification"
		return result, nil
	}

	cmd := device.Command{
		Template: "show lldp neighbors detail",
		Format:   "json",
		UseCache: true,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get LLDP neighbors: %v", err)
		return result, nil
	}

	lldpNeighbors := make(map[string]LLDPNeighborInfo)

	if lldpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if neighbors, ok := lldpData["lldpNeighbors"].(map[string]interface{}); ok {
			// EOS structure: lldpNeighbors is a map with interface names as keys
			for interfaceName, neighborData := range neighbors {
				if neighborInfo, ok := neighborData.(map[string]interface{}); ok {
					if neighborList, ok := neighborInfo["lldpNeighborInfo"].([]interface{}); ok && len(neighborList) > 0 {
						// Take the first neighbor if multiple exist
						if neighbor, ok := neighborList[0].(map[string]interface{}); ok {
							var info LLDPNeighborInfo
							info.LocalPort = interfaceName

							if systemName, ok := neighbor["systemName"].(string); ok {
								info.SystemName = systemName
							}
							if chassisId, ok := neighbor["chassisId"].(string); ok {
								info.ChassisId = chassisId
							}

							// Extract remote port information
							if intfInfo, ok := neighbor["neighborInterfaceInfo"].(map[string]interface{}); ok {
								if remotePort, ok := intfInfo["interfaceId_v2"].(string); ok {
									info.PortDesc = remotePort
								} else if remotePort, ok := intfInfo["interfaceId"].(string); ok {
									// Remove quotes if present: "Ethernet1/1" -> Ethernet1/1
									info.PortDesc = strings.Trim(remotePort, "\"")
								}
							}

							lldpNeighbors[interfaceName] = info
						}
					}
				}
			}
		}
	}

	failures := []string{}
	for _, intf := range t.Interfaces {
		neighbor, found := lldpNeighbors[intf.Interface]

		if !found {
			failures = append(failures, fmt.Sprintf("%s: no LLDP neighbor found", intf.Interface))
			continue
		}

		if intf.NeighborDevice != "" && !strings.Contains(strings.ToLower(neighbor.SystemName), strings.ToLower(intf.NeighborDevice)) {
			failures = append(failures, fmt.Sprintf("%s: expected neighbor %s, got %s",
				intf.Interface, intf.NeighborDevice, neighbor.SystemName))
		}

		if intf.NeighborPort != "" && !strings.Contains(neighbor.PortDesc, intf.NeighborPort) {
			failures = append(failures, fmt.Sprintf("%s: expected neighbor port %s, got %s",
				intf.Interface, intf.NeighborPort, neighbor.PortDesc))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("LLDP verification failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyLLDPNeighbors) ValidateInput(input any) error {
	if t.Interfaces == nil || len(t.Interfaces) == 0 {
		return fmt.Errorf("at least one interface must be specified")
	}

	for i, intf := range t.Interfaces {
		if intf.Interface == "" {
			return fmt.Errorf("interface at index %d has no name", i)
		}
	}

	return nil
}

type LLDPNeighborInfo struct {
	LocalPort  string
	SystemName string
	PortDesc   string
	ChassisId  string
}
