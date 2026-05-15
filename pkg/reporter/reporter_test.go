package reporter

import (
	"strings"
	"testing"
	"time"

	"github.com/fluidstackio/go-anta/pkg/test"
)

func sampleReport() *Report {
	start := time.Date(2026, 5, 14, 9, 0, 0, 0, time.UTC)
	return &Report{
		Title:     "smoke",
		Started:   start,
		Completed: start.Add(3 * time.Second),
		Devices: []DeviceInfo{
			{
				Name:       "leaf1",
				Host:       "10.0.0.1",
				Transport:  "eapi",
				Port:       443,
				Model:      "DCS-7060DX4-32-F",
				EOSVersion: "4.34.4M",
				Tags:       []string{"lab", "tor"},
				Connected:  true,
			},
			{
				Name:         "leaf2",
				Host:         "10.0.0.2",
				Transport:    "gnmi",
				Port:         6030,
				Connected:    false,
				ConnectError: "connection refused",
			},
		},
		Results: []test.TestResult{
			{TestName: "VerifyEOSVersion", DeviceName: "leaf1", Status: test.TestSuccess, Duration: 100 * time.Millisecond, Categories: []string{"system", "version"}},
			{TestName: "VerifyTemperature", DeviceName: "leaf1", Status: test.TestSuccess, Duration: 250 * time.Millisecond},
			{TestName: "VerifyHostname", DeviceName: "leaf1", Status: test.TestFailure, Message: "Hostname is 'leaf1-old', expected 'leaf1'", Duration: 80 * time.Millisecond,
				Details: map[string]any{"actual": "leaf1-old", "expected": "leaf1"}},
		},
	}
}

func TestRender_ContainsDeviceAndStatus(t *testing.T) {
	body, err := RenderToBytes(sampleReport())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(body)

	for _, want := range []string{
		"leaf1",
		"leaf2",
		"DCS-7060DX4-32-F",
		"4.34.4M",
		"connection refused",
		"VerifyEOSVersion",
		"VerifyHostname",
		"Hostname is &#39;leaf1-old&#39;",       // HTML-escaped
		`&#34;actual&#34;: &#34;leaf1-old&#34;`, // Details pretty-printed (HTML-escaped quotes)
		"smoke",                                 // title
		"3s",                                    // duration
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestRender_TotalsAddUp(t *testing.T) {
	body, _ := RenderToBytes(sampleReport())
	s := string(body)
	// 3 total: 2 success, 1 failure, 0 error
	if !strings.Contains(s, "<strong>3</strong> total") {
		t.Error("totals should show 3 total")
	}
	if !strings.Contains(s, "<strong>2</strong> success") {
		t.Error("totals should show 2 success")
	}
	if !strings.Contains(s, "<strong>1</strong> failure") {
		t.Error("totals should show 1 failure")
	}
}

func TestRender_DisconnectedDeviceMarked(t *testing.T) {
	body, _ := RenderToBytes(sampleReport())
	s := string(body)
	if !strings.Contains(s, `class="dot disconnected"`) {
		t.Error("disconnected device should get the disconnected dot")
	}
	if !strings.Contains(s, `class="dot connected"`) {
		t.Error("connected device should get the connected dot")
	}
}

func TestRender_OrphanResultStillRenders(t *testing.T) {
	// A test result for a device not in Devices should still show up,
	// so a runner glitch doesn't silently swallow output.
	r := &Report{
		Started: time.Now(),
		Results: []test.TestResult{
			{TestName: "VerifyX", DeviceName: "ghost", Status: test.TestError, Message: "no inventory entry"},
		},
	}
	body, err := RenderToBytes(r)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(body), "ghost") {
		t.Error("orphan device should still appear")
	}
}

func TestRender_StatusOrdering(t *testing.T) {
	// Errors then failures then successes inside a device — failures
	// should appear before successes in the markup.
	body, _ := RenderToBytes(sampleReport())
	s := string(body)
	failIdx := strings.Index(s, "VerifyHostname")
	succIdx := strings.Index(s, "VerifyEOSVersion")
	if failIdx < 0 || succIdx < 0 {
		t.Fatal("both test names should be present")
	}
	if failIdx > succIdx {
		t.Errorf("failures should sort before successes; failure @%d, success @%d", failIdx, succIdx)
	}
}
