package routing

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyOSPFNeighbors verifies OSPF neighbor adjacencies and their states.
//
// This test validates that OSPF neighbors are present and in the expected adjacency
// states (typically "Full" for established adjacencies). It ensures proper OSPF
// operation and network connectivity.
//
// Expected Results:
//   - Success: All specified OSPF neighbors are found with expected adjacency states.
//   - Failure: A neighbor is missing, has incorrect state, or router ID doesn't match.
//   - Error: The test will error if OSPF neighbor information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyOSPFNeighbors"
//     module: "routing"
//     inputs:
//       instance: "1"
//       neighbors:
//         - interface: "Ethernet1"
//           state: "Full"
//           router_id: "192.168.1.1"
//         - interface: "Ethernet2"
//           state: "Full"
//           router_id: "192.168.1.2"
type VerifyOSPFNeighbors struct {
	test.BaseTest
	Neighbors []OSPFNeighbor `yaml:"neighbors" json:"neighbors"`
	Instance  string         `yaml:"instance,omitempty" json:"instance,omitempty"`
}

type OSPFNeighbor struct {
	Interface string `yaml:"interface" json:"interface"`
	State     string `yaml:"state" json:"state"`
	RouterID  string `yaml:"router_id,omitempty" json:"router_id,omitempty"`
}

func NewVerifyOSPFNeighbors(inputs map[string]any) (test.Test, error) {
	t := &VerifyOSPFNeighbors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyOSPFNeighbors",
			TestDescription: "Verify OSPF neighbor adjacencies",
			TestCategories:  []string{"routing", "ospf"},
		},
		Instance: "1",
	}

	if inputs != nil {
		if neighbors, ok := inputs["neighbors"].([]any); ok {
			for _, n := range neighbors {
				if neighborMap, ok := n.(map[string]any); ok {
					neighbor := OSPFNeighbor{
						State: "Full",
					}
					if intf, ok := neighborMap["interface"].(string); ok {
						neighbor.Interface = intf
					}
					if state, ok := neighborMap["state"].(string); ok {
						neighbor.State = state
					}
					if routerId, ok := neighborMap["router_id"].(string); ok {
						neighbor.RouterID = routerId
					}
					t.Neighbors = append(t.Neighbors, neighbor)
				}
			}
		}

		if instance, ok := inputs["instance"].(string); ok {
			t.Instance = instance
		} else if instance, ok := inputs["instance"].(float64); ok {
			t.Instance = fmt.Sprintf("%d", int(instance))
		}
	}

	return t, nil
}

func (t *VerifyOSPFNeighbors) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Neighbors) == 0 {
		result.Status = test.TestError
		result.Message = "No OSPF neighbors configured for verification"
		return result, nil
	}

	cmd := device.Command{
		Template: fmt.Sprintf("show ip ospf %s neighbor", t.Instance),
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get OSPF neighbors: %v", err)
		return result, nil
	}

	issues := []string{}
	ospfNeighbors := make(map[string]OSPFNeighborInfo)

	if ospfData, ok := cmdResult.Output.(map[string]any); ok {
		if instances, ok := ospfData["instances"].(map[string]any); ok {
			if instData, ok := instances[t.Instance].(map[string]any); ok {
				if neighbors, ok := instData["neighbors"].([]any); ok {
					for _, n := range neighbors {
						if neighbor, ok := n.(map[string]any); ok {
							info := OSPFNeighborInfo{}
							
							if intf, ok := neighbor["interface"].(string); ok {
								info.Interface = intf
							}
							if state, ok := neighbor["adjacencyState"].(string); ok {
								info.State = state
							}
							if routerId, ok := neighbor["routerId"].(string); ok {
								info.RouterID = routerId
							}
							
							if info.Interface != "" {
								ospfNeighbors[info.Interface] = info
							}
						}
					}
				}
			}
		}
	}

	for _, expected := range t.Neighbors {
		actual, found := ospfNeighbors[expected.Interface]
		
		if !found {
			issues = append(issues, fmt.Sprintf("%s: no OSPF neighbor found", expected.Interface))
			continue
		}

		expectedState := normalizeOSPFState(expected.State)
		actualState := normalizeOSPFState(actual.State)
		
		if expectedState != actualState {
			issues = append(issues, fmt.Sprintf("%s: expected state %s, got %s",
				expected.Interface, expected.State, actual.State))
		}

		if expected.RouterID != "" && expected.RouterID != actual.RouterID {
			issues = append(issues, fmt.Sprintf("%s: expected router ID %s, got %s",
				expected.Interface, expected.RouterID, actual.RouterID))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("OSPF neighbor issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyOSPFNeighbors) ValidateInput(input any) error {
	if len(t.Neighbors) == 0 {
		return fmt.Errorf("at least one OSPF neighbor must be specified")
	}

	for i, neighbor := range t.Neighbors {
		if neighbor.Interface == "" {
			return fmt.Errorf("neighbor at index %d has no interface", i)
		}
	}

	return nil
}

type OSPFNeighborInfo struct {
	Interface string
	State     string
	RouterID  string
}

func normalizeOSPFState(state string) string {
	state = strings.ToLower(state)
	switch {
	case strings.Contains(state, "full"):
		return "full"
	case strings.Contains(state, "2way"):
		return "2way"
	case strings.Contains(state, "init"):
		return "init"
	case strings.Contains(state, "down"):
		return "down"
	default:
		return state
	}
}