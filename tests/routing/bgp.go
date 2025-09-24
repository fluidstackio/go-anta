package routing

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

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

func NewVerifyBGPPeers(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyBGPPeers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBGPPeers",
			TestDescription: "Verify BGP peer status and configuration",
			TestCategories:  []string{"routing", "bgp"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["peers"].([]interface{}); ok {
			for _, p := range peers {
				if peerMap, ok := p.(map[string]interface{}); ok {
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
	
	if bgpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]interface{}); ok {
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

				if vrf, ok := vrfData.(map[string]interface{}); ok {
					if peers, ok := vrf["peers"].(map[string]interface{}); ok {
						peerData, peerExists := peers[peer.Peer]
						if !peerExists {
							issues = append(issues, fmt.Sprintf("Peer %s not found in VRF %s", peer.Peer, vrfName))
							continue
						}

						if peerInfo, ok := peerData.(map[string]interface{}); ok {
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

func (t *VerifyBGPPeers) ValidateInput(input interface{}) error {
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

// VerifyBGPUnnumbered validates BGP unnumbered configurations
type VerifyBGPUnnumbered struct {
	test.BaseTest
	Interfaces []BGPUnnumberedInterface `yaml:"interfaces" json:"interfaces"`
	VRF        string                    `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

type BGPUnnumberedInterface struct {
	Interface    string `yaml:"interface" json:"interface"`
	RemoteASN    int    `yaml:"remote_asn,omitempty" json:"remote_asn,omitempty"`
	ExpectedState string `yaml:"expected_state,omitempty" json:"expected_state,omitempty"`
	Description  string `yaml:"description,omitempty" json:"description,omitempty"`
}

func NewVerifyBGPUnnumbered(inputs map[string]interface{}) (test.Test, error) {
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

		if interfaces, ok := inputs["interfaces"].([]interface{}); ok {
			for _, intf := range interfaces {
				if intfMap, ok := intf.(map[string]interface{}); ok {
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
	if bgpData, ok := summaryResult.Output.(map[string]interface{}); ok {
		if vrfs, ok := bgpData["vrfs"].(map[string]interface{}); ok {
			vrfName := t.VRF
			if vrfName == "" {
				vrfName = "default"
			}

			if vrfData, exists := vrfs[vrfName]; exists {
				if vrf, ok := vrfData.(map[string]interface{}); ok {
					if peers, ok := vrf["peers"].(map[string]interface{}); ok {
						
						// Validate each unnumbered interface
						for _, intf := range t.Interfaces {
							interfaceFound := false
							
							// Look for a peer that ends with %<interface>
							interfaceSuffix := "%" + intf.Interface
							
							for peerAddr, peerData := range peers {
								if strings.HasSuffix(peerAddr, interfaceSuffix) {
									interfaceFound = true
									
									if peerInfo, ok := peerData.(map[string]interface{}); ok {
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

func (t *VerifyBGPUnnumbered) ValidateInput(input interface{}) error {
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