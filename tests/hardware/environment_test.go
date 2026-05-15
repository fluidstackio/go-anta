package hardware

import (
	"strings"
	"testing"
)

func TestPowerSupplyParse_RealEOSShape(t *testing.T) {
	// Shape captured from `show system environment power` on
	// wdl101-fis-fm1-r1 — powerSupplies is a map keyed by slot.
	powerData := map[string]any{
		"powerSupplies": map[string]any{
			"1": map[string]any{
				"state":         "ok",
				"modelName":     "PWR-3001-AC-RED",
				"inputVoltage":  float64(207.5),
				"outputPower":   float64(276.5),
				"outputVoltage": float64(11.98),
				"capacity":      float64(3000),
			},
			"2": map[string]any{
				"state":        "failed",
				"modelName":    "PWR-3001-AC-RED",
				"inputVoltage": float64(0),
				"outputPower":  float64(0),
				"capacity":     float64(3000),
			},
		},
	}
	var psus []PSUReport
	walkContainer(powerData["powerSupplies"], func(name string, ps map[string]any) {
		psus = append(psus, psuRecord(name, ps))
	})
	if len(psus) != 2 {
		t.Fatalf("expected 2 PSUs, got %d", len(psus))
	}
	bySlot := map[string]PSUReport{}
	for _, p := range psus {
		bySlot[p.Slot] = p
	}
	if bySlot["1"].State != "ok" {
		t.Errorf("PSU 1 state: got %q want ok", bySlot["1"].State)
	}
	if bySlot["1"].Model != "PWR-3001-AC-RED" {
		t.Errorf("PSU 1 model not captured: %+v", bySlot["1"])
	}
	if bySlot["1"].CapacityW != 3000 {
		t.Errorf("PSU 1 capacity: got %v want 3000", bySlot["1"].CapacityW)
	}
	if bySlot["1"].OutputPowerW != 276.5 {
		t.Errorf("PSU 1 output power: got %v want 276.5", bySlot["1"].OutputPowerW)
	}
	if bySlot["2"].State != "failed" {
		t.Errorf("PSU 2 state: got %q want failed", bySlot["2"].State)
	}
}

func TestTempSensorParse_NestedAndTopLevel(t *testing.T) {
	// Real EOS shape: chassis sensors live at top-level `tempSensors`
	// while PSU-embedded ones are under powerSupplySlots[].tempSensors.
	// Both must contribute to the report.
	tempData := map[string]any{
		"systemStatus": "temperatureOk",
		"tempSensors": []any{
			map[string]any{
				"name":               "TempSensor1",
				"description":        "Local1",
				"currentTemperature": float64(24.8),
				"overheatThreshold":  float64(90),
				"criticalThreshold":  float64(100),
				"hwStatus":           "ok",
			},
		},
		"powerSupplySlots": []any{
			map[string]any{
				"relPos": "1",
				"tempSensors": []any{
					map[string]any{
						"name":               "TempSensorP1/1",
						"description":        "Ambient",
						"currentTemperature": float64(33),
						"overheatThreshold":  float64(65),
						"criticalThreshold":  float64(70),
						"hwStatus":           "ok",
					},
				},
			},
		},
	}

	var sensors []TempSensorReport
	walkContainer(tempData["tempSensors"], func(_ string, m map[string]any) {
		sensors = append(sensors, tempSensorRecord("chassis", m))
	})
	walkContainer(tempData["powerSupplySlots"], func(slotName string, slot map[string]any) {
		walkContainer(slot["tempSensors"], func(_ string, m map[string]any) {
			sensors = append(sensors, tempSensorRecord("PSU"+slotName, m))
		})
	})

	if len(sensors) != 2 {
		t.Fatalf("expected 2 sensors (1 chassis + 1 PSU), got %d", len(sensors))
	}
	byName := map[string]TempSensorReport{}
	for _, s := range sensors {
		byName[s.Name] = s
	}
	if byName["TempSensor1"].Container != "chassis" {
		t.Errorf("chassis sensor container: %+v", byName["TempSensor1"])
	}
	if byName["TempSensorP1/1"].Container != "PSU1" {
		t.Errorf("PSU sensor container: %+v", byName["TempSensorP1/1"])
	}
	if byName["TempSensorP1/1"].CriticalC != 70 {
		t.Errorf("PSU sensor critical threshold: %+v", byName["TempSensorP1/1"])
	}
}

func TestTempSensorParse_AlertThresholdLegacy(t *testing.T) {
	// Older EOS exposes `alertThreshold` instead of `criticalThreshold`.
	s := tempSensorRecord("chassis", map[string]any{
		"name":               "Old",
		"currentTemperature": float64(50),
		"alertThreshold":     float64(85),
		"hwStatus":           "ok",
	})
	if s.CriticalC != 85 {
		t.Errorf("alertThreshold should fall back into CriticalC, got %v", s.CriticalC)
	}
}

func TestTempSensorParse_FlagsCriticalAndOverheat(t *testing.T) {
	tt := &VerifyTemperature{FailureMargin: 0}
	hot := TempSensorReport{
		Name:      "T1",
		Container: "chassis",
		CurrentC:  100,
		CriticalC: 70,
		OverheatC: 65,
		Status:    "ok",
	}
	var warnings, alerts []string
	tt.checkSensor(hot, &warnings, &alerts)
	if len(alerts) == 0 {
		t.Errorf("expected critical alert, got %v / %v", alerts, warnings)
	}
}

func TestPowerSupplyParse_StatusFallback(t *testing.T) {
	// Some EOS variants use `status` instead of `state`; the record
	// helper falls back when state is missing.
	ps := map[string]any{
		"status":    "ok",
		"modelName": "PWR-500AC-F",
		"capacity":  float64(500),
	}
	r := psuRecord("PSU1", ps)
	if r.State != "ok" {
		t.Errorf("expected state from status fallback, got %q", r.State)
	}
}

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
