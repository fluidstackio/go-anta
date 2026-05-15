package hardware

import (
	"strings"
	"testing"
)

// Tests live at the unit level on the parsing path; the device call is
// not exercised here. We build a synthetic coolingData map shaped like
// the EOS response and run it through the same logic the Execute path
// uses (the walkContainer/fanRecord helpers), then assert what would
// land in TestResult.Details.

func TestCoolingParse_ListShape(t *testing.T) {
	// EOS on the 7060DX4 / 7280 returns fanTraySlots as a LIST of
	// entries, each with its own `label`. The original code assumed a
	// map and walked zero fans on these platforms.
	cooling := map[string]any{
		"airflowDirection":   "frontToBackAirflow",
		"ambientTemperature": 21.625,
		"coolingMode":        "automatic",
		"fanTraySlots": []any{
			map[string]any{
				"label":  "1",
				"status": "ok",
				"fans": []any{
					map[string]any{
						"label":           "1/1",
						"status":          "ok",
						"actualSpeed":     float64(29),
						"configuredSpeed": float64(30),
					},
				},
			},
			map[string]any{
				"label":  "2",
				"status": "ok",
				"fans": []any{
					map[string]any{
						"label":           "2/1",
						"status":          "failed",
						"actualSpeed":     float64(0),
						"configuredSpeed": float64(30),
					},
				},
			},
		},
	}

	var fans []FanReport
	var issues []string
	t1 := &VerifyEnvironmentCooling{}
	walkContainer(cooling["fanTraySlots"], func(name string, tray map[string]any) {
		t1.collectContainerFans("FanTraySlot/"+name, tray, &fans, &issues)
	})

	if len(fans) != 2 {
		t.Errorf("expected 2 fans walked from list-shaped fanTraySlots, got %d", len(fans))
	}
	if len(issues) != 1 || !strings.Contains(issues[0], "failed") {
		t.Errorf("expected one issue naming the failed fan, got %v", issues)
	}
	if fans[0].Container != "FanTraySlot/1" {
		t.Errorf("expected container FanTraySlot/1, got %q", fans[0].Container)
	}
	if fans[0].ActualSpeedPct != 29 || fans[0].ConfiguredPct != 30 {
		t.Errorf("speed not captured: %+v", fans[0])
	}
}

func TestCoolingParse_MapShape(t *testing.T) {
	// Older EOS / other platforms can return fanTraySlots keyed by
	// slot name. Both shapes must work.
	cooling := map[string]any{
		"fanTraySlots": map[string]any{
			"FanTray1": map[string]any{
				"status": "ok",
				"fans": map[string]any{
					"Fan1/1": map[string]any{
						"status":      "ok",
						"actualSpeed": float64(35),
					},
				},
			},
		},
	}
	var fans []FanReport
	var issues []string
	t1 := &VerifyEnvironmentCooling{}
	walkContainer(cooling["fanTraySlots"], func(name string, tray map[string]any) {
		t1.collectContainerFans("FanTraySlot/"+name, tray, &fans, &issues)
	})
	if len(fans) != 1 {
		t.Fatalf("expected 1 fan walked, got %d", len(fans))
	}
	if len(issues) != 0 {
		t.Errorf("expected no issues, got %v", issues)
	}
	if fans[0].Status != "ok" {
		t.Errorf("status should be 'ok', got %q", fans[0].Status)
	}
}
