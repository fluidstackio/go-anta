package routing

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyStaticRoutes struct {
	test.BaseTest
	Routes []StaticRoute `yaml:"routes" json:"routes"`
}

type StaticRoute struct {
	Prefix  string `yaml:"prefix" json:"prefix"`
	NextHop string `yaml:"next_hop" json:"next_hop"`
	VRF     string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyStaticRoutes(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyStaticRoutes{
		BaseTest: test.BaseTest{
			TestName:        "VerifyStaticRoutes",
			TestDescription: "Verify static routes are configured and active",
			TestCategories:  []string{"routing", "static"},
		},
	}

	if inputs != nil {
		if routes, ok := inputs["routes"].([]interface{}); ok {
			for _, r := range routes {
				if routeMap, ok := r.(map[string]interface{}); ok {
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

	cmd := device.Command{
		Template: "show ip route",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get routing table: %v", err)
		return result, nil
	}

	issues := []string{}
	
	if routeData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if vrfs, ok := routeData["vrfs"].(map[string]interface{}); ok {
			for _, expectedRoute := range t.Routes {
				vrfName := expectedRoute.VRF
				if vrfName == "" {
					vrfName = "default"
				}

				vrfData, vrfExists := vrfs[vrfName]
				if !vrfExists {
					issues = append(issues, fmt.Sprintf("VRF %s not found", vrfName))
					continue
				}

				if vrf, ok := vrfData.(map[string]interface{}); ok {
					if routes, ok := vrf["routes"].(map[string]interface{}); ok {
						routeData, routeExists := routes[expectedRoute.Prefix]
						if !routeExists {
							issues = append(issues, fmt.Sprintf("Route %s not found in VRF %s", 
								expectedRoute.Prefix, vrfName))
							continue
						}

						if route, ok := routeData.(map[string]interface{}); ok {
							found := false
							
							if routeType, ok := route["routeType"].(string); ok {
								if routeType != "static" {
									issues = append(issues, fmt.Sprintf("Route %s is not static (type: %s)",
										expectedRoute.Prefix, routeType))
									continue
								}
							}

							if vias, ok := route["vias"].([]interface{}); ok {
								for _, via := range vias {
									if viaData, ok := via.(map[string]interface{}); ok {
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

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Static route issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyStaticRoutes) ValidateInput(input interface{}) error {
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