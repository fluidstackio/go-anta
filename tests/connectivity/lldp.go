package connectivity

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyLLDPNeighbors struct {
	test.BaseTest
	Interfaces []LLDPInterface `yaml:"interfaces" json:"interfaces"`
}

type LLDPInterface struct {
	Interface      string `yaml:"interface" json:"interface"`
	NeighborDevice string `yaml:"neighbor_device" json:"neighbor_device"`
	NeighborPort   string `yaml:"neighbor_port" json:"neighbor_port"`
}

func NewVerifyLLDPNeighbors(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyLLDPNeighbors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyLLDPNeighbors",
			TestDescription: "Verify LLDP neighbors on specified interfaces",
			TestCategories:  []string{"connectivity", "lldp"},
		},
	}

	if inputs != nil {
		if interfaces, ok := inputs["interfaces"].([]interface{}); ok {
			for _, i := range interfaces {
				if intfMap, ok := i.(map[string]interface{}); ok {
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
		if neighbors, ok := lldpData["lldpNeighbors"].([]interface{}); ok {
			for _, n := range neighbors {
				if neighbor, ok := n.(map[string]interface{}); ok {
					var info LLDPNeighborInfo
					
					if port, ok := neighbor["port"].(string); ok {
						info.LocalPort = port
					}
					
					if neighborInfo, ok := neighbor["neighborInfo"].(map[string]interface{}); ok {
						if systemName, ok := neighborInfo["systemName"].(string); ok {
							info.SystemName = systemName
						}
						if portDesc, ok := neighborInfo["portDesc"].(string); ok {
							info.PortDesc = portDesc
						}
						if chassisId, ok := neighborInfo["chassisId"].(string); ok {
							info.ChassisId = chassisId
						}
					}
					
					if info.LocalPort != "" {
						lldpNeighbors[info.LocalPort] = info
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

func (t *VerifyLLDPNeighbors) ValidateInput(input interface{}) error {
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