package routing

import (
	"context"
	"fmt"

	"github.com/fluidstack/go-anta/pkg/device"
	"github.com/fluidstack/go-anta/pkg/test"
)

// VerifyStaticRoutes verifies that static routes are configured and active in the routing table.
//
// This test validates that manually configured static routes are present in the
// routing table with the correct next-hop addresses. Static routes provide
// explicit routing paths and are important for network connectivity.
//
// Expected Results:
//   - Success: All specified static routes are found with correct next-hops and are active.
//   - Failure: A static route is missing, has incorrect next-hop, or is not active.
//   - Error: The test will error if routing table information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyStaticRoutes"
//     module: "routing"
//     inputs:
//       routes:
//         - prefix: "192.168.1.0/24"
//           next_hop: "10.0.0.1"
//           vrf: "default"
//         - prefix: "172.16.0.0/16"
//           next_hop: "10.0.0.2"
//           vrf: "PROD"
type VerifyStaticRoutes struct {
	test.BaseTest
	Routes []StaticRoute `yaml:"routes" json:"routes"`
}

type StaticRoute struct {
	Prefix  string `yaml:"prefix" json:"prefix"`
	NextHop string `yaml:"next_hop" json:"next_hop"`
	VRF     string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyStaticRoutes(inputs map[string]any) (test.Test, error) {
	t := &VerifyStaticRoutes{
		BaseTest: test.BaseTest{
			TestName:        "VerifyStaticRoutes",
			TestDescription: "Verify static routes are configured and active",
			TestCategories:  []string{"routing", "static"},
		},
	}

	if inputs != nil {
		if routes, ok := inputs["routes"].([]any); ok {
			for _, r := range routes {
				if routeMap, ok := r.(map[string]any); ok {
					route := StaticRoute{
						VRF: "default",
					}
					if prefix, ok := routeMap["prefix"].(string); ok {
						route.Prefix = prefix
					}
					if nextHop, ok := routeMap["next_hop"].(string); ok {
						route.NextHop = nextHop
					}
					if vrf, ok := routeMap["vrf"].(string); ok {
						route.VRF = vrf
					}
					t.Routes = append(t.Routes, route)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyStaticRoutes) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Routes) == 0 {
		result.Status = test.TestError
		result.Message = "No static routes configured for verification"
		return result, nil
	}

	issues := []string{}

	// Group routes by VRF to minimize API calls
	routesByVrf := make(map[string][]StaticRoute)
	for _, route := range t.Routes {
		vrfName := route.VRF
		if vrfName == "" {
			vrfName = "default"
		}
		routesByVrf[vrfName] = append(routesByVrf[vrfName], route)
	}

	// Query each VRF separately
	for vrfName, routes := range routesByVrf {
		var cmd device.Command
		if vrfName == "default" {
			cmd = device.Command{
				Template: "show ip route",
				Format:   "json",
				UseCache: false,
			}
		} else {
			cmd = device.Command{
				Template: fmt.Sprintf("show ip route vrf %s", vrfName),
				Format:   "json",
				UseCache: false,
			}
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			result.Status = test.TestError
			result.Message = fmt.Sprintf("Failed to get routing table for VRF %s: %v", vrfName, err)
			return result, nil
		}

		if routeData, ok := cmdResult.Output.(map[string]any); ok {
			if vrfs, ok := routeData["vrfs"].(map[string]any); ok {
				vrfData, vrfExists := vrfs[vrfName]
				if !vrfExists {
					issues = append(issues, fmt.Sprintf("VRF %s not found", vrfName))
					continue
				}

				if vrf, ok := vrfData.(map[string]any); ok {
					if vrfRoutes, ok := vrf["routes"].(map[string]any); ok {
						// Check each expected route in this VRF
						for _, expectedRoute := range routes {
							routeData, routeExists := vrfRoutes[expectedRoute.Prefix]
							if !routeExists {
								issues = append(issues, fmt.Sprintf("Route %s not found in VRF %s",
									expectedRoute.Prefix, vrfName))
								continue
							}

							if route, ok := routeData.(map[string]any); ok {
								found := false

								if routeType, ok := route["routeType"].(string); ok {
									if routeType != "static" {
										issues = append(issues, fmt.Sprintf("Route %s is not static (type: %s)",
											expectedRoute.Prefix, routeType))
										continue
									}
								}

								if vias, ok := route["vias"].([]any); ok {
									for _, via := range vias {
										if viaData, ok := via.(map[string]any); ok {
											if nexthopAddr, ok := viaData["nexthopAddr"].(string); ok {
												if nexthopAddr == expectedRoute.NextHop {
													found = true
													break
												}
											}
										}
									}
								}

								if !found {
									issues = append(issues, fmt.Sprintf("Route %s: next-hop %s not found",
										expectedRoute.Prefix, expectedRoute.NextHop))
								}
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Static route issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyStaticRoutes) ValidateInput(input any) error {
	if len(t.Routes) == 0 {
		return fmt.Errorf("at least one static route must be specified")
	}

	for i, route := range t.Routes {
		if route.Prefix == "" {
			return fmt.Errorf("route at index %d has no prefix", i)
		}
		if route.NextHop == "" {
			return fmt.Errorf("route %s has no next-hop", route.Prefix)
		}
	}

	return nil
}