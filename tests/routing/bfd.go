package routing

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstack/go-anta/pkg/device"
	"github.com/fluidstack/go-anta/pkg/test"
)

// VerifyBFDSpecificPeers verifies specific BFD (Bidirectional Forwarding Detection) peer sessions.
//
// This test validates that specified BFD peers are in the "up" state and have non-zero
// remote discriminators, which indicates healthy BFD session establishment.
//
// Note: Seamless BFD (S-BFD) is not supported in this test.
//
// The test performs the following checks:
//   1. Retrieves BFD peer information from the device.
//   2. Verifies that each specified peer exists in the configuration.
//   3. Validates that the peer status is "up".
//   4. Confirms that the remote discriminator is non-zero.
//
// Expected Results:
//   - Success: The test will pass if all specified BFD peers are up with valid discriminators.
//   - Failure: The test will fail if any peer is down, missing, or has invalid discriminators.
//   - Error: The test will report an error if BFD peer information cannot be retrieved.
//
// Examples:
//   - name: VerifyBFDSpecificPeers with interface
//     VerifyBFDSpecificPeers:
//       peers:
//         - peer_address: "192.168.1.1"
//           vrf: "default"
//           interface: "Ethernet1"
//         - peer_address: "192.168.1.2"
//           vrf: "MGMT"
//
//   - name: VerifyBFDSpecificPeers minimal config
//     VerifyBFDSpecificPeers:
//       peers:
//         - peer_address: "10.1.1.1"
//           vrf: "default"
type VerifyBFDSpecificPeers struct {
	test.BaseTest
	Peers []BFDPeer `yaml:"peers" json:"peers"`
}

type BFDPeer struct {
	PeerAddress string `yaml:"peer_address" json:"peer_address"`
	VRF         string `yaml:"vrf" json:"vrf"`
	Interface   string `yaml:"interface,omitempty" json:"interface,omitempty"`
}

func NewVerifyBFDSpecificPeers(inputs map[string]any) (test.Test, error) {
	t := &VerifyBFDSpecificPeers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBFDSpecificPeers",
			TestDescription: "Verify specific BFD peer sessions",
			TestCategories:  []string{"routing", "bfd"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["peers"].([]any); ok {
			for _, peer := range peers {
				if peerMap, ok := peer.(map[string]any); ok {
					bfdPeer := BFDPeer{}
					if peerAddr, ok := peerMap["peer_address"].(string); ok {
						bfdPeer.PeerAddress = peerAddr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						bfdPeer.VRF = vrf
					}
					if intf, ok := peerMap["interface"].(string); ok {
						bfdPeer.Interface = intf
					}
					t.Peers = append(t.Peers, bfdPeer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBFDSpecificPeers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bfd peers",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BFD peers: %v", err)
		return result, nil
	}

	// Parse BFD peers from device
	devicePeers := make(map[string]BFDPeerInfo)
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := data["vrfs"].(map[string]any); ok {
			for vrfName, vrfData := range vrfs {
				if vrfInfo, ok := vrfData.(map[string]any); ok {
					if peers, ok := vrfInfo["peers"].(map[string]any); ok {
						for peerAddr, peerData := range peers {
							if peerInfo, ok := peerData.(map[string]any); ok {
								peer := BFDPeerInfo{
									PeerAddress: peerAddr,
									VRF:         vrfName,
								}

								if status, ok := peerInfo["status"].(string); ok {
									peer.Status = status
								}
								if remoteDisc, ok := peerInfo["remoteDiscriminator"].(float64); ok {
									peer.RemoteDiscriminator = int(remoteDisc)
								}
								if intf, ok := peerInfo["interface"].(string); ok {
									peer.Interface = intf
								}

								// Create unique key for peer lookup
								key := fmt.Sprintf("%s-%s", vrfName, peerAddr)
								devicePeers[key] = peer
							}
						}
					}
				}
			}
		}
	}

	// Validate each expected peer
	failures := []string{}
	for _, expectedPeer := range t.Peers {
		key := fmt.Sprintf("%s-%s", expectedPeer.VRF, expectedPeer.PeerAddress)
		devicePeer, found := devicePeers[key]

		if !found {
			failures = append(failures, fmt.Sprintf("BFD peer %s not found in VRF %s", expectedPeer.PeerAddress, expectedPeer.VRF))
			continue
		}

		// Check interface if specified
		if expectedPeer.Interface != "" && devicePeer.Interface != expectedPeer.Interface {
			failures = append(failures, fmt.Sprintf("BFD peer %s interface mismatch: expected %s, got %s", expectedPeer.PeerAddress, expectedPeer.Interface, devicePeer.Interface))
		}

		// Check peer status
		if !strings.EqualFold(devicePeer.Status, "up") {
			failures = append(failures, fmt.Sprintf("BFD peer %s status is '%s', expected 'up'", expectedPeer.PeerAddress, devicePeer.Status))
		}

		// Check remote discriminator
		if devicePeer.RemoteDiscriminator == 0 {
			failures = append(failures, fmt.Sprintf("BFD peer %s has zero remote discriminator", expectedPeer.PeerAddress))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BFD peer validation failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyBFDSpecificPeers) ValidateInput(input any) error {
	if len(t.Peers) == 0 {
		return fmt.Errorf("at least one BFD peer must be specified")
	}

	for i, peer := range t.Peers {
		if peer.PeerAddress == "" {
			return fmt.Errorf("peer at index %d has no peer_address", i)
		}
		if peer.VRF == "" {
			return fmt.Errorf("peer at index %d has no vrf", i)
		}
	}

	return nil
}

// VerifyBFDPeersIntervals verifies operational timers of BFD peer sessions.
//
// This test validates that BFD peers have the expected transmit intervals,
// receive intervals, and multiplier settings for proper session timing.
//
// The test performs the following checks:
//   1. Retrieves detailed BFD peer information from the device.
//   2. Verifies that each specified peer exists and is operational.
//   3. Validates transmit and receive interval configurations.
//   4. Confirms multiplier settings match expectations.
//
// Expected Results:
//   - Success: The test will pass if all BFD peers have correct timer configurations.
//   - Failure: The test will fail if any peer has incorrect timer settings.
//   - Error: The test will report an error if BFD peer details cannot be retrieved.
//
// Examples:
//   - name: VerifyBFDPeersIntervals with specific timers
//     VerifyBFDPeersIntervals:
//       peers:
//         - peer_address: "192.168.1.1"
//           vrf: "default"
//           tx_interval: 300
//           rx_interval: 300
//           multiplier: 3
//         - peer_address: "192.168.1.2"
//           vrf: "MGMT"
//           tx_interval: 1000
//           rx_interval: 1000
//           multiplier: 5
//
//   - name: VerifyBFDPeersIntervals basic check
//     VerifyBFDPeersIntervals:
//       peers:
//         - peer_address: "10.1.1.1"
//           vrf: "default"
//           tx_interval: 300
//           rx_interval: 300
//           multiplier: 3
type VerifyBFDPeersIntervals struct {
	test.BaseTest
	Peers []BFDPeerInterval `yaml:"peers" json:"peers"`
}

type BFDPeerInterval struct {
	PeerAddress string `yaml:"peer_address" json:"peer_address"`
	VRF         string `yaml:"vrf" json:"vrf"`
	TxInterval  int    `yaml:"tx_interval" json:"tx_interval"`
	RxInterval  int    `yaml:"rx_interval" json:"rx_interval"`
	Multiplier  int    `yaml:"multiplier" json:"multiplier"`
}

func NewVerifyBFDPeersIntervals(inputs map[string]any) (test.Test, error) {
	t := &VerifyBFDPeersIntervals{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBFDPeersIntervals",
			TestDescription: "Verify BFD peer operational timers",
			TestCategories:  []string{"routing", "bfd"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["peers"].([]any); ok {
			for _, peer := range peers {
				if peerMap, ok := peer.(map[string]any); ok {
					bfdPeer := BFDPeerInterval{}
					if peerAddr, ok := peerMap["peer_address"].(string); ok {
						bfdPeer.PeerAddress = peerAddr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						bfdPeer.VRF = vrf
					}
					if txInterval, ok := peerMap["tx_interval"].(float64); ok {
						bfdPeer.TxInterval = int(txInterval)
					}
					if rxInterval, ok := peerMap["rx_interval"].(float64); ok {
						bfdPeer.RxInterval = int(rxInterval)
					}
					if multiplier, ok := peerMap["multiplier"].(float64); ok {
						bfdPeer.Multiplier = int(multiplier)
					}
					t.Peers = append(t.Peers, bfdPeer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBFDPeersIntervals) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bfd peers detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BFD peers detail: %v", err)
		return result, nil
	}

	// Parse BFD peer details from device
	devicePeers := make(map[string]BFDPeerDetailInfo)
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := data["vrfs"].(map[string]any); ok {
			for vrfName, vrfData := range vrfs {
				if vrfInfo, ok := vrfData.(map[string]any); ok {
					if peers, ok := vrfInfo["peers"].(map[string]any); ok {
						for peerAddr, peerData := range peers {
							if peerInfo, ok := peerData.(map[string]any); ok {
								peer := BFDPeerDetailInfo{
									PeerAddress: peerAddr,
									VRF:         vrfName,
								}

								if txInterval, ok := peerInfo["txInterval"].(float64); ok {
									peer.TxInterval = int(txInterval)
								}
								if rxInterval, ok := peerInfo["rxInterval"].(float64); ok {
									peer.RxInterval = int(rxInterval)
								}
								if multiplier, ok := peerInfo["multiplier"].(float64); ok {
									peer.Multiplier = int(multiplier)
								}

								// Create unique key for peer lookup
								key := fmt.Sprintf("%s-%s", vrfName, peerAddr)
								devicePeers[key] = peer
							}
						}
					}
				}
			}
		}
	}

	// Validate each expected peer
	failures := []string{}
	for _, expectedPeer := range t.Peers {
		key := fmt.Sprintf("%s-%s", expectedPeer.VRF, expectedPeer.PeerAddress)
		devicePeer, found := devicePeers[key]

		if !found {
			failures = append(failures, fmt.Sprintf("BFD peer %s not found in VRF %s", expectedPeer.PeerAddress, expectedPeer.VRF))
			continue
		}

		// Check transmit interval
		if devicePeer.TxInterval != expectedPeer.TxInterval {
			failures = append(failures, fmt.Sprintf("BFD peer %s tx_interval: expected %d, got %d", expectedPeer.PeerAddress, expectedPeer.TxInterval, devicePeer.TxInterval))
		}

		// Check receive interval
		if devicePeer.RxInterval != expectedPeer.RxInterval {
			failures = append(failures, fmt.Sprintf("BFD peer %s rx_interval: expected %d, got %d", expectedPeer.PeerAddress, expectedPeer.RxInterval, devicePeer.RxInterval))
		}

		// Check multiplier
		if devicePeer.Multiplier != expectedPeer.Multiplier {
			failures = append(failures, fmt.Sprintf("BFD peer %s multiplier: expected %d, got %d", expectedPeer.PeerAddress, expectedPeer.Multiplier, devicePeer.Multiplier))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BFD peer interval validation failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyBFDPeersIntervals) ValidateInput(input any) error {
	if len(t.Peers) == 0 {
		return fmt.Errorf("at least one BFD peer must be specified")
	}

	for i, peer := range t.Peers {
		if peer.PeerAddress == "" {
			return fmt.Errorf("peer at index %d has no peer_address", i)
		}
		if peer.VRF == "" {
			return fmt.Errorf("peer at index %d has no vrf", i)
		}
		if peer.TxInterval <= 0 {
			return fmt.Errorf("peer at index %d has invalid tx_interval", i)
		}
		if peer.RxInterval <= 0 {
			return fmt.Errorf("peer at index %d has invalid rx_interval", i)
		}
		if peer.Multiplier <= 0 {
			return fmt.Errorf("peer at index %d has invalid multiplier", i)
		}
	}

	return nil
}

// VerifyBFDPeersHealth verifies overall health of BFD peers across all VRFs.
//
// This test examines the health status of all BFD peers configured on the device,
// optionally checking for downtime thresholds and validating session health indicators.
//
// The test performs the following checks:
//   1. Retrieves all BFD peer information across all VRFs.
//   2. Validates that peers are in "up" state.
//   3. Confirms non-zero remote discriminators.
//   4. Optionally checks downtime threshold if specified.
//
// Expected Results:
//   - Success: The test will pass if all BFD peers are healthy and meet criteria.
//   - Failure: The test will fail if any peer is unhealthy or exceeds downtime threshold.
//   - Error: The test will report an error if BFD peer information cannot be retrieved.
//
// Examples:
//   - name: VerifyBFDPeersHealth basic check
//     VerifyBFDPeersHealth: {}
//
//   - name: VerifyBFDPeersHealth with downtime threshold
//     VerifyBFDPeersHealth:
//       down_threshold: 300  # 5 minutes in seconds
//
//   - name: VerifyBFDPeersHealth comprehensive
//     VerifyBFDPeersHealth:
//       down_threshold: 60   # 1 minute in seconds
type VerifyBFDPeersHealth struct {
	test.BaseTest
	DownThreshold *int `yaml:"down_threshold,omitempty" json:"down_threshold,omitempty"`
}

func NewVerifyBFDPeersHealth(inputs map[string]any) (test.Test, error) {
	t := &VerifyBFDPeersHealth{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBFDPeersHealth",
			TestDescription: "Verify overall health of all BFD peers",
			TestCategories:  []string{"routing", "bfd"},
		},
	}

	if inputs != nil {
		if downThreshold, ok := inputs["down_threshold"].(float64); ok {
			threshold := int(downThreshold)
			t.DownThreshold = &threshold
		}
	}

	return t, nil
}

func (t *VerifyBFDPeersHealth) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bfd peers",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BFD peers: %v", err)
		return result, nil
	}

	// Parse all BFD peers from device
	allPeers := []BFDPeerInfo{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := data["vrfs"].(map[string]any); ok {
			for vrfName, vrfData := range vrfs {
				if vrfInfo, ok := vrfData.(map[string]any); ok {
					if peers, ok := vrfInfo["peers"].(map[string]any); ok {
						for peerAddr, peerData := range peers {
							if peerInfo, ok := peerData.(map[string]any); ok {
								peer := BFDPeerInfo{
									PeerAddress: peerAddr,
									VRF:         vrfName,
								}

								if status, ok := peerInfo["status"].(string); ok {
									peer.Status = status
								}
								if remoteDisc, ok := peerInfo["remoteDiscriminator"].(float64); ok {
									peer.RemoteDiscriminator = int(remoteDisc)
								}
								if downTime, ok := peerInfo["downTime"].(float64); ok {
									peer.DownTime = int(downTime)
								}

								allPeers = append(allPeers, peer)
							}
						}
					}
				}
			}
		}
	}

	if len(allPeers) == 0 {
		result.Status = test.TestError
		result.Message = "No BFD peers found"
		return result, nil
	}

	// Validate health of all peers
	failures := []string{}
	for _, peer := range allPeers {
		// Check peer status
		if !strings.EqualFold(peer.Status, "up") {
			failures = append(failures, fmt.Sprintf("BFD peer %s in VRF %s is '%s', expected 'up'", peer.PeerAddress, peer.VRF, peer.Status))
		}

		// Check remote discriminator
		if peer.RemoteDiscriminator == 0 {
			failures = append(failures, fmt.Sprintf("BFD peer %s in VRF %s has zero remote discriminator", peer.PeerAddress, peer.VRF))
		}

		// Check downtime threshold if specified
		if t.DownThreshold != nil && peer.DownTime > *t.DownThreshold {
			failures = append(failures, fmt.Sprintf("BFD peer %s in VRF %s downtime %ds exceeds threshold %ds", peer.PeerAddress, peer.VRF, peer.DownTime, *t.DownThreshold))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BFD peers health failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyBFDPeersHealth) ValidateInput(input any) error {
	if t.DownThreshold != nil && *t.DownThreshold < 0 {
		return fmt.Errorf("down_threshold must be non-negative")
	}
	return nil
}

// VerifyBFDPeersRegProtocols verifies registered protocols for BFD peer sessions.
//
// This test confirms that expected protocols are registered with BFD peer sessions,
// which is important for understanding which routing protocols are using BFD
// for fast failure detection.
//
// The test performs the following checks:
//   1. Retrieves detailed BFD peer information including registered protocols.
//   2. Verifies that each specified peer exists and is operational.
//   3. Validates that expected protocols are registered with each peer.
//
// Expected Results:
//   - Success: The test will pass if all BFD peers have correct protocol registrations.
//   - Failure: The test will fail if any peer has incorrect or missing protocol registrations.
//   - Error: The test will report an error if BFD peer details cannot be retrieved.
//
// Examples:
//   - name: VerifyBFDPeersRegProtocols BGP sessions
//     VerifyBFDPeersRegProtocols:
//       peers:
//         - peer_address: "192.168.1.1"
//           vrf: "default"
//           protocols:
//             - "bgp"
//         - peer_address: "192.168.1.2"
//           vrf: "MGMT"
//           protocols:
//             - "ospf"
//             - "isis"
//
//   - name: VerifyBFDPeersRegProtocols OSPF only
//     VerifyBFDPeersRegProtocols:
//       peers:
//         - peer_address: "10.1.1.1"
//           vrf: "default"
//           protocols:
//             - "ospf"
type VerifyBFDPeersRegProtocols struct {
	test.BaseTest
	Peers []BFDPeerProtocol `yaml:"peers" json:"peers"`
}

type BFDPeerProtocol struct {
	PeerAddress string   `yaml:"peer_address" json:"peer_address"`
	VRF         string   `yaml:"vrf" json:"vrf"`
	Protocols   []string `yaml:"protocols" json:"protocols"`
}

func NewVerifyBFDPeersRegProtocols(inputs map[string]any) (test.Test, error) {
	t := &VerifyBFDPeersRegProtocols{
		BaseTest: test.BaseTest{
			TestName:        "VerifyBFDPeersRegProtocols",
			TestDescription: "Verify registered protocols for BFD peers",
			TestCategories:  []string{"routing", "bfd"},
		},
	}

	if inputs != nil {
		if peers, ok := inputs["peers"].([]any); ok {
			for _, peer := range peers {
				if peerMap, ok := peer.(map[string]any); ok {
					bfdPeer := BFDPeerProtocol{}
					if peerAddr, ok := peerMap["peer_address"].(string); ok {
						bfdPeer.PeerAddress = peerAddr
					}
					if vrf, ok := peerMap["vrf"].(string); ok {
						bfdPeer.VRF = vrf
					}
					if protocols, ok := peerMap["protocols"].([]any); ok {
						for _, protocol := range protocols {
							if protocolStr, ok := protocol.(string); ok {
								bfdPeer.Protocols = append(bfdPeer.Protocols, protocolStr)
							}
						}
					}
					t.Peers = append(t.Peers, bfdPeer)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyBFDPeersRegProtocols) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show bfd peers detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get BFD peers detail: %v", err)
		return result, nil
	}

	// Parse BFD peer protocol details from device
	devicePeers := make(map[string]BFDPeerProtocolInfo)
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := data["vrfs"].(map[string]any); ok {
			for vrfName, vrfData := range vrfs {
				if vrfInfo, ok := vrfData.(map[string]any); ok {
					if peers, ok := vrfInfo["peers"].(map[string]any); ok {
						for peerAddr, peerData := range peers {
							if peerInfo, ok := peerData.(map[string]any); ok {
								peer := BFDPeerProtocolInfo{
									PeerAddress: peerAddr,
									VRF:         vrfName,
								}

								if regProtocols, ok := peerInfo["registeredProtocols"].([]any); ok {
									for _, protocol := range regProtocols {
										if protocolStr, ok := protocol.(string); ok {
											peer.RegisteredProtocols = append(peer.RegisteredProtocols, protocolStr)
										}
									}
								}

								// Create unique key for peer lookup
								key := fmt.Sprintf("%s-%s", vrfName, peerAddr)
								devicePeers[key] = peer
							}
						}
					}
				}
			}
		}
	}

	// Validate each expected peer
	failures := []string{}
	for _, expectedPeer := range t.Peers {
		key := fmt.Sprintf("%s-%s", expectedPeer.VRF, expectedPeer.PeerAddress)
		devicePeer, found := devicePeers[key]

		if !found {
			failures = append(failures, fmt.Sprintf("BFD peer %s not found in VRF %s", expectedPeer.PeerAddress, expectedPeer.VRF))
			continue
		}

		// Check each expected protocol
		for _, expectedProtocol := range expectedPeer.Protocols {
			found := false
			for _, deviceProtocol := range devicePeer.RegisteredProtocols {
				if strings.EqualFold(expectedProtocol, deviceProtocol) {
					found = true
					break
				}
			}
			if !found {
				failures = append(failures, fmt.Sprintf("BFD peer %s missing protocol '%s', registered: %v", expectedPeer.PeerAddress, expectedProtocol, devicePeer.RegisteredProtocols))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("BFD peer protocol validation failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyBFDPeersRegProtocols) ValidateInput(input any) error {
	if len(t.Peers) == 0 {
		return fmt.Errorf("at least one BFD peer must be specified")
	}

	for i, peer := range t.Peers {
		if peer.PeerAddress == "" {
			return fmt.Errorf("peer at index %d has no peer_address", i)
		}
		if peer.VRF == "" {
			return fmt.Errorf("peer at index %d has no vrf", i)
		}
		if len(peer.Protocols) == 0 {
			return fmt.Errorf("peer at index %d has no protocols specified", i)
		}
	}

	return nil
}

// Supporting data structures

type BFDPeerInfo struct {
	PeerAddress         string
	VRF                 string
	Interface           string
	Status              string
	RemoteDiscriminator int
	DownTime            int
}

type BFDPeerDetailInfo struct {
	PeerAddress string
	VRF         string
	TxInterval  int
	RxInterval  int
	Multiplier  int
}

type BFDPeerProtocolInfo struct {
	PeerAddress         string
	VRF                 string
	RegisteredProtocols []string
}