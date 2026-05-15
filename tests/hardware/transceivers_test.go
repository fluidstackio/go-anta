package hardware

import (
	"strings"
	"testing"
)

func sampleOptic() map[string]any {
	return map[string]any{
		"channel":     "1",
		"mediaType":   "100GBASE-PSM4",
		"vendorName":  "Arista Networks",
		"vendorSn":    "XMN223000569",
		"slot":        "Ethernet1",
		"temperature": float64(29.27),
		"voltage":     float64(3.34),
		"rxPower":     float64(-10.69),
		"txPower":     float64(0.22),
		"txBias":      float64(31.27),
		"details": map[string]any{
			"temperature": map[string]any{
				"highAlarm": float64(80), "highWarn": float64(75),
				"lowAlarm": float64(-15), "lowWarn": float64(-10),
			},
			"voltage": map[string]any{
				"highAlarm": float64(3.6), "highWarn": float64(3.5),
				"lowAlarm": float64(3.0), "lowWarn": float64(3.1),
			},
			"rxPower": map[string]any{
				"highAlarm": float64(4.0), "highWarn": float64(3.0),
				"lowAlarm": float64(-14.7), "lowWarn": float64(-13.7),
			},
			"txPower": map[string]any{
				"highAlarm": float64(4.0), "highWarn": float64(3.0),
				"lowAlarm": float64(-6.5), "lowWarn": float64(-5.5),
			},
			"txBias": map[string]any{
				"highAlarm": float64(100), "highWarn": float64(95),
				"lowAlarm": float64(6), "lowWarn": float64(6.5),
			},
		},
	}
}

func TestTransceiverParse_HappyPath(t *testing.T) {
	r := transceiverRecord("Ethernet1/1", sampleOptic())
	if r.MediaType != "100GBASE-PSM4" {
		t.Errorf("MediaType: %q", r.MediaType)
	}
	if r.VendorSN != "XMN223000569" {
		t.Errorf("VendorSN: %q", r.VendorSN)
	}
	if r.Channel != "1" {
		t.Errorf("Channel: %q", r.Channel)
	}
	if r.TemperatureC != 29.27 {
		t.Errorf("TemperatureC: %v", r.TemperatureC)
	}
	if r.RxPowerDBM != -10.69 {
		t.Errorf("RxPowerDBM: %v", r.RxPowerDBM)
	}
	if r.Status != "ok" {
		t.Errorf("expected default ok status, got %q", r.Status)
	}
}

func TestTransceiverValidate_RxBelowLowAlarmIsAlarm(t *testing.T) {
	intf := sampleOptic()
	intf["rxPower"] = float64(-20) // far below the -14.7 dBm low-alarm
	r := transceiverRecord("Ethernet1/1", intf)
	tt := &VerifyTransceivers{}
	var issues []string
	tt.validateOptic(&r, intf, &issues)
	if r.Status != "alarm" {
		t.Errorf("expected status=alarm for rxPower well below lowAlarm, got %q", r.Status)
	}
	if len(issues) == 0 || !strings.Contains(issues[0], "rxPower") {
		t.Errorf("expected an rxPower issue, got %v", issues)
	}
}

func TestTransceiverValidate_TempBetweenWarnAndAlarmIsWarning(t *testing.T) {
	intf := sampleOptic()
	intf["temperature"] = float64(77) // between highWarn=75 and highAlarm=80
	r := transceiverRecord("Ethernet1/1", intf)
	tt := &VerifyTransceivers{CheckTemperature: true}
	var issues []string
	tt.validateOptic(&r, intf, &issues)
	if r.Status != "warning" {
		t.Errorf("expected warning status between warn and alarm, got %q", r.Status)
	}
}

func TestTransceiverValidate_ManufacturerCheck(t *testing.T) {
	intf := sampleOptic()
	intf["vendorName"] = "Generic Optics Co"
	r := transceiverRecord("Ethernet1/1", intf)
	tt := &VerifyTransceivers{
		CheckManufacturer: true,
		Manufacturers:     []string{"Arista", "Finisar"},
	}
	var issues []string
	tt.validateOptic(&r, intf, &issues)
	if r.Status != "alarm" {
		t.Errorf("unauthorized manufacturer should alarm, got %q", r.Status)
	}
	if len(issues) == 0 || !strings.Contains(issues[0], "unauthorized") {
		t.Errorf("expected unauthorized-manufacturer issue, got %v", issues)
	}
}

func TestTransceiverParse_EmptyCageIsSkipped(t *testing.T) {
	// The Execute path filters empty cages — those with no mediaType
	// and no vendorSn. Confirm the record builder returns the empty
	// values so the caller knows to skip.
	empty := map[string]any{
		"channel": "1",
		"slot":    "Ethernet32",
	}
	r := transceiverRecord("Ethernet32/1", empty)
	if r.MediaType != "" || r.VendorSN != "" {
		t.Errorf("expected empty MediaType/VendorSN for unpopulated cage, got %+v", r)
	}
}

func TestMetricThresholds_AbsentReturnsOkFalse(t *testing.T) {
	_, _, _, _, ok := metricThresholds(map[string]any{}, "voltage")
	if ok {
		t.Error("expected ok=false when sub-object is absent")
	}
}
