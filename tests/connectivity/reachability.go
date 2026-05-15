package connectivity

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyReachability verifies IP reachability to one or more destinations
// using gNOI System.Ping. Requires `transport: gnmi` on the device; the
// eAPI transport returns TestError because the only eAPI path (parsing
// free-form `ping` output) is too fragile to keep alive.
//
// Inputs:
//
//	hosts:
//	  - destination: "8.8.8.8"
//	    vrf: "default"            # optional, "" = default VRF
//	    source: "10.0.0.1"        # optional, MUST be an IP — not an interface name
//	    reachable: true           # optional, defaults to true
//	repeat: 3                     # echoes per host, default 2
//	size: 100                     # bytes per echo, optional
//
// Source semantics: Arista's gNOI rejects interface names ("Loopback0")
// because it forwards the value to Linux ping, which doesn't know EOS
// interface naming. ValidateInput catches non-IP sources up front so the
// failure surfaces as a clear test-config error rather than a generic
// device error.
type VerifyReachability struct {
	test.BaseTest
	Hosts  []ReachabilityHost `yaml:"hosts" json:"hosts"`
	Repeat int                `yaml:"repeat,omitempty" json:"repeat,omitempty"`
	Size   int                `yaml:"size,omitempty" json:"size,omitempty"`
}

type ReachabilityHost struct {
	Destination string `yaml:"destination" json:"destination"`
	VRF         string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
	Source      string `yaml:"source,omitempty" json:"source,omitempty"`
	Reachable   bool   `yaml:"reachable" json:"reachable"`
}

func NewVerifyReachability(inputs map[string]any) (test.Test, error) {
	t := &VerifyReachability{
		BaseTest: test.BaseTest{
			TestName:        "VerifyReachability",
			TestDescription: "Verify network reachability via gNOI System.Ping",
			TestCategories:  []string{"connectivity", "reachability"},
		},
		Repeat: 2,
		Size:   100,
	}

	if inputs == nil {
		return t, nil
	}

	if hosts, ok := inputs["hosts"].([]any); ok {
		for _, h := range hosts {
			hostMap, ok := h.(map[string]any)
			if !ok {
				continue
			}
			host := ReachabilityHost{Reachable: true}
			if dest, ok := hostMap["destination"].(string); ok {
				host.Destination = dest
			}
			if vrf, ok := hostMap["vrf"].(string); ok {
				host.VRF = vrf
			}
			if src, ok := hostMap["source"].(string); ok {
				host.Source = src
			}
			if reachable, ok := hostMap["reachable"].(bool); ok {
				host.Reachable = reachable
			}
			t.Hosts = append(t.Hosts, host)
		}
	}

	if repeat, ok := inputs["repeat"].(float64); ok {
		t.Repeat = int(repeat)
	} else if repeat, ok := inputs["repeat"].(int); ok {
		t.Repeat = repeat
	}
	if size, ok := inputs["size"].(float64); ok {
		t.Size = int(size)
	} else if size, ok := inputs["size"].(int); ok {
		t.Size = size
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
		opts := device.PingOpts{
			Destination: host.Destination,
			Source:      host.Source,
			VRF:         host.VRF,
			Count:       t.Repeat,
			Size:        t.Size,
		}
		res, err := dev.Ping(ctx, opts)
		if err != nil {
			if errors.Is(err, device.ErrDiagUnsupported) {
				result.Status = test.TestError
				result.Message = "VerifyReachability requires transport: gnmi (eAPI cannot serve gNOI Ping)"
				return result, nil
			}
			if host.Reachable {
				failures = append(failures, fmt.Sprintf("%s: ping error - %v", host.Destination, err))
			}
			continue
		}

		switch {
		case host.Reachable && res.Stats.Received != res.Stats.Sent:
			failures = append(failures, fmt.Sprintf("%s - Packet loss detected - Transmitted: %d Received: %d",
				host.Destination, res.Stats.Sent, res.Stats.Received))
		case !host.Reachable && res.Stats.Received != 0:
			failures = append(failures, fmt.Sprintf("%s - Destination is expected to be unreachable but %d/%d replies received",
				host.Destination, res.Stats.Received, res.Stats.Sent))
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Reachability failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyReachability) ValidateInput(input any) error {
	if len(t.Hosts) == 0 {
		return fmt.Errorf("at least one host must be specified")
	}
	for i, host := range t.Hosts {
		if host.Destination == "" {
			return fmt.Errorf("host[%d] has no destination", i)
		}
		if net.ParseIP(host.Destination) == nil {
			return fmt.Errorf("host[%d] destination %q is not a valid IP address", i, host.Destination)
		}
		if host.Source != "" && net.ParseIP(host.Source) == nil {
			return fmt.Errorf("host[%d] source %q must be a literal IP address (interface names like %q are rejected by EOS gNOI)",
				i, host.Source, host.Source)
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
