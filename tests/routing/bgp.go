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