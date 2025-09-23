package connectivity

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

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

func NewVerifyReachability(inputs map[string]interface{}) (test.Test, error) {
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

		if pingResult, ok := cmdResult.Output.(map[string]interface{}); ok {
			success := false
			
			if messages, ok := pingResult["messages"].([]interface{}); ok && len(messages) > 0 {
				for _, msg := range messages {
					if msgStr, ok := msg.(string); ok {
						if contains(msgStr, "bytes from") || contains(msgStr, "Success rate is") {
							transmitted := 0
							received := 0
							
							if stats, ok := pingResult["stats"].(map[string]interface{}); ok {
								if tx, ok := stats["transmitted"].(float64); ok {
									transmitted = int(tx)
								}
								if rx, ok := stats["received"].(float64); ok {
									received = int(rx)
								}
							}
							
							if transmitted > 0 && received > 0 {
								success = true
							}
							break
						}
					}
				}
			}

			if success != host.Reachable {
				if host.Reachable {
					failures = append(failures, fmt.Sprintf("%s: expected reachable but was unreachable", host.Destination))
				} else {
					failures = append(failures, fmt.Sprintf("%s: expected unreachable but was reachable", host.Destination))
				}
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Reachability failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyReachability) ValidateInput(input interface{}) error {
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || 
		   len(s) > len(substr) && containsHelper(s[1:], substr)
}

func containsHelper(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	if s[:len(substr)] == substr {
		return true
	}
	return containsHelper(s[1:], substr)
}