package evpn

import (
	"context"
	"fmt"
	"net"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyEVPNType5Routes verifies EVPN Type-5 routes for given IP prefixes and VNIs.
//
// This test supports multiple levels of verification based on the provided input:
//
//  1. **Prefix/VNI only:** Verifies there is at least one 'active' and 'valid' path across all
//     Route Distinguishers (RDs) learning the given prefix and VNI.
//  2. **Specific Routes (RD/Domain):** Verifies that routes matching the specified RDs and domains
//     exist for the prefix/VNI. For each specified route, it checks if at least one of its paths
//     is 'active' and 'valid'.
//  3. **Specific Paths (Nexthop/Route Targets):** Verifies that specific paths exist within a
//     specified route (RD/Domain). For each specified path criteria (nexthop and optional route targets),
//     it finds all matching paths received from the peer and checks if at least one of these
//     matching paths is 'active' and 'valid'. The route targets check ensures all specified RTs
//     are present in the path's extended communities (subset check).
//
// Expected Results:
//   - Success:
//       * If only prefix/VNI is provided: The prefix/VNI exists in the EVPN table
//         and has at least one active and valid path across all RDs.
//       * If specific routes are provided: All specified routes (by RD/Domain) are found,
//         and each has at least one active and valid path (if paths are not specified for the route).
//       * If specific paths are provided: All specified routes are found, and for each specified path criteria (nexthop/RTs),
//         at least one matching path exists and is active and valid.
//   - Failure:
//       * No EVPN Type-5 routes are found for the given prefix/VNI.
//       * A specified route (RD/Domain) is not found.
//       * No active and valid path is found when required (either globally for the prefix, per specified route, or per specified path criteria).
//       * A specified path criteria (nexthop/RTs) does not match any received paths for the route.
//   - Error: Unable to retrieve EVPN routing information from the device.
//
// Example YAML configuration:
//   - name: "VerifyEVPNType5Routes"
//     module: "evpn"
//     inputs:
//       prefixes:
//         # At least one active/valid path across all RDs
//         - address: 192.168.10.0/24
//           vni: 10
//         # Specific routes each has at least one active/valid path
//         - address: 192.168.20.0/24
//           vni: 20
//           routes:
//             - rd: "10.0.0.1:20"
//               domain: local
//             - rd: "10.0.0.2:20"
//               domain: remote
//         # At least one active/valid path matching the nexthop
//         - address: 192.168.30.0/24
//           vni: 30
//           routes:
//             - rd: "10.0.0.1:30"
//               domain: local
//               paths:
//                 - nexthop: 10.1.1.1
//         # At least one active/valid path matching nexthop and specific RTs
//         - address: 192.168.40.0/24
//           vni: 40
//           routes:
//             - rd: "10.0.0.1:40"
//               domain: local
//               paths:
//                 - nexthop: 10.1.1.1
//                   route_targets:
//                     - "40:40"

type VerifyEVPNType5Routes struct {
	test.BaseTest
	Prefixes []EVPNPrefix `yaml:"prefixes,omitempty" json:"prefixes,omitempty"`
	Routes   []EVPNRoute  `yaml:"routes,omitempty" json:"routes,omitempty"`
	Paths    []EVPNPath   `yaml:"paths,omitempty" json:"paths,omitempty"`
}

type EVPNPrefix struct {
	Prefix string `yaml:"prefix" json:"prefix"`
	VNI    int    `yaml:"vni" json:"vni"`
}

type EVPNRoute struct {
	RD     string `yaml:"rd" json:"rd"`
	Domain string `yaml:"domain,omitempty" json:"domain,omitempty"`
	Prefix string `yaml:"prefix" json:"prefix"`
	VNI    int    `yaml:"vni" json:"vni"`
}

type EVPNPath struct {
	Prefix       string   `yaml:"prefix" json:"prefix"`
	VNI          int      `yaml:"vni" json:"vni"`
	NextHop      string   `yaml:"nexthop,omitempty" json:"nexthop,omitempty"`
	RouteTargets []string `yaml:"route_targets,omitempty" json:"route_targets,omitempty"`
}

func NewVerifyEVPNType5Routes(inputs map[string]any) (test.Test, error) {
	t := &VerifyEVPNType5Routes{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEVPNType5Routes",
			TestDescription: "Verify EVPN Type-5 routes for given IP prefixes and VNIs",
			TestCategories:  []string{"evpn", "routing"},
		},
	}

	if inputs != nil {
		// Parse prefixes
		if prefixes, ok := inputs["prefixes"].([]any); ok {
			for _, p := range prefixes {
				if prefixMap, ok := p.(map[string]any); ok {
					prefix := EVPNPrefix{}
					if prefixStr, ok := prefixMap["prefix"].(string); ok {
						prefix.Prefix = prefixStr
					}
					if vni, ok := prefixMap["vni"].(float64); ok {
						prefix.VNI = int(vni)
					} else if vni, ok := prefixMap["vni"].(int); ok {
						prefix.VNI = vni
					}
					if prefix.Prefix != "" && prefix.VNI > 0 {
						t.Prefixes = append(t.Prefixes, prefix)
					}
				}
			}
		}

		// Parse specific routes
		if routes, ok := inputs["routes"].([]any); ok {
			for _, r := range routes {
				if routeMap, ok := r.(map[string]any); ok {
					route := EVPNRoute{}
					if rd, ok := routeMap["rd"].(string); ok {
						route.RD = rd
					}
					if domain, ok := routeMap["domain"].(string); ok {
						route.Domain = domain
					}
					if prefixStr, ok := routeMap["prefix"].(string); ok {
						route.Prefix = prefixStr
					}
					if vni, ok := routeMap["vni"].(float64); ok {
						route.VNI = int(vni)
					} else if vni, ok := routeMap["vni"].(int); ok {
						route.VNI = vni
					}
					if route.RD != "" && route.Prefix != "" {
						t.Routes = append(t.Routes, route)
					}
				}
			}
		}

		// Parse specific paths
		if paths, ok := inputs["paths"].([]any); ok {
			for _, p := range paths {
				if pathMap, ok := p.(map[string]any); ok {
					path := EVPNPath{}
					if prefixStr, ok := pathMap["prefix"].(string); ok {
						path.Prefix = prefixStr
					}
					if vni, ok := pathMap["vni"].(float64); ok {
						path.VNI = int(vni)
					} else if vni, ok := pathMap["vni"].(int); ok {
						path.VNI = vni
					}
					if nexthop, ok := pathMap["nexthop"].(string); ok {
						path.NextHop = nexthop
					}
					if rts, ok := pathMap["route_targets"].([]any); ok {
						for _, rt := range rts {
							if rtStr, ok := rt.(string); ok {
								path.RouteTargets = append(path.RouteTargets, rtStr)
							}
						}
					}
					if path.Prefix != "" && path.VNI > 0 {
						t.Paths = append(t.Paths, path)
					}
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyEVPNType5Routes) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp evpn route-type ip-prefix ipv4",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get EVPN Type-5 routes: %v", err)
		return result, nil
	}

	issues := []string{}
	deviceRoutes := make(map[string][]DeviceEVPNRoute)

	evpnData, ok := cmdResult.Output.(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to parse EVPN output"
		return result, nil
	}

	vrfRoutes, ok := evpnData["vrf"].(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "No VRF data found in EVPN output"
		return result, nil
	}

	for vrfName, vrfData := range vrfRoutes {
		vrf, ok := vrfData.(map[string]any)
		if !ok {
			continue
		}

		bgpRouteEntries, ok := vrf["bgpRouteEntries"].(map[string]any)
		if !ok {
			continue
		}

		for prefix, routeData := range bgpRouteEntries {
			route, ok := routeData.(map[string]any)
			if !ok {
				continue
			}

			deviceRoute := DeviceEVPNRoute{
				Prefix: prefix,
				VRF:    vrfName,
			}

			// Parse VNI
			if vni, ok := route["vni"].(float64); ok {
				deviceRoute.VNI = int(vni)
			}

			// Parse Route Distinguisher
			if rd, ok := route["routeDistinguisher"].(string); ok {
				deviceRoute.RD = rd
			}

			// Parse paths
			deviceRoute.Paths = parseBGPRoutePaths(route)

			deviceRoutes[prefix] = append(deviceRoutes[prefix], deviceRoute)
		}
	}

	// Check prefixes (basic level)
	for _, expectedPrefix := range t.Prefixes {
		found := false
		hasActiveValidPath := false

		if routes, exists := deviceRoutes[expectedPrefix.Prefix]; exists {
			for _, route := range routes {
				if route.VNI == expectedPrefix.VNI {
					found = true
					for _, path := range route.Paths {
						if path.Active && path.Valid {
							hasActiveValidPath = true
							break
						}
					}
					break
				}
			}
		}

		if !found {
			issues = append(issues, fmt.Sprintf("EVPN Type-5 route for prefix %s VNI %d not found",
				expectedPrefix.Prefix, expectedPrefix.VNI))
		} else if !hasActiveValidPath {
			issues = append(issues, fmt.Sprintf("EVPN Type-5 route for prefix %s VNI %d has no active and valid paths",
				expectedPrefix.Prefix, expectedPrefix.VNI))
		}
	}

	// Check specific routes
	for _, expectedRoute := range t.Routes {
		found := false

		if routes, exists := deviceRoutes[expectedRoute.Prefix]; exists {
			for _, route := range routes {
				if route.RD == expectedRoute.RD && route.VNI == expectedRoute.VNI {
					if expectedRoute.Domain == "" || route.VRF == expectedRoute.Domain {
						found = true
						break
					}
				}
			}
		}

		if !found {
			issues = append(issues, fmt.Sprintf("EVPN Type-5 specific route RD %s prefix %s VNI %d not found",
				expectedRoute.RD, expectedRoute.Prefix, expectedRoute.VNI))
		}
	}

	// Check specific paths
	for _, expectedPath := range t.Paths {
		found := false

		if routes, exists := deviceRoutes[expectedPath.Prefix]; exists {
			for _, route := range routes {
				if route.VNI == expectedPath.VNI {
					for _, path := range route.Paths {
						pathMatches := true

						if expectedPath.NextHop != "" && path.NextHop != expectedPath.NextHop {
							pathMatches = false
						}

						if len(expectedPath.RouteTargets) > 0 {
							for _, expectedRT := range expectedPath.RouteTargets {
								rtFound := false
								for _, deviceRT := range path.RouteTargets {
									if deviceRT == expectedRT {
										rtFound = true
										break
									}
								}
								if !rtFound {
									pathMatches = false
									break
								}
							}
						}

						if pathMatches {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
		}

		if !found {
			issues = append(issues, fmt.Sprintf("EVPN Type-5 path for prefix %s VNI %d with specified criteria not found",
				expectedPath.Prefix, expectedPath.VNI))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("EVPN Type-5 route issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"checked_prefixes":    len(t.Prefixes),
			"checked_routes":      len(t.Routes),
			"checked_paths":       len(t.Paths),
			"total_device_routes": len(deviceRoutes),
		}
	}

	return result, nil
}

// parseBGPRoutePaths extracts BGP paths from a route data structure
func parseBGPRoutePaths(route map[string]any) []EVPNRoutePath {
	var paths []EVPNRoutePath

	bgpRoutePaths, ok := route["bgpRoutePaths"].([]any)
	if !ok {
		return paths
	}

	for _, pathData := range bgpRoutePaths {
		path, ok := pathData.(map[string]any)
		if !ok {
			continue
		}

		routePath := EVPNRoutePath{}

		if valid, ok := path["valid"].(bool); ok {
			routePath.Valid = valid
		}
		if active, ok := path["active"].(bool); ok {
			routePath.Active = active
		}
		if nexthop, ok := path["nexthop"].(string); ok {
			routePath.NextHop = nexthop
		}

		routePath.RouteTargets = parseRouteTargets(path)
		paths = append(paths, routePath)
	}

	return paths
}

// parseRouteTargets extracts route targets from path extended communities
func parseRouteTargets(path map[string]any) []string {
	var routeTargets []string

	routeDetail, ok := path["routeDetail"].(map[string]any)
	if !ok {
		return routeTargets
	}

	extCommunityList, ok := routeDetail["extCommunityList"].([]any)
	if !ok {
		return routeTargets
	}

	for _, extComm := range extCommunityList {
		comm, ok := extComm.(map[string]any)
		if !ok {
			continue
		}

		commType, ok := comm["type"].(string)
		if !ok || commType != "routeTarget" {
			continue
		}

		if value, ok := comm["value"].(string); ok {
			routeTargets = append(routeTargets, value)
		}
	}

	return routeTargets
}

func (t *VerifyEVPNType5Routes) ValidateInput(input any) error {
	if len(t.Prefixes) == 0 && len(t.Routes) == 0 && len(t.Paths) == 0 {
		return fmt.Errorf("at least one prefix, route, or path must be specified")
	}

	// Validate prefixes
	for i, prefix := range t.Prefixes {
		if prefix.Prefix == "" {
			return fmt.Errorf("prefix at index %d has empty prefix", i)
		}
		if _, _, err := net.ParseCIDR(prefix.Prefix); err != nil {
			return fmt.Errorf("prefix at index %d has invalid CIDR format: %s", i, prefix.Prefix)
		}
		if prefix.VNI < 1 || prefix.VNI > 16777215 {
			return fmt.Errorf("prefix at index %d has invalid VNI %d (must be 1-16777215)", i, prefix.VNI)
		}
	}

	// Validate routes
	for i, route := range t.Routes {
		if route.RD == "" {
			return fmt.Errorf("route at index %d has empty route distinguisher", i)
		}
		if route.Prefix == "" {
			return fmt.Errorf("route at index %d has empty prefix", i)
		}
		if _, _, err := net.ParseCIDR(route.Prefix); err != nil {
			return fmt.Errorf("route at index %d has invalid CIDR format: %s", i, route.Prefix)
		}
		if route.VNI < 1 || route.VNI > 16777215 {
			return fmt.Errorf("route at index %d has invalid VNI %d (must be 1-16777215)", i, route.VNI)
		}
	}

	// Validate paths
	for i, path := range t.Paths {
		if path.Prefix == "" {
			return fmt.Errorf("path at index %d has empty prefix", i)
		}
		if _, _, err := net.ParseCIDR(path.Prefix); err != nil {
			return fmt.Errorf("path at index %d has invalid CIDR format: %s", i, path.Prefix)
		}
		if path.VNI < 1 || path.VNI > 16777215 {
			return fmt.Errorf("path at index %d has invalid VNI %d (must be 1-16777215)", i, path.VNI)
		}
		if path.NextHop != "" && net.ParseIP(path.NextHop) == nil {
			return fmt.Errorf("path at index %d has invalid nexthop IP: %s", i, path.NextHop)
		}
	}

	return nil
}

type DeviceEVPNRoute struct {
	Prefix string
	VNI    int
	RD     string
	VRF    string
	Paths  []EVPNRoutePath
}

type EVPNRoutePath struct {
	Active       bool
	Valid        bool
	NextHop      string
	RouteTargets []string
}
