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
		"Hostname is &#39;leaf1-old&#39;", // HTML-escaped
		"<dt>Actual</dt>",                 // Details rendered as summary kv
		"<dd>leaf1-old</dd>",
		"smoke", // title
		"3s",    // duration
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

func TestRender_FansTable(t *testing.T) {
	// A test result with the same shape VerifyEnvironmentCooling now
	// produces should render as a real table with the fan icon, a
	// status pill, and speed columns — not as raw JSON.
	r := &Report{
		Started: time.Now(),
		Devices: []DeviceInfo{{Name: "tor1", Connected: true}},
		Results: []test.TestResult{
			{
				TestName:   "VerifyEnvironmentCooling",
				DeviceName: "tor1",
				Status:     test.TestSuccess,
				Details: map[string]any{
					"fan_count":         3,
					"cooling_mode":      "automatic",
					"airflow_direction": "frontToBackAirflow",
					"fans": []any{
						map[string]any{
							"name":             "1",
							"container":        "FanTraySlot/1",
							"label":            "1/1",
							"status":           "ok",
							"actual_speed_pct": float64(29),
							"configured_pct":   float64(30),
						},
						map[string]any{
							"name":             "2",
							"container":        "FanTraySlot/2",
							"label":            "2/1",
							"status":           "failed",
							"actual_speed_pct": float64(0),
							"configured_pct":   float64(30),
						},
					},
				},
			},
		},
	}
	body, err := RenderToBytes(r)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		`class="detail"`,             // table renders
		`class="icon fan"`,           // icon appears
		`class="pill success"`,       // ok status colored
		`class="pill failure"`,       // failed status colored
		"FanTraySlot/1",              // container shown
		"1/1",                        // label shown
		"29%",                        // speed shown
		`<dt>Cooling mode</dt>`,      // summary key-value
		`<dd>automatic</dd>`,         // summary value
		`<dt>Airflow direction</dt>`, // humanized snake_case key
		`<dd>frontToBackAirflow</dd>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
	// Make sure the raw JSON `{"fans": ...}` blob is NOT in the output
	// — that would mean the table fell through to JSON-fallback.
	if strings.Contains(s, `"fan_count": 3`) {
		t.Errorf("fans should render as table, not as raw JSON")
	}
}

func TestRender_PSUsTable(t *testing.T) {
	r := &Report{
		Started: time.Now(),
		Devices: []DeviceInfo{{Name: "tor1", Connected: true}},
		Results: []test.TestResult{
			{
				TestName:   "VerifyEnvironmentPower",
				DeviceName: "tor1",
				Status:     test.TestSuccess,
				Details: map[string]any{
					"power_supplies": []any{
						map[string]any{
							"slot":           "1",
							"model":          "PWR-3001-AC-RED",
							"state":          "ok",
							"input_voltage":  float64(207.5),
							"output_power_w": float64(276.5),
							"capacity_w":     float64(3000),
						},
						map[string]any{
							"slot":  "2",
							"model": "PWR-3001-AC-RED",
							"state": "failed",
						},
					},
				},
			},
		},
	}
	body, _ := RenderToBytes(r)
	s := string(body)
	for _, want := range []string{
		`class="icon psu"`,
		"PWR-3001-AC-RED",
		"207.5 V",
		"276 W",                // output power rounded (Go bankers' rounding)
		"3000 W",               // capacity
		"9%",                   // load = 276.5/3000 = 9.2% → "9%"
		`class="pill success"`, // PSU 1 ok
		`class="pill failure"`, // PSU 2 failed
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestRender_TempsTable(t *testing.T) {
	r := &Report{
		Started: time.Now(),
		Devices: []DeviceInfo{{Name: "tor1", Connected: true}},
		Results: []test.TestResult{
			{
				TestName:   "VerifyTemperature",
				DeviceName: "tor1",
				Status:     test.TestSuccess,
				Details: map[string]any{
					"temperatures": []any{
						map[string]any{
							"name":        "T1",
							"description": "Ambient",
							"container":   "chassis",
							"current_c":   float64(32.5),
							"overheat_c":  float64(65),
							"critical_c":  float64(70),
							"status":      "ok",
						},
						map[string]any{
							"name":        "T2",
							"description": "ASIC",
							"container":   "chassis",
							"current_c":   float64(95),
							"overheat_c":  float64(95),
							"critical_c":  float64(100),
							"status":      "ok",
						},
					},
				},
			},
		},
	}
	body, _ := RenderToBytes(r)
	s := string(body)
	for _, want := range []string{
		`class="icon temp"`,
		"32.5 °C",
		"65 °C",
		"95 °C",
		`class="bar hot"`, // T2 is at 100% of overheat
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestRender_OpticsTable(t *testing.T) {
	r := &Report{
		Started: time.Now(),
		Devices: []DeviceInfo{{Name: "tor1", Connected: true}},
		Results: []test.TestResult{
			{
				TestName:   "VerifyTransceivers",
				DeviceName: "tor1",
				Status:     test.TestSuccess,
				Details: map[string]any{
					"populated_count": 2,
					"empty_count":     30,
					"transceivers": []any{
						map[string]any{
							"port":          "Ethernet1/1",
							"media_type":    "100GBASE-PSM4",
							"vendor_sn":     "XMN223000569",
							"channel":       "1",
							"temperature_c": float64(29.27),
							"voltage_v":     float64(3.34),
							"rx_power_dbm":  float64(-10.69),
							"tx_power_dbm":  float64(0.22),
							"tx_bias_ma":    float64(31.27),
							"status":        "ok",
						},
						map[string]any{
							"port":         "Ethernet2/1",
							"media_type":   "100GBASE-LR4",
							"vendor_sn":    "AAA111",
							"rx_power_dbm": float64(-25),
							"status":       "alarm",
						},
					},
				},
			},
		},
	}
	body, _ := RenderToBytes(r)
	s := string(body)
	for _, want := range []string{
		`class="icon optic"`,
		"100GBASE-PSM4",
		"XMN223000569",
		"29.3 °C",
		"3.34 V",
		"-10.69 dBm",
		"0.22 dBm",
		"31.3 mA",
		`class="pill success"`, // ok port
		`class="pill failure"`, // alarm port
		"<dt>Populated count</dt>",
		"<dd>2</dd>",
		"<dt>Empty count</dt>",
		"<dd>30</dd>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered HTML missing %q", want)
		}
	}
}

func TestRender_IssuesList(t *testing.T) {
	r := &Report{
		Started: time.Now(),
		Devices: []DeviceInfo{{Name: "tor1", Connected: true}},
		Results: []test.TestResult{
			{
				TestName:   "VerifyEnvironmentCooling",
				DeviceName: "tor1",
				Status:     test.TestFailure,
				Details: map[string]any{
					"issues": []any{"FanTraySlot/2/2/1: status failed"},
				},
			},
		},
	}
	body, _ := RenderToBytes(r)
	s := string(body)
	if !strings.Contains(s, `<ul class="issues">`) {
		t.Error("issues list should render with .issues class")
	}
	if !strings.Contains(s, "FanTraySlot/2/2/1: status failed") {
		t.Error("issue text should appear")
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
