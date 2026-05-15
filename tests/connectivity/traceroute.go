package connectivity

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyTraceroute verifies the network path to one or more destinations
// via gNOI System.Traceroute. Requires `transport: gnmi`.
//
// EOS slowness caveat: Arista's gNOI Traceroute buffers the entire
// response on the device and flushes at completion — including probing
// the remaining TTLs up to MaxTTL even after the destination has been
// reached. Defaults are conservative (MaxTTL=3, Wait=1s) so an
// otherwise-one-hop traceroute returns within ~10s. Override per-host
// when paths are longer.
//
// Inputs:
//
//	destinations:
//	  - destination: "fc00:800f:1f::d"
//	    vrf: "default"               # optional
//	    source: "fc00:800f:f::6"     # optional, IP only
//	    max_hops: 5                  # optional, default 3
//	    expected_hops:               # optional ordered list, each
//	      - hop: 1                   # entry asserts which address is
//	        address: "fc00:800f:1f::1" # seen at the given hop
//	      - hop: 2
//	        address: "fc00:800f:1f::d"
//	    must_reach: true             # optional, defaults true — fails if
//	                                 # destination address is not in the
//	                                 # observed hop set
type VerifyTraceroute struct {
	test.BaseTest
	Destinations []TracerouteTarget `yaml:"destinations" json:"destinations"`
}

type TracerouteTarget struct {
	Destination  string                `yaml:"destination" json:"destination"`
	VRF          string                `yaml:"vrf,omitempty" json:"vrf,omitempty"`
	Source       string                `yaml:"source,omitempty" json:"source,omitempty"`
	MaxHops      int                   `yaml:"max_hops,omitempty" json:"max_hops,omitempty"`
	ExpectedHops []TracerouteHopAssert `yaml:"expected_hops,omitempty" json:"expected_hops,omitempty"`
	MustReach    bool                  `yaml:"must_reach,omitempty" json:"must_reach,omitempty"`
}

type TracerouteHopAssert struct {
	Hop     int    `yaml:"hop" json:"hop"`
	Address string `yaml:"address" json:"address"`
}

func NewVerifyTraceroute(inputs map[string]any) (test.Test, error) {
	t := &VerifyTraceroute{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTraceroute",
			TestDescription: "Verify network path to destinations via gNOI System.Traceroute",
			TestCategories:  []string{"connectivity", "traceroute"},
		},
	}

	if inputs == nil {
		return t, nil
	}

	dests, ok := inputs["destinations"].([]any)
	if !ok {
		return t, nil
	}
	for _, d := range dests {
		m, ok := d.(map[string]any)
		if !ok {
			continue
		}
		dest := TracerouteTarget{MaxHops: 3, MustReach: true}
		if v, ok := m["destination"].(string); ok {
			dest.Destination = v
		}
		if v, ok := m["vrf"].(string); ok {
			dest.VRF = v
		}
		if v, ok := m["source"].(string); ok {
			dest.Source = v
		}
		if v, ok := m["max_hops"].(float64); ok {
			dest.MaxHops = int(v)
		} else if v, ok := m["max_hops"].(int); ok {
			dest.MaxHops = v
		}
		if v, ok := m["must_reach"].(bool); ok {
			dest.MustReach = v
		}
		if rawHops, ok := m["expected_hops"].([]any); ok {
			for _, h := range rawHops {
				hm, ok := h.(map[string]any)
				if !ok {
					continue
				}
				assert := TracerouteHopAssert{}
				if v, ok := hm["hop"].(float64); ok {
					assert.Hop = int(v)
				} else if v, ok := hm["hop"].(int); ok {
					assert.Hop = v
				}
				if v, ok := hm["address"].(string); ok {
					assert.Address = v
				}
				dest.ExpectedHops = append(dest.ExpectedHops, assert)
			}
		}
		t.Destinations = append(t.Destinations, dest)
	}

	return t, nil
}

func (t *VerifyTraceroute) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.Destinations) == 0 {
		result.Status = test.TestError
		result.Message = "No destinations configured for traceroute test"
		return result, nil
	}

	failures := []string{}
	for _, dest := range t.Destinations {
		opts := device.TracerouteOpts{
			Destination:  dest.Destination,
			Source:       dest.Source,
			VRF:          dest.VRF,
			MaxTTL:       dest.MaxHops,
			Wait:         1 * time.Second,
			DoNotResolve: true,
		}
		res, err := dev.Traceroute(ctx, opts)
		if err != nil {
			if errors.Is(err, device.ErrDiagUnsupported) {
				result.Status = test.TestError
				result.Message = "VerifyTraceroute requires transport: gnmi (eAPI cannot serve gNOI Traceroute)"
				return result, nil
			}
			failures = append(failures, fmt.Sprintf("%s: traceroute error - %v", dest.Destination, err))
			continue
		}

		// Collect observed addresses per hop (first probe wins; the
		// downstream assertions are point-in-time, not statistical).
		observed := map[int]string{}
		var reached bool
		for _, hop := range res.Hops {
			for _, p := range hop.Probes {
				if p.Address != "" {
					observed[hop.Number] = p.Address
					if strings.EqualFold(p.Address, dest.Destination) {
						reached = true
					}
					break
				}
			}
		}

		if dest.MustReach && !reached {
			failures = append(failures, fmt.Sprintf("%s: destination not in observed hops (saw %d hops, MaxTTL=%d)",
				dest.Destination, len(res.Hops), dest.MaxHops))
		}

		for _, assert := range dest.ExpectedHops {
			got, ok := observed[assert.Hop]
			if !ok {
				failures = append(failures, fmt.Sprintf("%s: expected hop %d not present in trace",
					dest.Destination, assert.Hop))
				continue
			}
			if !strings.EqualFold(got, assert.Address) {
				failures = append(failures, fmt.Sprintf("%s: hop %d expected %q, got %q",
					dest.Destination, assert.Hop, assert.Address, got))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Traceroute failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyTraceroute) ValidateInput(input any) error {
	if len(t.Destinations) == 0 {
		return fmt.Errorf("at least one destination must be specified")
	}
	for i, d := range t.Destinations {
		if d.Destination == "" {
			return fmt.Errorf("destinations[%d] has no destination", i)
		}
		if net.ParseIP(d.Destination) == nil {
			return fmt.Errorf("destinations[%d] destination %q is not a valid IP address", i, d.Destination)
		}
		if d.Source != "" && net.ParseIP(d.Source) == nil {
			return fmt.Errorf("destinations[%d] source %q must be a literal IP (interface names like %q are rejected by EOS gNOI)",
				i, d.Source, d.Source)
		}
		if d.MaxHops < 0 {
			return fmt.Errorf("destinations[%d] max_hops cannot be negative", i)
		}
		for j, h := range d.ExpectedHops {
			if h.Hop < 1 {
				return fmt.Errorf("destinations[%d].expected_hops[%d] hop must be >= 1", i, j)
			}
			if h.Address == "" {
				return fmt.Errorf("destinations[%d].expected_hops[%d] address is required", i, j)
			}
		}
	}
	return nil
}
