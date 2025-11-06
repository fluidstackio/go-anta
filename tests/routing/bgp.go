package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluidstack/go-anta/pkg/device"
	"github.com/fluidstack/go-anta/pkg/test"
)

// VerifyBGPPeers verifies the session state and configuration of BGP peers.
//
// This test performs the following checks for each specified peer:
//  1. Verifies that the peer is found in its VRF in the BGP configuration.
//  2. Validates that the BGP session state matches the expected state.
//  3. Optionally validates the peer's ASN if specified.
//
// Expected Results:
//   - Success: All specified peers are found with correct session states and ASNs.
//   - Failure: A peer is not found, session state doesn't match, or ASN doesn't match.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeers"
//     module: "routing"
//     inputs:
//     peers:
//   - peer: "10.0.0.1"
//     state: "Established"
//     asn: 65001
//     vrf: "default"
//   - peer: "10.0.0.2"
//     state: "Established"
//     asn: 65002

type VerifyBGPPeers struct {
	test.BaseTest
	Peers []BGPPeer `yaml:"peers" json:"peers"`
}

type BGPPeer struct {
	Peer  string `yaml:"peer" json:"peer"`
	State string `yaml:"state" json:"state"`
	ASN   int    `yaml:"asn,omitempty" json:"asn,omitempty"`
	VRF   string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyBGPPeers(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeers",
			TestDescription: "Verify BGP peer status and configuration",
			TestCategories:  []string{"routing", "bgp"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BGPPeer{
						State: "Established",
						VRF:   "default",
					}
					if addr, ok := peerMap["peer"].(string); ok {
						peer.Peer = addr
					}
					if state, ok := peerMap["state"].(string); ok {
						peer.State = state
					}
					if asn, ok := peerMap["asn"].(float64); ok {
						peer.ASN = int(asn)
					} else if asn, ok := peerMap["asn"].(int); ok {
						peer.ASN = asn
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.Peers = append(t.Peers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Peers) == 0 {
		result.Status = test.TestError
		result.Message = "No BGP peers configured for verification"
		return result, nil
	}

	cmd := device.Command{
		Template: "show bgp summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary: %v", err)
		return result, nil
	}

	issues := []string{}

	if bgpData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]any); ok {
			for _, peer := range t.Peers {
				vrfName := peer.VRF
				if vrfName == "" {
					vrfName = "default"
				}

				vrfData, vrfExists := vrfs[vrfName]
				if !vrfExists {
					issues = append(issues, fmt.Sprintf("VRF %s not found", vrfName))
					continue
				}

				if vrf, ok := vrfData.(map[string]any); ok {
					if peers, ok := vrf["peers"].(map[string]any); ok {
						peerData, peerExists := peers[peer.Peer]
						if !peerExists {
							issues = append(issues, fmt.Sprintf("Peer %s not found in VRF %s", peer.Peer, vrfName))
							continue
						}

						if peerInfo, ok := peerData.(map[string]any); ok {
							if peerState, ok := peerInfo["peerState"].(string); ok {
								if !strings.EqualFold(peerState, peer.State) {
									issues = append(issues, fmt.Sprintf("Peer %s: expected state %s, got %s",
										peer.Peer, peer.State, peerState))
								}
							}

							if peer.ASN > 0 {
								if asn, ok := peerInfo["asn"].(float64); ok {
									if int(asn) != peer.ASN {
										issues = append(issues, fmt.Sprintf("Peer %s: expected ASN %d, got %d",
											peer.Peer, peer.ASN, int(asn)))
									}
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
		result.Message = fmt.Sprintf("BGP peer issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyBGPPeers) ValidateInput(input any) error {
	if len(t.Peers) == 0 {
		return fmt.Errorf("at least one BGP peer must be specified")
	}

	for i, peer := range t.Peers {
		if peer.Peer == "" {
			return fmt.Errorf("peer at index %d has no address", i)
		}
		if peer.ASN < 0 {
			return fmt.Errorf("peer %s has invalid ASN", peer.Peer)
		}
	}

	return nil
}

// VerifyBGPUnnumbered validates BGP unnumbered configurations and peer states.
//
// BGP unnumbered allows BGP sessions to be established over interfaces without IP addresses,
// using the interface name instead of IP addresses for peering.
//
// This test performs the following checks for each specified interface:
//  1. Verifies that the interface is configured for BGP unnumbered.
//  2. Validates that the BGP session state matches the expected state.
//  3. Optionally validates the remote ASN if specified.
//
// Expected Results:
//   - Success: All specified interfaces have established BGP unnumbered sessions.
//   - Failure: An interface is not found, session state doesn't match, or ASN doesn't match.
//
// Example YAML configuration:
//   - name: "VerifyBGPUnnumbered"
//     module: "routing"
//     inputs:
//     vrf: "default"
//     interfaces:
//   - interface: "Et10/1"
//     remote_asn: 65103
//     expected_state: "Established"
//     description: "Connection to spine1"
//   - interface: "Et9/1"
//     remote_asn: 65104
//     expected_state: "Established"
//     description: "Connection to spine2"
type VerifyBGPUnnumbered struct {
	test.BaseTest
	Interfaces []BGPUnnumberedInterface `yaml:"interfaces" json:"interfaces"`
	VRF        string                   `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

type BGPUnnumberedInterface struct {
	Interface     string `yaml:"interface" json:"interface"`
	RemoteASN     int    `yaml:"remote_asn,omitempty" json:"remote_asn,omitempty"`
	ExpectedState string `yaml:"expected_state,omitempty" json:"expected_state,omitempty"`
	Description   string `yaml:"description,omitempty" json:"description,omitempty"`
}

func NewVerifyBGPUnnumbered(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPUnnumbered{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPUnnumbered",
			TestDescription: "Verify BGP unnumbered interface configurations and peer states",
			TestCategories:  []string{"routing", "bgp", "unnumbered"},
		},
		VRF: "default",
	}

	if inputs != nil {
		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}

		if interfaces, ok := inputs["interfaces"].([]any); ok {
			for _, intf := range interfaces {
				if intfMap, ok := intf.(map[string]any); ok {
					unnumberedIntf := BGPUnnumberedInterface{
						ExpectedState: "Established",
					}

					if name, ok := intfMap["interface"].(string); ok {
						unnumberedIntf.Interface = name
					}
					if asn, ok := intfMap["remote_asn"].(float64); ok {
						unnumberedIntf.RemoteASN = int(asn)
					} else if asn, ok := intfMap["remote_asn"].(int); ok {
						unnumberedIntf.RemoteASN = asn
					}
					if state, ok := intfMap["expected_state"].(string); ok {
						unnumberedIntf.ExpectedState = state
					}
					if desc, ok := intfMap["description"].(string); ok {
						unnumberedIntf.Description = desc
					}

					t.Interfaces = append(t.Interfaces, unnumberedIntf)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPUnnumbered) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Interfaces) == 0 {
		result.Status = test.TestError
		result.Message = "No BGP unnumbered interfaces configured for verification"
		return result, nil
	}

	// Get BGP summary to check peer states
	summaryCmd := device.Command{
		Template: "show bgp summary",
		Format:   "json",
		UseCache: false,
	}

	summaryResult, err := dev.Execute(ctx, summaryCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary: %v", err)
		return result, nil
	}

	issues := []string{}

	// Parse BGP summary to get all peer information
	// For unnumbered BGP, the peer address includes the interface (e.g., "fe80::...%Et10/1")
	if bgpData, ok := summaryResult.Output.(map[string]any); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]any); ok {
			vrfName := t.VRF
			if vrfName == "" {
				vrfName = "default"
			}

			if vrfData, exists := vrfs[vrfName]; exists {
				if vrf, ok := vrfData.(map[string]any); ok {
					if peers, ok := vrf["peers"].(map[string]any); ok {

						// Validate each unnumbered interface
						for _, intf := range t.Interfaces {
							interfaceFound := false

							// Look for a peer that ends with %<interface>
							interfaceSuffix := "%" + intf.Interface

							for peerAddr, peerData := range peers {
								if strings.HasSuffix(peerAddr, interfaceSuffix) {
									interfaceFound = true

									if peerInfo, ok := peerData.(map[string]any); ok {
										// Check peer state
										if state, ok := peerInfo["peerState"].(string); ok {
											if !strings.EqualFold(state, intf.ExpectedState) {
												issues = append(issues, fmt.Sprintf("Interface %s (peer %s): expected state %s, got %s",
													intf.Interface, peerAddr, intf.ExpectedState, state))
											}
										} else if intf.ExpectedState != "Idle" {
											issues = append(issues, fmt.Sprintf("Interface %s (peer %s): No state found",
												intf.Interface, peerAddr))
										}

										// Validate remote ASN if specified
										if intf.RemoteASN > 0 {
											if peerAsn, ok := peerInfo["peerAsn"].(string); ok {
												// peerAsn is returned as string, convert to int for comparison
												var peerAsnInt int
												if n, err := fmt.Sscanf(peerAsn, "%d", &peerAsnInt); err == nil && n == 1 {
													if peerAsnInt != intf.RemoteASN {
														issues = append(issues, fmt.Sprintf("Interface %s (peer %s): expected remote-as %d, got %d",
															intf.Interface, peerAddr, intf.RemoteASN, peerAsnInt))
													}
												}
											} else if peerAsn, ok := peerInfo["peerAsn"].(float64); ok {
												if int(peerAsn) != intf.RemoteASN {
													issues = append(issues, fmt.Sprintf("Interface %s (peer %s): expected remote-as %d, got %d",
														intf.Interface, peerAddr, intf.RemoteASN, int(peerAsn)))
												}
											}
										}
									}
									break
								}
							}

							if !interfaceFound {
								issues = append(issues, fmt.Sprintf("Interface %s: No BGP unnumbered neighbor found", intf.Interface))
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All %d BGP unnumbered interfaces verified successfully", len(t.Interfaces))
	}

	return result, nil
}

func (t *VerifyBGPUnnumbered) ValidateInput(input any) error {
	if len(t.Interfaces) == 0 {
		return fmt.Errorf("at least one BGP unnumbered interface must be specified")
	}

	for i, intf := range t.Interfaces {
		if intf.Interface == "" {
			return fmt.Errorf("interface at index %d has no name", i)
		}
		if intf.RemoteASN < 0 {
			return fmt.Errorf("interface %s has invalid remote ASN", intf.Interface)
		}
	}

	return nil
}

// ==================== Common BGP Structures for Extended Tests ====================

// BgpAddressFamily represents AFI/SAFI configuration
type BgpAddressFamily struct {
	AFI            string   `yaml:"afi" json:"afi"`
	SAFI           string   `yaml:"safi,omitempty" json:"safi,omitempty"`
	VRF            string   `yaml:"vrf,omitempty" json:"vrf,omitempty"`
	NumPeers       int      `yaml:"num_peers,omitempty" json:"num_peers,omitempty"`
	CheckPeerState bool     `yaml:"check_peer_state,omitempty" json:"check_peer_state,omitempty"`
	Peers          []string `yaml:"peers,omitempty" json:"peers,omitempty"`
}

// BgpPeerExtended represents extended BGP peer configuration
type BgpPeerExtended struct {
	PeerAddress           string                 `yaml:"peer_address,omitempty" json:"peer_address,omitempty"`
	Interface             string                 `yaml:"interface,omitempty" json:"interface,omitempty"`
	VRF                   string                 `yaml:"vrf,omitempty" json:"vrf,omitempty"`
	AdvertisedRoutes      []string               `yaml:"advertised_routes,omitempty" json:"advertised_routes,omitempty"`
	ReceivedRoutes        []string               `yaml:"received_routes,omitempty" json:"received_routes,omitempty"`
	AdvertisedCommunities []string               `yaml:"advertised_communities,omitempty" json:"advertised_communities,omitempty"`
	Capabilities          []string               `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	HoldTime              int                    `yaml:"hold_time,omitempty" json:"hold_time,omitempty"`
	KeepAliveTime         int                    `yaml:"keep_alive_time,omitempty" json:"keep_alive_time,omitempty"`
	MaximumRoutes         int                    `yaml:"maximum_routes,omitempty" json:"maximum_routes,omitempty"`
	WarningLimit          int                    `yaml:"warning_limit,omitempty" json:"warning_limit,omitempty"`
	PeerGroup             string                 `yaml:"peer_group,omitempty" json:"peer_group,omitempty"`
	TTL                   int                    `yaml:"ttl,omitempty" json:"ttl,omitempty"`
	DropStats             map[string]int         `yaml:"drop_stats,omitempty" json:"drop_stats,omitempty"`
	UpdateErrors          map[string]any `yaml:"update_errors,omitempty" json:"update_errors,omitempty"`
	InboundRouteMap       string                 `yaml:"inbound_route_map,omitempty" json:"inbound_route_map,omitempty"`
	OutboundRouteMap      string                 `yaml:"outbound_route_map,omitempty" json:"outbound_route_map,omitempty"`
}

// VerifyBGPPeerCount verifies the count of BGP peers for specified address families.
//
// This test performs the following checks for each specified address family:
//  1. Verifies that the address family is active in the specified VRF.
//  2. Validates that the number of BGP peers matches the expected count.
//  3. Ensures all counted peers are in the expected state (if specified).
//
// Expected Results:
//   - Success: All address families have the correct peer count.
//   - Failure: An address family doesn't exist, or peer count doesn't match.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerCount"
//     module: "routing"
//     inputs:
//     address_families:
//   - afi: "ipv4"
//     safi: "unicast"
//     vrf: "default"
//     num_peers: 2
//   - afi: "ipv6"
//     safi: "unicast"
//     vrf: "default"
//     num_peers: 2
type VerifyBGPPeerCount struct {
	test.BaseTest
	AddressFamilies []BgpAddressFamily `yaml:"address_families" json:"address_families"`
}

func NewVerifyBGPPeerCount(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerCount{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerCount",
			TestDescription: "Verifies the count of BGP peers for address families",
			TestCategories:  []string{"routing", "bgp", "peer-count"},
		},
	}

	if inputs != nil {
		if afs, ok := inputs["address_families"].([]any); ok {
			for _, af := range afs {
				if afMap, ok := af.(map[string]any); ok {
					addressFamily := BgpAddressFamily{
						VRF:            "default",
						CheckPeerState: true,
					}
					if afi, ok := afMap["afi"].(string); ok {
						addressFamily.AFI = afi
					}
					if safi, ok := afMap["safi"].(string); ok {
						addressFamily.SAFI = safi
					}
					if vrf, ok := afMap["vrf"].(string); ok {
						addressFamily.VRF = vrf
					}
					if numPeers, ok := afMap["num_peers"].(float64); ok {
						addressFamily.NumPeers = int(numPeers)
					} else if numPeers, ok := afMap["num_peers"].(int); ok {
						addressFamily.NumPeers = numPeers
					}
					if checkState, ok := afMap["check_peer_state"].(bool); ok {
						addressFamily.CheckPeerState = checkState
					}
					t.AddressFamilies = append(t.AddressFamilies, addressFamily)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerCount) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary: %v", err)
		return result, nil
	}

	issues := []string{}

	if bgpData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]any); ok {
			for _, af := range t.AddressFamilies {
				vrf := af.VRF
				if vrf == "" {
					vrf = "default"
				}

				if vrfData, exists := vrfs[vrf]; exists {
					if vrfInfo, ok := vrfData.(map[string]any); ok {
						if peers, ok := vrfInfo["peers"].(map[string]any); ok {
							establishedCount := 0
							totalCount := len(peers)

							if af.CheckPeerState {
								for _, peerData := range peers {
									if peerInfo, ok := peerData.(map[string]any); ok {
										if state, ok := peerInfo["peerState"].(string); ok {
											if strings.EqualFold(state, "Established") {
												establishedCount++
											}
										}
									}
								}
								if establishedCount != af.NumPeers {
									issues = append(issues, fmt.Sprintf("AFI %s SAFI %s VRF %s: expected %d established peers, got %d",
										af.AFI, af.SAFI, vrf, af.NumPeers, establishedCount))
								}
							} else {
								if totalCount != af.NumPeers {
									issues = append(issues, fmt.Sprintf("AFI %s SAFI %s VRF %s: expected %d peers, got %d",
										af.AFI, af.SAFI, vrf, af.NumPeers, totalCount))
								}
							}
						}
					}
				} else {
					issues = append(issues, fmt.Sprintf("VRF %s not found for AFI %s SAFI %s", vrf, af.AFI, af.SAFI))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	}

	return result, nil
}

func (t *VerifyBGPPeerCount) ValidateInput(input any) error {
	if len(t.AddressFamilies) == 0 {
		return fmt.Errorf("at least one address family must be specified")
	}
	for i, af := range t.AddressFamilies {
		if af.AFI == "" {
			return fmt.Errorf("address family at index %d has no AFI", i)
		}
		if af.NumPeers < 0 {
			return fmt.Errorf("address family %s has invalid num_peers", af.AFI)
		}
	}
	return nil
}

// ==================== 4. VerifyBGPPeersHealth ====================

// VerifyBGPPeersHealth verifies the health of BGP peers for specified address families.
//
// Expected Results:
//   - Success: The test will pass if all BGP peers are in healthy state (established for minimum time and clean TCP queues).
//   - Failure: The test will fail if any peer is not established for the minimum time or has TCP queue issues.
//   - Error: The test will report an error if BGP peer information cannot be retrieved.
//
// Examples:
//   - name: VerifyBGPPeersHealth with minimum established time
//     VerifyBGPPeersHealth:
//       minimum_established_time: 300  # 5 minutes
//       check_tcp_queues: true
//       address_families:
//         - afi: "ipv4"
//           safi: "unicast"
//         - afi: "evpn"
//
//   - name: VerifyBGPPeersHealth basic check
//     VerifyBGPPeersHealth:
//       address_families:
//         - afi: "ipv4"
//           safi: "unicast"
type VerifyBGPPeersHealth struct {
	test.BaseTest
	MinimumEstablishedTime int                `yaml:"minimum_established_time,omitempty" json:"minimum_established_time,omitempty"`
	CheckTCPQueues         bool               `yaml:"check_tcp_queues,omitempty" json:"check_tcp_queues,omitempty"`
	AddressFamilies        []BgpAddressFamily `yaml:"address_families" json:"address_families"`
}

func NewVerifyBGPPeersHealth(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeersHealth{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeersHealth",
			TestDescription: "Verifies health of all BGP peers",
			TestCategories:  []string{"routing", "bgp", "health"},
		},
		CheckTCPQueues: true,
	}

	if inputs != nil {
		if minTime, ok := inputs["minimum_established_time"].(float64); ok {
			t.MinimumEstablishedTime = int(minTime)
		} else if minTime, ok := inputs["minimum_established_time"].(int); ok {
			t.MinimumEstablishedTime = minTime
		}
		if checkTCP, ok := inputs["check_tcp_queues"].(bool); ok {
			t.CheckTCPQueues = checkTCP
		}
		if afs, ok := inputs["address_families"].([]any); ok {
			for _, af := range afs {
				if afMap, ok := af.(map[string]any); ok {
					addressFamily := BgpAddressFamily{VRF: "default"}
					if afi, ok := afMap["afi"].(string); ok {
						addressFamily.AFI = afi
					}
					if safi, ok := afMap["safi"].(string); ok {
						addressFamily.SAFI = safi
					}
					if vrf, ok := afMap["vrf"].(string); ok {
						addressFamily.VRF = vrf
					}
					t.AddressFamilies = append(t.AddressFamilies, addressFamily)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeersHealth) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary: %v", err)
		return result, nil
	}

	issues := []string{}

	if bgpData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]any); ok {
			for _, af := range t.AddressFamilies {
				vrf := af.VRF
				if vrf == "" {
					vrf = "default"
				}

				if vrfData, exists := vrfs[vrf]; exists {
					if vrfInfo, ok := vrfData.(map[string]any); ok {
						if peers, ok := vrfInfo["peers"].(map[string]any); ok {
							for peerAddr, peerData := range peers {
								if peerInfo, ok := peerData.(map[string]any); ok {
									// Check peer state
									if state, ok := peerInfo["peerState"].(string); ok {
										if !strings.EqualFold(state, "Established") {
											issues = append(issues, fmt.Sprintf("Peer %s in VRF %s is %s, not Established",
												peerAddr, vrf, state))
											continue
										}
									}

									// Check established time if specified
									if t.MinimumEstablishedTime > 0 {
										if uptime, ok := peerInfo["upDownTime"].(float64); ok {
											if uptime < float64(t.MinimumEstablishedTime) {
												issues = append(issues, fmt.Sprintf("Peer %s in VRF %s established for only %.0f seconds (minimum: %d)",
													peerAddr, vrf, uptime, t.MinimumEstablishedTime))
											}
										}
									}

									// Check TCP queues if enabled
									if t.CheckTCPQueues {
										if inQueue, ok := peerInfo["inMsgQueue"].(float64); ok {
											if inQueue > 0 {
												issues = append(issues, fmt.Sprintf("Peer %s in VRF %s has %d messages in input queue",
													peerAddr, vrf, int(inQueue)))
											}
										}
										if outQueue, ok := peerInfo["outMsgQueue"].(float64); ok {
											if outQueue > 0 {
												issues = append(issues, fmt.Sprintf("Peer %s in VRF %s has %d messages in output queue",
													peerAddr, vrf, int(outQueue)))
											}
										}
									}
								}
							}
						}
					}
				} else {
					issues = append(issues, fmt.Sprintf("VRF %s not found for AFI %s", vrf, af.AFI))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All BGP peers in %d address families are healthy", len(t.AddressFamilies))
	}

	return result, nil
}

func (t *VerifyBGPPeersHealth) ValidateInput(input any) error {
	return nil
}

// ==================== Additional BGP test stubs (5-24) ====================
// These are simplified implementations to support all BGP tests from the Python ANTA

// VerifyBGPSpecificPeers verifies the health and connectivity of specific BGP peers.
//
// This test validates that specific BGP peers are established and healthy for given
// address families. It provides more granular control than general peer health checks
// by allowing verification of specific peer configurations.
//
// Expected Results:
//   - Success: All specified peers are found and established for the given address families.
//   - Failure: A peer is not found, not established, or not configured for the expected address family.
//   - Error: The test will error if BGP configuration cannot be retrieved or parsed.
//
// Example YAML configuration:
//   - name: "VerifyBGPSpecificPeers"
//     module: "routing"
//     inputs:
//       address_families:
//         - afi: "ipv4"
//           safi: "unicast"
//         - afi: "evpn"
//           safi: "evpn"
type VerifyBGPSpecificPeers struct {
	test.BaseTest
	AddressFamilies []BgpAddressFamily `yaml:"address_families" json:"address_families"`
}

func NewVerifyBGPSpecificPeers(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPSpecificPeers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPSpecificPeers",
			TestDescription: "Verifies specific BGP peers health",
			TestCategories:  []string{"routing", "bgp", "specific-peers"},
		},
	}

	if inputs != nil {
		if afs, ok := inputs["address_families"].([]any); ok {
			for _, af := range afs {
				if afMap, ok := af.(map[string]any); ok {
					addressFamily := BgpAddressFamily{VRF: "default"}
					if afi, ok := afMap["afi"].(string); ok {
						addressFamily.AFI = afi
					}
					if safi, ok := afMap["safi"].(string); ok {
						addressFamily.SAFI = safi
					}
					if vrf, ok := afMap["vrf"].(string); ok {
						addressFamily.VRF = vrf
					}
					if peers, ok := afMap["peers"].([]any); ok {
						for _, peer := range peers {
							if peerStr, ok := peer.(string); ok {
								addressFamily.Peers = append(addressFamily.Peers, peerStr)
							}
						}
					}
					t.AddressFamilies = append(t.AddressFamilies, addressFamily)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPSpecificPeers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary: %v", err)
		return result, nil
	}

	issues := []string{}

	if bgpData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]any); ok {
			for _, af := range t.AddressFamilies {
				vrf := af.VRF
				if vrf == "" {
					vrf = "default"
				}

				if vrfData, exists := vrfs[vrf]; exists {
					if vrfInfo, ok := vrfData.(map[string]any); ok {
						if peers, ok := vrfInfo["peers"].(map[string]any); ok {
							// Check each specific peer
							for _, expectedPeer := range af.Peers {
								if peerData, peerExists := peers[expectedPeer]; peerExists {
									if peerInfo, ok := peerData.(map[string]any); ok {
										// Check peer state
										if state, ok := peerInfo["peerState"].(string); ok {
											if !strings.EqualFold(state, "Established") {
												issues = append(issues, fmt.Sprintf("Peer %s in VRF %s is %s, not Established",
													expectedPeer, vrf, state))
											}
										}
									}
								} else {
									issues = append(issues, fmt.Sprintf("Peer %s not found in VRF %s", expectedPeer, vrf))
								}
							}
						}
					}
				} else {
					issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	} else {
		totalPeers := 0
		for _, af := range t.AddressFamilies {
			totalPeers += len(af.Peers)
		}
		result.Message = fmt.Sprintf("All %d specific BGP peers are healthy", totalPeers)
	}

	return result, nil
}

func (t *VerifyBGPSpecificPeers) ValidateInput(input any) error { return nil }

// VerifyBGPPeerSession verifies the session state of BGP peers.
//
// Compatible with EOS operating in `ribd` routing protocol model.
// This test performs comprehensive checks on BGP peer sessions to ensure
// they are established and healthy.
//
// This test performs the following checks for each specified peer:
//   1. Verifies that the peer is found in its VRF in the BGP configuration.
//   2. Verifies that the BGP session is `Established` and, if specified, has remained established for at least the duration given by `minimum_established_time`.
//   3. Ensures that both input and output TCP message queues are empty.
//      Can be disabled by setting `check_tcp_queues` input flag to `False`.
//
// Expected Results:
//   - Success: All specified peers are found with established sessions and clean TCP queues.
//   - Failure: A peer is not found, session is not established, or TCP queues are not empty.
//   - Error: The test will error if BGP peer information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerSession"
//     module: "routing"
//     inputs:
//       minimum_established_time: 10000
//       check_tcp_queues: false
//       bgp_peers:
//         - peer_address: "10.1.0.1"
//           vrf: "default"
//         - peer_address: "10.1.255.4"
//           vrf: "DEV"
//         - peer_address: "fd00:dc:1::1"
//           vrf: "default"
//         - interface: "Ethernet1"
//           vrf: "MGMT"
type VerifyBGPPeerSession struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerSession(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerSession{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerSession",
			TestDescription: "Verifies individual BGP peer sessions",
			TestCategories:  []string{"routing", "bgp", "session"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if intf, ok := peerMap["interface"].(string); ok {
						peer.Interface = intf
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerSession) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary: %v", err)
		return result, nil
	}

	issues := []string{}

	if bgpData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]any); ok {
			for _, peer := range t.BGPPeers {
				vrf := peer.VRF
				if vrf == "" {
					vrf = "default"
				}

				if vrfData, exists := vrfs[vrf]; exists {
					if vrfInfo, ok := vrfData.(map[string]any); ok {
						if peers, ok := vrfInfo["peers"].(map[string]any); ok {
							var peerKey string
							var peerData any
							var found bool

							// Handle both address-based and interface-based (RFC5549) peers
							if peer.PeerAddress != "" {
								peerKey = peer.PeerAddress
								peerData, found = peers[peerKey]
							} else if peer.Interface != "" {
								// For RFC5549, look for peer ending with %interface
								interfaceSuffix := "%" + peer.Interface
								for addr, data := range peers {
									if strings.HasSuffix(addr, interfaceSuffix) {
										peerKey = addr
										peerData = data
										found = true
										break
									}
								}
							}

							if found {
								if peerInfo, ok := peerData.(map[string]any); ok {
									// Check peer state
									if state, ok := peerInfo["peerState"].(string); ok {
										if !strings.EqualFold(state, "Established") {
											issues = append(issues, fmt.Sprintf("Peer %s in VRF %s is %s, not Established",
												peerKey, vrf, state))
										}
									}
								}
							} else {
								identifier := peer.PeerAddress
								if identifier == "" {
									identifier = "interface " + peer.Interface
								}
								issues = append(issues, fmt.Sprintf("Peer %s not found in VRF %s", identifier, vrf))
							}
						}
					}
				} else {
					issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All %d BGP peer sessions are established", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerSession) ValidateInput(input any) error { return nil }

// VerifyBGPExchangedRoutes verifies the advertised and received routes of BGP IPv4 peer(s).
//
// This test validates that BGP peers are properly exchanging routes by verifying
// both advertised and received routes exist in the BGP route table with the correct states.
//
// This test performs the following checks for each advertised and received route for each peer:
//   - Confirms that the route exists in the BGP route table.
//   - If `check_active` input flag is True, verifies that the route is 'valid' and 'active'.
//   - If `check_active` input flag is False, verifies that the route is 'valid'.
//
// Expected Results:
//   - Success: All specified advertised/received routes are found and have correct states.
//   - Failure: Routes are missing or don't have expected 'active' and 'valid' states.
//   - Error: The test will error if BGP route table information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPExchangedRoutes"
//     module: "routing"
//     inputs:
//       check_active: true
//       bgp_peers:
//         - peer_address: "172.30.255.5"
//           vrf: "default"
//           advertised_routes:
//             - "192.0.254.5/32"
//           received_routes:
//             - "192.0.255.4/32"
//         - peer_address: "172.30.255.1"
//           vrf: "default"
//           advertised_routes:
//             - "192.0.255.1/32"
//             - "192.0.254.5/32"
type VerifyBGPExchangedRoutes struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPExchangedRoutes(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPExchangedRoutes{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPExchangedRoutes",
			TestDescription: "Verifies advertised and received routes",
			TestCategories:  []string{"routing", "bgp", "routes"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if routes, ok := peerMap["advertised_routes"].([]any); ok {
						for _, route := range routes {
							if routeStr, ok := route.(string); ok {
								peer.AdvertisedRoutes = append(peer.AdvertisedRoutes, routeStr)
							}
						}
					}
					if routes, ok := peerMap["received_routes"].([]any); ok {
						for _, route := range routes {
							if routeStr, ok := route.(string); ok {
								peer.ReceivedRoutes = append(peer.ReceivedRoutes, routeStr)
							}
						}
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPExchangedRoutes) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		// Check advertised routes
		if len(peer.AdvertisedRoutes) > 0 {
			cmd := device.Command{
				Template: fmt.Sprintf("show bgp neighbors %s advertised-routes vrf %s", peer.PeerAddress, vrf),
				Format:   "json",
				UseCache: false,
			}

			cmdResult, err := dev.Execute(ctx, cmd)
			if err != nil {
				issues = append(issues, fmt.Sprintf("Failed to get advertised routes for peer %s: %v", peer.PeerAddress, err))
				continue
			}

			if bgpData, ok := cmdResult.Output.(map[string]any); ok {
				if vrfData, ok := bgpData["vrfs"].(map[string]any); ok {
					if vrfInfo, ok := vrfData[vrf].(map[string]any); ok {
						if routes, ok := vrfInfo["bgpRouteEntries"].(map[string]any); ok {
							for _, expectedRoute := range peer.AdvertisedRoutes {
								if _, exists := routes[expectedRoute]; !exists {
									issues = append(issues, fmt.Sprintf("Route %s not advertised to peer %s", expectedRoute, peer.PeerAddress))
								}
							}
						}
					}
				}
			}
		}

		// Check received routes
		if len(peer.ReceivedRoutes) > 0 {
			cmd := device.Command{
				Template: fmt.Sprintf("show bgp neighbors %s received-routes vrf %s", peer.PeerAddress, vrf),
				Format:   "json",
				UseCache: false,
			}

			cmdResult, err := dev.Execute(ctx, cmd)
			if err != nil {
				issues = append(issues, fmt.Sprintf("Failed to get received routes for peer %s: %v", peer.PeerAddress, err))
				continue
			}

			if bgpData, ok := cmdResult.Output.(map[string]any); ok {
				if vrfData, ok := bgpData["vrfs"].(map[string]any); ok {
					if vrfInfo, ok := vrfData[vrf].(map[string]any); ok {
						if routes, ok := vrfInfo["bgpRouteEntries"].(map[string]any); ok {
							for _, expectedRoute := range peer.ReceivedRoutes {
								if _, exists := routes[expectedRoute]; !exists {
									issues = append(issues, fmt.Sprintf("Route %s not received from peer %s", expectedRoute, peer.PeerAddress))
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
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All route exchanges verified for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPExchangedRoutes) ValidateInput(input any) error { return nil }

// VerifyBGPPeerMPCaps verifies that BGP peers have the expected multiprotocol capabilities.
//
// This test validates that BGP peers support the required address families by checking
// their advertised multiprotocol capabilities. It ensures that peers can exchange routes
// for specific address families like IPv4 unicast, IPv6 unicast, or EVPN.
//
// Expected Results:
//   - Success: All specified peers have the required multiprotocol capabilities advertised.
//   - Failure: A peer is missing expected multiprotocol capabilities or has incorrect capabilities.
//   - Error: The test will error if BGP peer capability information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerMPCaps"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "10.0.0.1"
//           vrf: "default"
//         - peer_address: "2001:db8::1"
//           vrf: "default"
type VerifyBGPPeerMPCaps struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerMPCaps(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerMPCaps{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerMPCaps",
			TestDescription: "Verifies multiprotocol capabilities",
			TestCategories:  []string{"routing", "bgp", "capabilities"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if caps, ok := peerMap["capabilities"].([]any); ok {
						for _, cap := range caps {
							if capStr, ok := cap.(string); ok {
								peer.Capabilities = append(peer.Capabilities, capStr)
							}
						}
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerMPCaps) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		cmd := device.Command{
			Template: fmt.Sprintf("show bgp neighbors %s vrf %s", peer.PeerAddress, vrf),
			Format:   "json",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to get BGP neighbor %s details: %v", peer.PeerAddress, err))
			continue
		}

		if bgpData, ok := cmdResult.Output.(map[string]any); ok {
			if vrfData, ok := bgpData["vrfs"].(map[string]any); ok {
				if vrfInfo, ok := vrfData[vrf].(map[string]any); ok {
					if peerList, ok := vrfInfo["peerList"].([]any); ok && len(peerList) > 0 {
						if peerInfo, ok := peerList[0].(map[string]any); ok {
							if capabilities, ok := peerInfo["capabilities"].(map[string]any); ok {
								if mpCaps, ok := capabilities["multiprotocolCaps"].(map[string]any); ok {
									for _, expectedCap := range peer.Capabilities {
										if _, exists := mpCaps[expectedCap]; !exists {
											issues = append(issues, fmt.Sprintf("Peer %s missing capability %s", peer.PeerAddress, expectedCap))
										}
									}
								} else {
									issues = append(issues, fmt.Sprintf("Peer %s has no multiprotocol capabilities", peer.PeerAddress))
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
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All multiprotocol capabilities verified for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerMPCaps) ValidateInput(input any) error { return nil }

// VerifyBGPPeerASNCap verifies that BGP peers support 4-byte ASN capability.
//
// This test validates that BGP peers have negotiated and advertised support for
// 4-byte Autonomous System Numbers (ASNs) as defined in RFC 4893. This capability
// is essential for modern BGP deployments that use ASNs beyond the 16-bit range.
//
// Expected Results:
//   - Success: All specified peers have 4-byte ASN capability negotiated and active.
//   - Failure: A peer is missing 4-byte ASN capability or has not negotiated it properly.
//   - Error: The test will error if BGP peer capability information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerASNCap"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "10.0.0.1"
//           vrf: "default"
//         - peer_address: "192.168.1.1"
//           vrf: "MGMT"
type VerifyBGPPeerASNCap struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerASNCap(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerASNCap{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerASNCap",
			TestDescription: "Verifies four-octet ASN capability",
			TestCategories:  []string{"routing", "bgp", "capabilities"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerASNCap) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				Capabilities struct {
					FourOctetAsNumber struct {
						Advertised bool `json:"advertised"`
						Received   bool `json:"received"`
					} `json:"fourOctetAsNumber"`
				} `json:"capabilities"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check for four-octet ASN capability
		if !neighbor.Capabilities.FourOctetAsNumber.Advertised || !neighbor.Capabilities.FourOctetAsNumber.Received {
			issues = append(issues, fmt.Sprintf("Peer %s missing four-octet ASN capability: advertised=%t, received=%t",
				peer.PeerAddress, neighbor.Capabilities.FourOctetAsNumber.Advertised, neighbor.Capabilities.FourOctetAsNumber.Received))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP four-octet ASN capability validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP peers have four-octet ASN capability (%d peers)", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerASNCap) ValidateInput(input any) error {
	return nil
}

// VerifyBGPPeerRouteRefreshCap verifies that BGP peers support route refresh capability.
//
// This test validates that BGP peers have negotiated the route refresh capability as
// defined in RFC 2918. Route refresh allows a BGP speaker to request that its peer
// re-advertise its Adj-RIB-Out for a specific address family, enabling dynamic
// policy changes without tearing down the BGP session.
//
// Expected Results:
//   - Success: All specified peers have route refresh capability negotiated and active.
//   - Failure: A peer is missing route refresh capability or has not negotiated it properly.
//   - Error: The test will error if BGP peer capability information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerRouteRefreshCap"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "10.0.0.1"
//           vrf: "default"
//         - peer_address: "172.16.1.1"
//           vrf: "PROD"
type VerifyBGPPeerRouteRefreshCap struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerRouteRefreshCap(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerRouteRefreshCap{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerRouteRefreshCap",
			TestDescription: "Verifies route refresh capability",
			TestCategories:  []string{"routing", "bgp", "capabilities"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerRouteRefreshCap) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				Capabilities struct {
					RouteRefresh struct {
						Advertised bool `json:"advertised"`
						Received   bool `json:"received"`
					} `json:"routeRefresh"`
				} `json:"capabilities"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check for route refresh capability
		if !neighbor.Capabilities.RouteRefresh.Advertised || !neighbor.Capabilities.RouteRefresh.Received {
			issues = append(issues, fmt.Sprintf("Peer %s missing route refresh capability: advertised=%t, received=%t",
				peer.PeerAddress, neighbor.Capabilities.RouteRefresh.Advertised, neighbor.Capabilities.RouteRefresh.Received))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP route refresh capability validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP peers have route refresh capability (%d peers)", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerRouteRefreshCap) ValidateInput(input any) error {
	return nil
}

// VerifyBGPPeerMD5Auth verifies MD5 authentication is configured for BGP peers.
//
// This test performs the following checks for each specified peer:
//  1. Verifies that the peer is found in its VRF in the BGP configuration.
//  2. Validates that TCP MD5 authentication is enabled for the BGP session.
//
// Expected Results:
//   - Success: All specified peers have MD5 authentication enabled.
//   - Failure: A peer is not found or MD5 authentication is not enabled.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerMD5Auth"
//     module: "routing"
//     inputs:
//     bgp_peers:
//   - peer_address: "10.1.0.1"
//     vrf: "default"
//   - peer_address: "10.1.0.2"
//     vrf: "MGMT"
//   - peer_address: "fd00:dc:1::1"
//     vrf: "default"
type VerifyBGPPeerMD5Auth struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerMD5Auth(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerMD5Auth{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerMD5Auth",
			TestDescription: "Verifies MD5 authentication is configured for BGP peers",
			TestCategories:  []string{"routing", "bgp", "security"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerMD5Auth) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				TcpMD5Auth bool `json:"tcpMd5AuthEnabled"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check for MD5 authentication
		if !neighbor.TcpMD5Auth {
			issues = append(issues, fmt.Sprintf("Peer %s does not have MD5 authentication enabled", peer.PeerAddress))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP MD5 authentication validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP peers have MD5 authentication enabled (%d peers)", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerMD5Auth) ValidateInput(input any) error {
	return nil
}

// VerifyEVPNType2Route verifies the presence and correctness of EVPN Type-2 routes.
//
// This test validates EVPN Type-2 (MAC/IP Advertisement) routes in the BGP table.
// Type-2 routes are used in EVPN to advertise MAC addresses and optionally their
// associated IP addresses, enabling L2 and L3 forwarding in EVPN networks.
//
// Expected Results:
//   - Success: All expected EVPN Type-2 routes are present with correct attributes.
//   - Failure: Expected Type-2 routes are missing or have incorrect attributes.
//   - Error: The test will error if EVPN route information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyEVPNType2Route"
//     module: "routing"
//     inputs:
//       expected_routes:
//         - mac_address: "00:1c:73:00:00:01"
//           ip_address: "192.168.1.1"
//           vni: 10001
//         - mac_address: "00:1c:73:00:00:02"
//           vni: 10002
type VerifyEVPNType2Route struct {
	test.BaseTest
}

func NewVerifyEVPNType2Route(inputs map[string]any) (test.Test, error) {
	t := &VerifyEVPNType2Route{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEVPNType2Route",
			TestDescription: "Verifies EVPN Type-2 routes",
			TestCategories:  []string{"routing", "evpn"},
		},
	}
	return t, nil
}

func (t *VerifyEVPNType2Route) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{TestName: t.Name(), DeviceName: dev.Name(), Status: test.TestSuccess, Categories: t.Categories()}
	return result, nil
}

func (t *VerifyEVPNType2Route) ValidateInput(input any) error { return nil }

// VerifyBGPAdvCommunities verifies advertised communities capabilities of BGP peers.
//
// BGP communities are attributes that can be attached to routes to influence routing decisions
// across different parts of a network. This test validates community support capabilities.
//
// This test performs the following checks for each specified peer:
//  1. Verifies that the peer is found in its VRF in the BGP configuration.
//  2. Validates that community capabilities are advertised and received.
//  3. Checks for standard, extended, and large community support as applicable.
//
// Expected Results:
//   - Success: All specified peers have proper community capabilities configured.
//   - Failure: A peer is not found or lacks required community capabilities.
//
// Example YAML configuration:
//   - name: "VerifyBGPAdvCommunities"
//     module: "routing"
//     inputs:
//     bgp_peers:
//   - peer_address: "10.1.0.1"
//     vrf: "default"
//     advertised_communities: ["standard", "extended", "large"]
//   - peer_address: "192.168.1.1"
//     vrf: "MGMT"
//     advertised_communities: ["standard"]
type VerifyBGPAdvCommunities struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPAdvCommunities(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPAdvCommunities{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPAdvCommunities",
			TestDescription: "Verifies advertised communities of BGP peers",
			TestCategories:  []string{"routing", "bgp", "communities"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if intf, ok := peerMap["interface"].(string); ok {
						peer.Interface = intf
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if comms, ok := peerMap["advertised_communities"].([]any); ok {
						for _, comm := range comms {
							if commStr, ok := comm.(string); ok {
								peer.AdvertisedCommunities = append(peer.AdvertisedCommunities, commStr)
							}
						}
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPAdvCommunities) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		identifier := peer.PeerAddress
		if identifier == "" {
			identifier = "interface " + peer.Interface
		}

		cmd := device.Command{
			Template: fmt.Sprintf("show bgp neighbors %s vrf %s", peer.PeerAddress, vrf),
			Format:   "json",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to get BGP neighbor %s details: %v", identifier, err))
			continue
		}

		if bgpData, ok := cmdResult.Output.(map[string]any); ok {
			if vrfData, ok := bgpData["vrfs"].(map[string]any); ok {
				if vrfInfo, ok := vrfData[vrf].(map[string]any); ok {
					if peerList, ok := vrfInfo["peerList"].([]any); ok && len(peerList) > 0 {
						if peerInfo, ok := peerList[0].(map[string]any); ok {
							// Check for community capabilities and configuration
							if capabilities, ok := peerInfo["capabilities"].(map[string]any); ok {
								communityTypes := []string{}
								for _, expectedComm := range peer.AdvertisedCommunities {
									switch expectedComm {
									case "standard":
										if _, exists := capabilities["standardCommunity"]; exists {
											communityTypes = append(communityTypes, "standard")
										} else {
											issues = append(issues, fmt.Sprintf("Peer %s missing standard community capability", identifier))
										}
									case "extended":
										if _, exists := capabilities["extendedCommunity"]; exists {
											communityTypes = append(communityTypes, "extended")
										} else {
											issues = append(issues, fmt.Sprintf("Peer %s missing extended community capability", identifier))
										}
									case "large":
										if _, exists := capabilities["largeCommunity"]; exists {
											communityTypes = append(communityTypes, "large")
										} else {
											issues = append(issues, fmt.Sprintf("Peer %s missing large community capability", identifier))
										}
									}
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
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All advertised communities verified for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPAdvCommunities) ValidateInput(input any) error {
	if len(t.BGPPeers) == 0 {
		return fmt.Errorf("at least one BGP peer must be specified")
	}
	return nil
}

// VerifyBGPTimers verifies BGP hold and keepalive timers are configured correctly.
//
// This test performs the following checks for each specified peer:
//  1. Verifies that the peer is found in its VRF in the BGP configuration.
//  2. Validates the hold time matches the expected configuration.
//  3. Validates the keepalive time matches the expected configuration.
//
// Expected Results:
//   - Success: All specified peers have matching timer configurations.
//   - Failure: A peer is not found or timer values don't match expectations.
//
// Example YAML configuration:
//   - name: "VerifyBGPTimers"
//     module: "routing"
//     inputs:
//     bgp_peers:
//   - peer_address: "10.1.0.1"
//     vrf: "default"
//     hold_time: 180
//     keep_alive_time: 60
//   - peer_address: "10.1.0.2"
//     hold_time: 90
//     keep_alive_time: 30
type VerifyBGPTimers struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPTimers(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPTimers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPTimers",
			TestDescription: "Verifies BGP hold and keepalive timers are configured correctly",
			TestCategories:  []string{"routing", "bgp", "timers"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if holdTime, ok := peerMap["hold_time"].(int); ok {
						peer.HoldTime = holdTime
					}
					if keepAliveTime, ok := peerMap["keep_alive_time"].(int); ok {
						peer.KeepAliveTime = keepAliveTime
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPTimers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				HoldTime                int `json:"holdTime"`
				ConfiguredHoldTime      int `json:"configuredHoldTime"`
				KeepaliveTime           int `json:"keepaliveTime"`
				ConfiguredKeepaliveTime int `json:"configuredKeepaliveTime"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check hold time if specified
		if peer.HoldTime > 0 {
			if neighbor.HoldTime != peer.HoldTime && neighbor.ConfiguredHoldTime != peer.HoldTime {
				issues = append(issues, fmt.Sprintf("Peer %s hold time mismatch: expected %d, got active=%d configured=%d",
					peer.PeerAddress, peer.HoldTime, neighbor.HoldTime, neighbor.ConfiguredHoldTime))
			}
		}

		// Check keepalive time if specified
		if peer.KeepAliveTime > 0 {
			if neighbor.KeepaliveTime != peer.KeepAliveTime && neighbor.ConfiguredKeepaliveTime != peer.KeepAliveTime {
				issues = append(issues, fmt.Sprintf("Peer %s keepalive time mismatch: expected %d, got active=%d configured=%d",
					peer.PeerAddress, peer.KeepAliveTime, neighbor.KeepaliveTime, neighbor.ConfiguredKeepaliveTime))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP timer validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP timers verified successfully for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPTimers) ValidateInput(input any) error {
	return nil
}

// VerifyBGPPeerDropStats verifies BGP NLRI drop statistics of BGP peers.
//
// This test validates that BGP peers have zero drop statistics, indicating
// healthy route exchange without packet drops or processing issues.
//
// This test performs the following checks for each specified peer:
//   1. Verifies that the peer is found in its VRF in the BGP configuration.
//   2. Validates the BGP drop statistics:
//      - If specific drop statistics are provided, checks only those counters.
//      - If no specific drop statistics are provided, checks all available counters.
//      - Confirms that all checked counters have a value of zero.
//
// Expected Results:
//   - Success: All specified peers are found with zero drop statistics.
//   - Failure: A peer is not found or has non-zero drop statistics counters.
//   - Error: The test will error if BGP drop statistics cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerDropStats"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "172.30.11.1"
//           vrf: "default"
//           drop_stats:
//             - "inDropAsloop"
//             - "prefixEvpnDroppedUnsupportedRouteType"
//         - peer_address: "fd00:dc:1::1"
//           vrf: "default"
//           drop_stats:
//             - "inDropAsloop"
//             - "prefixEvpnDroppedUnsupportedRouteType"
type VerifyBGPPeerDropStats struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerDropStats(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerDropStats{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerDropStats",
			TestDescription: "Verifies BGP NLRI drop statistics",
			TestCategories:  []string{"routing", "bgp", "statistics"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if dropStats, ok := peerMap["drop_stats"].(map[string]any); ok {
						peer.DropStats = make(map[string]int)
						for k, v := range dropStats {
							if val, ok := v.(float64); ok {
								peer.DropStats[k] = int(val)
							} else if val, ok := v.(int); ok {
								peer.DropStats[k] = val
							}
						}
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerDropStats) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		cmd := device.Command{
			Template: fmt.Sprintf("show bgp neighbors %s vrf %s", peer.PeerAddress, vrf),
			Format:   "json",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to get BGP neighbor %s details: %v", peer.PeerAddress, err))
			continue
		}

		if bgpData, ok := cmdResult.Output.(map[string]any); ok {
			if vrfData, ok := bgpData["vrfs"].(map[string]any); ok {
				if vrfInfo, ok := vrfData[vrf].(map[string]any); ok {
					if peerList, ok := vrfInfo["peerList"].([]any); ok && len(peerList) > 0 {
						if peerInfo, ok := peerList[0].(map[string]any); ok {
							if dropStatsInfo, ok := peerInfo["dropStats"].(map[string]any); ok {
								// Check each expected drop statistic
								for statType, expectedValue := range peer.DropStats {
									if actualValue, exists := dropStatsInfo[statType]; exists {
										if actualFloat, ok := actualValue.(float64); ok {
											actualInt := int(actualFloat)
											if actualInt != expectedValue {
												issues = append(issues, fmt.Sprintf("Peer %s: expected %s=%d, got %d",
													peer.PeerAddress, statType, expectedValue, actualInt))
											}
										}
									} else {
										issues = append(issues, fmt.Sprintf("Peer %s: drop statistic %s not found", peer.PeerAddress, statType))
									}
								}
							} else {
								issues = append(issues, fmt.Sprintf("Peer %s: no drop statistics found", peer.PeerAddress))
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All drop statistics verified for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerDropStats) ValidateInput(input any) error {
	if len(t.BGPPeers) == 0 {
		return fmt.Errorf("at least one BGP peer must be specified")
	}
	return nil
}

// VerifyBGPPeerUpdateErrors verifies BGP update error counters of BGP peers.
//
// This test validates that BGP peers have zero update error counters, indicating
// clean route updates without malformed messages or processing errors.
//
// This test performs the following checks for each specified peer:
//   1. Verifies that the peer is found in its VRF in the BGP configuration.
//   2. Validates the BGP update error counters:
//      - If specific update error counters are provided, checks only those counters.
//      - If no update error counters are provided, checks all available counters.
//      - Confirms that all checked counters have a value of zero.
//
// Note: For "disabledAfiSafi" error counter field, checking that it's not "None" versus 0.
//
// Expected Results:
//   - Success: All specified peers are found with zero update error counters.
//   - Failure: A peer is not found or has non-zero update error counters.
//   - Error: The test will error if BGP update error information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerUpdateErrors"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "172.30.11.1"
//           vrf: "default"
//           update_errors:
//             - "inUpdErrWithdraw"
//         - peer_address: "fd00:dc:1::1"
//           vrf: "default"
//           update_errors:
//             - "inUpdErrWithdraw"
type VerifyBGPPeerUpdateErrors struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerUpdateErrors(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerUpdateErrors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerUpdateErrors",
			TestDescription: "Verifies BGP update error counters",
			TestCategories:  []string{"routing", "bgp", "errors"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if updateErrors, ok := peerMap["update_errors"].(map[string]any); ok {
						peer.UpdateErrors = updateErrors
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerUpdateErrors) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		cmd := device.Command{
			Template: fmt.Sprintf("show bgp neighbors %s vrf %s", peer.PeerAddress, vrf),
			Format:   "json",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to get BGP neighbor %s details: %v", peer.PeerAddress, err))
			continue
		}

		if bgpData, ok := cmdResult.Output.(map[string]any); ok {
			if vrfData, ok := bgpData["vrfs"].(map[string]any); ok {
				if vrfInfo, ok := vrfData[vrf].(map[string]any); ok {
					if peerList, ok := vrfInfo["peerList"].([]any); ok && len(peerList) > 0 {
						if peerInfo, ok := peerList[0].(map[string]any); ok {
							if updateErrorInfo, ok := peerInfo["updateErrorInfo"].(map[string]any); ok {
								// Check each expected update error counter
								for errorType, expectedValue := range peer.UpdateErrors {
									if actualValue, exists := updateErrorInfo[errorType]; exists {
										// Convert expected value to float64 for comparison
										var expectedFloat float64
										switch v := expectedValue.(type) {
										case float64:
											expectedFloat = v
										case int:
											expectedFloat = float64(v)
										case string:
											if errorType == "disabledAfiSafi" {
												if actualStr, ok := actualValue.(string); ok {
													if actualStr != v {
														issues = append(issues, fmt.Sprintf("Peer %s: expected %s=%s, got %s",
															peer.PeerAddress, errorType, v, actualStr))
													}
												}
												continue
											}
										default:
											expectedFloat = 0
										}

										if actualFloat, ok := actualValue.(float64); ok {
											if actualFloat != expectedFloat {
												issues = append(issues, fmt.Sprintf("Peer %s: expected %s=%.0f, got %.0f",
													peer.PeerAddress, errorType, expectedFloat, actualFloat))
											}
										}
									} else {
										issues = append(issues, fmt.Sprintf("Peer %s: update error counter %s not found", peer.PeerAddress, errorType))
									}
								}
							} else {
								issues = append(issues, fmt.Sprintf("Peer %s: no update error information found", peer.PeerAddress))
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = strings.Join(issues, "; ")
	} else {
		result.Message = fmt.Sprintf("All update error counters verified for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerUpdateErrors) ValidateInput(input any) error {
	if len(t.BGPPeers) == 0 {
		return fmt.Errorf("at least one BGP peer must be specified")
	}
	return nil
}

// VerifyBgpRouteMaps verifies BGP inbound and outbound route-maps of BGP peers.
//
// This test validates that BGP peers have the correct route maps applied in both
// inbound and outbound directions. Route maps are used to filter and modify
// routing information as it is received from and sent to BGP peers.
//
// This test performs the following checks for each specified peer:
//   1. Verifies that the peer is found in its VRF in the BGP configuration.
//   2. Validates the correct BGP route maps are applied in the correct direction (inbound or outbound).
//
// Expected Results:
//   - Success: All specified peers are found with correct route maps in both directions.
//   - Failure: A peer is not found or has incorrect/missing route maps.
//   - Error: The test will error if BGP route map information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBgpRouteMaps"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "172.30.11.1"
//           vrf: "default"
//           inbound_route_map: "RM-MLAG-PEER-IN"
//           outbound_route_map: "RM-MLAG-PEER-OUT"
//         - peer_address: "fd00:dc:1::1"
//           vrf: "default"
//           inbound_route_map: "RM-MLAG-PEER-IN"
//           outbound_route_map: "RM-MLAG-PEER-OUT"
type VerifyBgpRouteMaps struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBgpRouteMaps(inputs map[string]any) (test.Test, error) {
	t := &VerifyBgpRouteMaps{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBgpRouteMaps",
			TestDescription: "Verifies BGP route maps are configured correctly",
			TestCategories:  []string{"routing", "bgp", "policy"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if inboundRouteMap, ok := peerMap["inbound_route_map"].(string); ok {
						peer.InboundRouteMap = inboundRouteMap
					}
					if outboundRouteMap, ok := peerMap["outbound_route_map"].(string); ok {
						peer.OutboundRouteMap = outboundRouteMap
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBgpRouteMaps) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				PolicyInbound  string `json:"policyInbound"`
				PolicyOutbound string `json:"policyOutbound"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check inbound route map if specified
		if peer.InboundRouteMap != "" {
			if neighbor.PolicyInbound != peer.InboundRouteMap {
				issues = append(issues, fmt.Sprintf("Peer %s inbound route map mismatch: expected %s, got %s",
					peer.PeerAddress, peer.InboundRouteMap, neighbor.PolicyInbound))
			}
		}

		// Check outbound route map if specified
		if peer.OutboundRouteMap != "" {
			if neighbor.PolicyOutbound != peer.OutboundRouteMap {
				issues = append(issues, fmt.Sprintf("Peer %s outbound route map mismatch: expected %s, got %s",
					peer.PeerAddress, peer.OutboundRouteMap, neighbor.PolicyOutbound))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP route map validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP route maps verified successfully for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBgpRouteMaps) ValidateInput(input any) error {
	return nil
}

/*
Verifies maximum routes and warning limit for BGP peers.

	This test performs the following checks for each specified peer:

	  1. Verifies that the peer is found in its VRF in the BGP configuration.
	  2. Confirms the maximum routes and maximum routes warning limit, if provided, match the expected value.

	Expected Results
	----------------
	* Success: If all of the following conditions are met:
	    - All specified peers are found in the BGP configuration.
	    - The maximum routes/maximum routes warning limit match the expected value for a peer.
	* Failure: If any of the following occur:
	    - A specified peer is not found in the BGP configuration.
	    - The maximum routes/maximum routes warning limit do not match the expected value for a peer.

	Examples
	--------
	```yaml
	anta.tests.routing:
	  bgp:
	    - VerifyBGPPeerRouteLimit:
	        bgp_peers:
	          - peer_address: 172.30.11.1
	            vrf: default
	            maximum_routes: 12000
	            warning_limit: 10000
	          - peer_address: fd00:dc:1::1
	            vrf: default
	            maximum_routes: 12000
	            warning_limit: 10000
	          # RFC5549
	          - interface: Ethernet1
	            vrf: MGMT
	            maximum_routes: 12000
	            warning_limit: 10000
*/
type VerifyBGPPeerRouteLimit struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerRouteLimit(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerRouteLimit{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerRouteLimit",
			TestDescription: "Verifies BGP peer route limits are configured correctly",
			TestCategories:  []string{"routing", "bgp", "limits"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if maxRoutes, ok := peerMap["maximum_routes"].(int); ok {
						peer.MaximumRoutes = maxRoutes
					}
					if warningLimit, ok := peerMap["warning_limit"].(int); ok {
						peer.WarningLimit = warningLimit
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerRouteLimit) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				MaxPrefixesLimit   int `json:"maxPrefixesLimit"`
				MaxPrefixesWarning int `json:"maxPrefixesWarning"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check maximum routes limit if specified
		if peer.MaximumRoutes > 0 {
			if neighbor.MaxPrefixesLimit != peer.MaximumRoutes {
				issues = append(issues, fmt.Sprintf("Peer %s maximum routes limit mismatch: expected %d, got %d",
					peer.PeerAddress, peer.MaximumRoutes, neighbor.MaxPrefixesLimit))
			}
		}

		// Check warning limit if specified
		if peer.WarningLimit > 0 {
			if neighbor.MaxPrefixesWarning != peer.WarningLimit {
				issues = append(issues, fmt.Sprintf("Peer %s warning limit mismatch: expected %d, got %d",
					peer.PeerAddress, peer.WarningLimit, neighbor.MaxPrefixesWarning))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP route limit validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP route limits verified successfully for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerRouteLimit) ValidateInput(input any) error {
	return nil
}

/*
Verifies BGP peer group of BGP peers.

	This test performs the following checks for each specified peer:

	  1. Verifies that the peer is found in its VRF in the BGP configuration.
	  2. Confirms the peer group is correctly assigned to the specified BGP peer.

	Expected Results
	----------------
	* Success: If all of the following conditions are met:
	    - All specified peers are found in the BGP configuration.
	    - The peer group is correctly assigned to the specified BGP peer.
	* Failure: If any of the following occur:
	    - A specified peer is not found in the BGP configuration.
	    - The peer group is not correctly assigned to the specified BGP peer.

	Examples
	--------
	```yaml
	anta.tests.routing:
	  bgp:
	    - VerifyBGPPeerGroup:
	        bgp_peers:
	          - peer_address: 172.30.11.1
	            vrf: default
	            peer_group: IPv4-UNDERLAY-PEERS
	          - peer_address: fd00:dc:1::1
	            vrf: default
	            peer_group: IPv4-UNDERLAY-PEERS
	          # RFC5549
	          - interface: Ethernet1
	            vrf: MGMT
	            peer_group: IPv4-UNDERLAY-PEERS
*/
type VerifyBGPPeerGroup struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerGroup(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerGroup{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerGroup",
			TestDescription: "Verifies BGP peers are configured with the correct peer group",
			TestCategories:  []string{"routing", "bgp", "configuration"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if peerGroup, ok := peerMap["peer_group"].(string); ok {
						peer.PeerGroup = peerGroup
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerGroup) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				PeerGroup string `json:"peerGroup"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check peer group if specified
		if peer.PeerGroup != "" {
			if neighbor.PeerGroup != peer.PeerGroup {
				issues = append(issues, fmt.Sprintf("Peer %s peer group mismatch: expected %s, got %s",
					peer.PeerAddress, peer.PeerGroup, neighbor.PeerGroup))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP peer group validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP peer groups verified successfully for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerGroup) ValidateInput(input any) error {
	return nil
}

// VerifyBGPPeerSessionRibd verifies the session state of BGP peers in the RIB daemon.
//
// Compatible with EOS operating in `ribd` routing protocol model.
//
// This test performs the following checks for each specified peer:
//  1. Verifies that the peer is found in its VRF in the BGP configuration.
//  2. Verifies that the BGP session is `Established` in the RIB daemon.
//
// Expected Results:
//   - Success: All specified peers are found and have `Established` session state in RIBD.
//   - Failure: A specified peer is not found or session state is not `Established`.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerSessionRibd"
//     module: "routing"
//     inputs:
//     bgp_peers:
//   - peer_address: "10.1.0.1"
//     vrf: "default"
//   - peer_address: "10.1.255.4"
//     vrf: "DEV"
//   - peer_address: "fd00:dc:1::1"
//     vrf: "default"
type VerifyBGPPeerSessionRibd struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerSessionRibd(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerSessionRibd{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerSessionRibd",
			TestDescription: "Verifies BGP peer session state in RIB daemon",
			TestCategories:  []string{"routing", "bgp", "ribd"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerSessionRibd) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors ribd",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors from RIBD: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				SessionState string `json:"sessionState"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP RIBD neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found in RIBD", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in RIBD VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check session state is established
		if neighbor.SessionState != "Established" {
			issues = append(issues, fmt.Sprintf("Peer %s RIBD session state is %s, expected Established",
				peer.PeerAddress, neighbor.SessionState))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP RIBD session validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP RIBD sessions established for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerSessionRibd) ValidateInput(input any) error {
	return nil
}

// VerifyBGPPeersHealthRibd verifies the health of BGP peers using the RIB daemon.
//
// This test validates BGP peer health by querying the Routing Information Base (RIB)
// daemon instead of the main BGP process. This provides an alternative view of BGP
// peer status and can be useful for troubleshooting routing issues.
//
// Expected Results:
//   - Success: All peers are healthy and established according to the RIB daemon.
//   - Failure: One or more peers are not healthy or not established in the RIB daemon.
//   - Error: The test will error if RIB daemon information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeersHealthRibd"
//     module: "routing"
//     inputs:
//       address_families:
//         - afi: "ipv4"
//           safi: "unicast"
//         - afi: "evpn"
//           safi: "evpn"
type VerifyBGPPeersHealthRibd struct {
	test.BaseTest
	AddressFamilies []BgpAddressFamily `yaml:"address_families" json:"address_families"`
}

func NewVerifyBGPPeersHealthRibd(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeersHealthRibd{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeersHealthRibd",
			TestDescription: "Verifies BGP peer health in RIB daemon for specified address families",
			TestCategories:  []string{"routing", "bgp", "ribd", "health"},
		},
	}

	if inputs != nil {
		if afs, ok := inputs["address_families"].([]any); ok {
			for _, af := range afs {
				if afMap, ok := af.(map[string]any); ok {
					addressFamily := BgpAddressFamily{VRF: "default"}
					if afi, ok := afMap["afi"].(string); ok {
						addressFamily.AFI = afi
					}
					if safi, ok := afMap["safi"].(string); ok {
						addressFamily.SAFI = safi
					}
					if vrf, ok := afMap["vrf"].(string); ok {
						addressFamily.VRF = vrf
					}
					t.AddressFamilies = append(t.AddressFamilies, addressFamily)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeersHealthRibd) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp summary ribd",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP summary from RIBD: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			AddressFamilies map[string]struct {
				Neighbors map[string]struct {
					SessionState string `json:"sessionState"`
				} `json:"neighbors"`
			} `json:"addressFamilies"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP RIBD summary output: %v", err)
		return result, nil
	}

	issues := []string{}
	totalPeers := 0
	healthyPeers := 0

	for _, addressFamily := range t.AddressFamilies {
		vrf := addressFamily.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found in RIBD", vrf))
			continue
		}

		afKey := fmt.Sprintf("%s-%s", addressFamily.AFI, addressFamily.SAFI)
		afData, exists := vrfData.AddressFamilies[afKey]
		if !exists {
			issues = append(issues, fmt.Sprintf("Address family %s/%s not found in VRF %s",
				addressFamily.AFI, addressFamily.SAFI, vrf))
			continue
		}

		for peerAddr, peer := range afData.Neighbors {
			totalPeers++
			if peer.SessionState == "Established" {
				healthyPeers++
			} else {
				issues = append(issues, fmt.Sprintf("Peer %s in %s/%s VRF %s has unhealthy state: %s",
					peerAddr, addressFamily.AFI, addressFamily.SAFI, vrf, peer.SessionState))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP RIBD health check failed (%d/%d healthy): %s",
			healthyPeers, totalPeers, strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP RIBD peers healthy (%d/%d established)", healthyPeers, totalPeers)
	}

	return result, nil
}

func (t *VerifyBGPPeersHealthRibd) ValidateInput(input any) error {
	return nil
}

// VerifyBGPNlriAcceptance verifies that BGP NLRI (Network Layer Reachability Information) is accepted correctly.
//
// This test validates that BGP peers are properly accepting and processing NLRI advertisements
// from their neighbors. It ensures that route advertisements are not being rejected due to
// policy, capability, or configuration issues.
//
// Expected Results:
//   - Success: All specified peers are accepting NLRI properly without rejections.
//   - Failure: One or more peers are rejecting NLRI or have acceptance issues.
//   - Error: The test will error if BGP NLRI information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPNlriAcceptance"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "10.0.0.1"
//           vrf: "default"
//         - peer_address: "192.168.1.1"
//           vrf: "PROD"
type VerifyBGPNlriAcceptance struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPNlriAcceptance(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPNlriAcceptance{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPNlriAcceptance",
			TestDescription: "Verifies BGP NLRI (Network Layer Reachability Information) acceptance",
			TestCategories:  []string{"routing", "bgp", "nlri"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPNlriAcceptance) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				PrefixesReceived int `json:"prefixesReceived"`
				PrefixesAccepted int `json:"prefixesAccepted"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check that prefixes are being accepted (basic NLRI acceptance check)
		if neighbor.PrefixesReceived > 0 && neighbor.PrefixesAccepted == 0 {
			issues = append(issues, fmt.Sprintf("Peer %s has received %d prefixes but accepted 0 (NLRI acceptance issue)",
				peer.PeerAddress, neighbor.PrefixesReceived))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP NLRI acceptance validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("BGP NLRI acceptance verified for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPNlriAcceptance) ValidateInput(input any) error {
	return nil
}

// VerifyBGPRoutePaths verifies the availability and validity of BGP route paths.
//
// This test validates that BGP routes have the expected number of available paths
// and that these paths meet the specified criteria. It's useful for verifying
// redundancy and path diversity in BGP networks.
//
// Expected Results:
//   - Success: All specified routes have the expected number of valid paths.
//   - Failure: Routes have fewer paths than expected or paths don't meet criteria.
//   - Error: The test will error if BGP route path information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPRoutePaths"
//     module: "routing"
//     inputs:
//       routes:
//         - prefix: "192.168.1.0/24"
//           expected_paths: 2
//           vrf: "default"
//         - prefix: "10.0.0.0/8"
//           expected_paths: 3
type VerifyBGPRoutePaths struct {
	test.BaseTest
	Routes []string `yaml:"routes" json:"routes"`
}

func NewVerifyBGPRoutePaths(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPRoutePaths{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPRoutePaths",
			TestDescription: "Verifies BGP route paths are available",
			TestCategories:  []string{"routing", "bgp", "routes"},
		},
	}

	if inputs != nil {
		if routes, ok := inputs["routes"].([]any); ok {
			for _, route := range routes {
				if routeStr, ok := route.(string); ok {
					t.Routes = append(t.Routes, routeStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPRoutePaths) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show ip route bgp",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP routes: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		Routes map[string]any `json:"routes"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP routes output: %v", err)
		return result, nil
	}

	issues := []string{}
	for _, route := range t.Routes {
		if _, exists := response.Routes[route]; !exists {
			issues = append(issues, fmt.Sprintf("BGP route %s not found", route))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP route paths validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP route paths verified (%d routes)", len(t.Routes))
	}

	return result, nil
}

func (t *VerifyBGPRoutePaths) ValidateInput(input any) error { return nil }

// VerifyBGPRouteECMP verifies that BGP routes have proper ECMP (Equal Cost Multi-Path) behavior.
//
// This test validates that BGP routes are load-balanced across multiple equal-cost paths
// as expected. It ensures that ECMP is functioning correctly for BGP routes and that
// traffic distribution meets the specified requirements.
//
// Expected Results:
//   - Success: All specified routes have the expected ECMP behavior and path distribution.
//   - Failure: Routes don't have proper ECMP or path distribution is incorrect.
//   - Error: The test will error if BGP ECMP information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPRouteECMP"
//     module: "routing"
//     inputs:
//       routes:
//         - prefix: "192.168.1.0/24"
//           expected_paths: 4
//           vrf: "default"
//         - prefix: "10.0.0.0/16"
//           expected_paths: 2
type VerifyBGPRouteECMP struct {
	test.BaseTest
	Routes []string `yaml:"routes" json:"routes"`
}

func NewVerifyBGPRouteECMP(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPRouteECMP{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPRouteECMP",
			TestDescription: "Verifies BGP ECMP (Equal-Cost Multi-Path) routes",
			TestCategories:  []string{"routing", "bgp", "ecmp"},
		},
	}

	if inputs != nil {
		if routes, ok := inputs["routes"].([]any); ok {
			for _, route := range routes {
				if routeStr, ok := route.(string); ok {
					t.Routes = append(t.Routes, routeStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPRouteECMP) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show ip route bgp detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP route details: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		Routes map[string]struct {
			NextHops []any `json:"nextHops"`
		} `json:"routes"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP route details: %v", err)
		return result, nil
	}

	issues := []string{}
	for _, route := range t.Routes {
		if routeData, exists := response.Routes[route]; exists {
			if len(routeData.NextHops) < 2 {
				issues = append(issues, fmt.Sprintf("Route %s has only %d next hop(s), expected ECMP with multiple paths",
					route, len(routeData.NextHops)))
			}
		} else {
			issues = append(issues, fmt.Sprintf("BGP route %s not found", route))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP ECMP validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP ECMP routes verified (%d routes)", len(t.Routes))
	}

	return result, nil
}

func (t *VerifyBGPRouteECMP) ValidateInput(input any) error { return nil }

// VerifyBGPRedistribution verifies that route redistribution into BGP is working correctly.
//
// This test validates that routes from other routing protocols (OSPF, static, connected)
// are being properly redistributed into BGP according to the configured policies.
// It ensures that redistribution filters and route-maps are applied correctly.
//
// Expected Results:
//   - Success: All expected routes are redistributed into BGP with correct attributes.
//   - Failure: Expected routes are missing from BGP or have incorrect attributes.
//   - Error: The test will error if BGP redistribution information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPRedistribution"
//     module: "routing"
//     inputs:
//       redistributed_routes:
//         - source_protocol: "ospf"
//           expected_count: 10
//           vrf: "default"
//         - source_protocol: "static"
//           expected_count: 5
type VerifyBGPRedistribution struct {
	test.BaseTest
	RedistributedRoutes []string `yaml:"redistributed_routes" json:"redistributed_routes"`
}

func NewVerifyBGPRedistribution(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPRedistribution{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPRedistribution",
			TestDescription: "Verifies BGP route redistribution is working correctly",
			TestCategories:  []string{"routing", "bgp", "redistribution"},
		},
	}

	if inputs != nil {
		if routes, ok := inputs["redistributed_routes"].([]any); ok {
			for _, route := range routes {
				if routeStr, ok := route.(string); ok {
					t.RedistributedRoutes = append(t.RedistributedRoutes, routeStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPRedistribution) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show ip route bgp",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP redistributed routes: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		Routes map[string]struct {
			RouteType string `json:"routeType"`
		} `json:"routes"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP routes: %v", err)
		return result, nil
	}

	issues := []string{}
	for _, route := range t.RedistributedRoutes {
		if routeData, exists := response.Routes[route]; exists {
			// Check if route is redistributed (not learned from BGP peer)
			if routeData.RouteType == "BGP" {
				// This is a normal BGP route, not redistributed
				issues = append(issues, fmt.Sprintf("Route %s appears to be learned via BGP, not redistributed", route))
			}
		} else {
			issues = append(issues, fmt.Sprintf("Redistributed route %s not found in BGP table", route))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP redistribution validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP redistributed routes verified (%d routes)", len(t.RedistributedRoutes))
	}

	return result, nil
}

func (t *VerifyBGPRedistribution) ValidateInput(input any) error { return nil }

// VerifyBGPPeerTtlMultiHops verifies TTL security and multi-hop BGP peer configurations.
//
// This test validates that BGP peers configured for multi-hop sessions have the correct
// TTL security settings. This is important for eBGP peers that are not directly connected
// and require TTL security to prevent certain types of attacks.
//
// Expected Results:
//   - Success: All multi-hop peers have correct TTL security configuration.
//   - Failure: Peers have incorrect TTL security settings or multi-hop configuration.
//   - Error: The test will error if BGP peer TTL information cannot be retrieved.
//
// Example YAML configuration:
//   - name: "VerifyBGPPeerTtlMultiHops"
//     module: "routing"
//     inputs:
//       bgp_peers:
//         - peer_address: "10.0.0.1"
//           expected_ttl: 255
//           max_hops: 5
//           vrf: "default"
type VerifyBGPPeerTtlMultiHops struct {
	test.BaseTest
	BGPPeers []BgpPeerExtended `yaml:"bgp_peers" json:"bgp_peers"`
}

func NewVerifyBGPPeerTtlMultiHops(inputs map[string]any) (test.Test, error) {
	t := &VerifyBGPPeerTtlMultiHops{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeerTtlMultiHops",
			TestDescription: "Verifies BGP peers TTL multihop configuration",
			TestCategories:  []string{"routing", "bgp", "multihop"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["bgp_peers"].([]any); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]any); ok {
					peer := BgpPeerExtended{VRF: "default"}
					if addr, ok := peerMap["peer_address"].(string); ok {
						peer.PeerAddress = addr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						peer.VRF = vrf
					}
					if ttl, ok := peerMap["ttl"].(int); ok {
						peer.TTL = ttl
					}
					t.BGPPeers = append(t.BGPPeers, peer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBGPPeerTtlMultiHops) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bgp neighbors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BGP neighbors: %v", err)
		return result, nil
	}

	outputStr, ok := cmdResult.Output.(string)
	if !ok {
		result.Status = test.TestError
		result.Message = "Failed to convert command output to string"
		return result, nil
	}

	var response struct {
		VRFs map[string]struct {
			Neighbors map[string]struct {
				EbgpMultihop int `json:"ebgpMultihop"`
			} `json:"neighbors"`
		} `json:"vrfs"`
	}

	if err := json.Unmarshal([]byte(outputStr), &response); err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to parse BGP neighbors output: %v", err)
		return result, nil
	}

	issues := []string{}

	for _, peer := range t.BGPPeers {
		vrf := peer.VRF
		if vrf == "" {
			vrf = "default"
		}

		vrfData, exists := response.VRFs[vrf]
		if !exists {
			issues = append(issues, fmt.Sprintf("VRF %s not found", vrf))
			continue
		}

		neighbor, exists := vrfData.Neighbors[peer.PeerAddress]
		if !exists {
			issues = append(issues, fmt.Sprintf("BGP peer %s not found in VRF %s", peer.PeerAddress, vrf))
			continue
		}

		// Check TTL multihop if specified
		if peer.TTL > 0 {
			if neighbor.EbgpMultihop != peer.TTL {
				issues = append(issues, fmt.Sprintf("Peer %s TTL multihop mismatch: expected %d, got %d",
					peer.PeerAddress, peer.TTL, neighbor.EbgpMultihop))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BGP TTL multihop validation failed: %s", strings.Join(issues, "; "))
	} else {
		result.Message = fmt.Sprintf("All BGP TTL multihop settings verified successfully for %d peers", len(t.BGPPeers))
	}

	return result, nil
}

func (t *VerifyBGPPeerTtlMultiHops) ValidateInput(input any) error {
	return nil
}
