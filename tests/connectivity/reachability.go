package connectivity

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyReachability verifies IP reachability to specified destinations using ping.
//
// Expected Results:
//   - Success: The test will pass if all specified destinations have the expected reachability state.
//   - Failure: The test will fail if any destination does not match the expected reachability state.
//   - Error: The test will report an error if ping commands cannot be executed.
//
// Examples:
//   - name: VerifyReachability to external hosts
//     VerifyReachability:
//       hosts:
//         - destination: "8.8.8.8"
//           vrf: "default"
//           reachable: true
//         - destination: "1.1.1.1"
//           reachable: true
//       repeat: 3
//       size: 100
//
//   - name: VerifyReachability negative test
//     VerifyReachability:
//       hosts:
//         - destination: "192.0.2.1"  # Should not be reachable
//           reachable: false
type VerifyReachability struct {
	test.BaseTest
	Hosts  []ReachabilityHost `yaml:"hosts" json:"hosts"`
	Repeat int                `yaml:"repeat,omitempty" json:"repeat,omitempty"`
	Size   int                `yaml:"size,omitempty" json:"size,omitempty"`
}

type ReachabilityHost struct {
	Destination string `yaml:"destination" json:"destination"`
	VRF         string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
	Reachable   bool   `yaml:"reachable" json:"reachable"`
}

func NewVerifyReachability(inputs map[string]any) (test.Test, error) {
	t := &VerifyReachability{
		BaseTest: test.BaseTest{
			TestName:        "VerifyReachability",
			TestDescription: "Verify network reachability to specified destinations",
			TestCategories:  []string{"connectivity", "reachability"},
		},
		Repeat: 2,
		Size:   100,
	}

	if inputs != nil {
		if hosts, ok := inputs["hosts"].([]interface{}); ok {
			for _, h := range hosts {
				if hostMap, ok := h.(map[string]interface{}); ok {
					host := ReachabilityHost{
						Reachable: true,
					}
					if dest, ok := hostMap["destination"].(string); ok {
						host.Destination = dest
					}
					if vrf, ok := hostMap["vrf"].(string); ok {
						host.VRF = vrf
					}
					if reachable, ok := hostMap["reachable"].(bool); ok {
						host.Reachable = reachable
					}
					t.Hosts = append(t.Hosts, host)
				}
			}
		}

		if repeat, ok := inputs["repeat"].(int); ok {
			t.Repeat = repeat
		}
		if size, ok := inputs["size"].(int); ok {
			t.Size = size
		}
	}

	return t, nil
}

func (t *VerifyReachability) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}


	if len(t.Hosts) == 0 {
		result.Status = test.TestError
		result.Message = "No hosts configured for reachability test"
		return result, nil
	}

	failures := []string{}
	for _, host := range t.Hosts {
		cmd := device.Command{
			Template: fmt.Sprintf("ping %s", host.Destination),
			Format:   "json",
			UseCache: false,
		}

		if host.VRF != "" && host.VRF != "default" {
			cmd.Template = fmt.Sprintf("ping vrf %s %s", host.VRF, host.Destination)
		}

		if t.Repeat > 0 {
			cmd.Template += fmt.Sprintf(" repeat %d", t.Repeat)
		}
		if t.Size > 0 {
			cmd.Template += fmt.Sprintf(" size %d", t.Size)
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			if host.Reachable {
				failures = append(failures, fmt.Sprintf("%s: error executing ping - %v", host.Destination, err))
			}
			continue
		}

		// Extract message from EOS JSON output - EOS wraps the command result
		// The actual structure is: result[0].messages[0] contains the ping output text
		if pingResult, ok := cmdResult.Output.(map[string]interface{}); ok {
			if messages, ok := pingResult["messages"].([]interface{}); ok && len(messages) > 0 {
				if message, ok := messages[0].(string); ok {
					if failure := t.isHostReachable(host, message); failure != "" {
						failures = append(failures, failure)
					}
				} else {
					if host.Reachable {
						failures = append(failures, fmt.Sprintf("%s: ping message not a string - type: %T",
							host.Destination, messages[0]))
					}
				}
			} else {
				if host.Reachable {
					failures = append(failures, fmt.Sprintf("%s: no ping messages found in result", host.Destination))
				}
			}
		} else {
			if host.Reachable {
				failures = append(failures, fmt.Sprintf("%s: unexpected JSON structure - type: %T",
					host.Destination, cmdResult.Output))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Reachability failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyReachability) ValidateInput(input any) error {
	if t.Hosts == nil || len(t.Hosts) == 0 {
		return fmt.Errorf("at least one host must be specified")
	}

	for i, host := range t.Hosts {
		if host.Destination == "" {
			return fmt.Errorf("host at index %d has no destination", i)
		}
	}

	if t.Repeat < 0 {
		return fmt.Errorf("repeat count cannot be negative")
	}

	if t.Size < 0 {
		return fmt.Errorf("packet size cannot be negative")
	}

	return nil
}

// isHostReachable checks if a host is reachable based on ping message, following Python ANTA pattern
// Returns empty string if test passes, or failure message if test fails
func (t *VerifyReachability) isHostReachable(host ReachabilityHost, message string) string {
	// Extract received packet count using regex, limiting digits to prevent ReDoS
	// From the actual EOS output: "2 packets transmitted, 2 received, 0% packet loss"
	pattern := regexp.MustCompile(`(\d{1,20})\s+received`)
	matches := pattern.FindStringSubmatch(message)

	receivedPackets := 0
	if len(matches) > 1 {
		if count, err := strconv.Atoi(matches[1]); err == nil {
			receivedPackets = count
		}
	}

	expectedPackets := t.Repeat
	if expectedPackets == 0 {
		expectedPackets = 5 // EOS default
	}


	// Check if host is expected to be reachable
	if host.Reachable && receivedPackets != expectedPackets {
		return fmt.Sprintf("%s - Packet loss detected - Transmitted: %d Received: %d",
			host.Destination, expectedPackets, receivedPackets)
	}

	// Check if host is expected to be unreachable
	if !host.Reachable && receivedPackets != 0 {
		return fmt.Sprintf("%s - Destination is expected to be unreachable but found reachable",
			host.Destination)
	}

	return "" // Test passed
}
