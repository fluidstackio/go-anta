package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/platform"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyTransceivers verifies the health and compliance of optical transceivers.
//
// This test performs the following checks for each transceiver:
//  1. Validates manufacturer compliance against an approved list if specified.
//  2. Monitors transceiver temperature against high alarm and warning thresholds.
//  3. Monitors supply voltage against high and low alarm thresholds.
//  4. Checks receive power levels for adequate signal strength.
//
// Expected Results:
//   - Success: All transceivers pass health checks and compliance requirements.
//   - Failure: Transceivers exceed temperature/voltage thresholds, have low RX power, or use unauthorized manufacturers.
//   - Error: Unable to retrieve transceiver inventory or health data.
//
// Example YAML configuration:
//   - name: "VerifyTransceivers"
//     module: "hardware"
//     inputs:
//     check_manufacturer: true
//     manufacturers: ["Arista Networks", "Finisar"]
//     check_temperature: true
//     check_voltage: true
type VerifyTransceivers struct {
	test.BaseTest
	CheckManufacturer bool     `yaml:"check_manufacturer" json:"check_manufacturer"`
	Manufacturers     []string `yaml:"manufacturers,omitempty" json:"manufacturers,omitempty"`
	CheckTemperature  bool     `yaml:"check_temperature,omitempty" json:"check_temperature,omitempty"`
	CheckVoltage      bool     `yaml:"check_voltage,omitempty" json:"check_voltage,omitempty"`
}

func NewVerifyTransceivers(inputs map[string]any) (test.Test, error) {
	t := &VerifyTransceivers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTransceivers",
			TestDescription: "Verify transceiver inventory and health",
			TestCategories:  []string{"hardware", "optics"},
		},
		CheckManufacturer: true,
		CheckTemperature:  true,
		CheckVoltage:      true,
	}

	if inputs != nil {
		if check, ok := inputs["check_manufacturer"].(bool); ok {
			t.CheckManufacturer = check
		}
		if manufacturers, ok := inputs["manufacturers"].([]any); ok {
			for _, m := range manufacturers {
				if mfg, ok := m.(string); ok {
					t.Manufacturers = append(t.Manufacturers, mfg)
				}
			}
		}
		if check, ok := inputs["check_temperature"].(bool); ok {
			t.CheckTemperature = check
		}
		if check, ok := inputs["check_voltage"].(bool); ok {
			t.CheckVoltage = check
		}
	}

	return t, nil
}

// TransceiverReport is the structured record of one populated optic,
// surfaced in TestResult.Details so the HTML reporter renders a typed
// table. JSON keys mirror what pkg/reporter's transceivers block reads.
type TransceiverReport struct {
	Port         string  `json:"port"`
	Slot         string  `json:"slot,omitempty"`
	MediaType    string  `json:"media_type,omitempty"`
	VendorName   string  `json:"vendor_name,omitempty"`
	VendorSN     string  `json:"vendor_sn,omitempty"`
	Channel      string  `json:"channel,omitempty"`
	TemperatureC float64 `json:"temperature_c,omitempty"`
	VoltageV     float64 `json:"voltage_v,omitempty"`
	RxPowerDBM   float64 `json:"rx_power_dbm,omitempty"`
	TxPowerDBM   float64 `json:"tx_power_dbm,omitempty"`
	TxBiasMA     float64 `json:"tx_bias_ma,omitempty"`
	Status       string  `json:"status"` // "ok" | "warning" | "alarm"
}

// metricThresholds extracts the `{highAlarm, highWarn, lowAlarm,
// lowWarn}` quartet for one metric sub-object inside `details`.
// EOS uses this same shape for temperature / voltage / rxPower /
// txPower / txBias. Returns false on absent or malformed input so
// callers can skip the threshold check rather than alert on noise.
func metricThresholds(details map[string]any, key string) (highAlarm, highWarn, lowAlarm, lowWarn float64, ok bool) {
	sub, ok := details[key].(map[string]any)
	if !ok {
		return 0, 0, 0, 0, false
	}
	gf := func(k string) float64 {
		if v, ok := sub[k].(float64); ok {
			return v
		}
		return 0
	}
	return gf("highAlarm"), gf("highWarn"), gf("lowAlarm"), gf("lowWarn"), true
}

func (t *VerifyTransceivers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical transceivers are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical transceivers are not present"); skipResult != nil {
		return skipResult, nil
	}

	// Use the `detail` variant so the per-metric threshold sub-objects
	// (highAlarm/highWarn/lowAlarm/lowWarn) come back populated.
	cmd := device.Command{
		Template: "show interfaces transceiver detail",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get transceiver data: %v", err)
		return result, nil
	}

	transceiverData, err := test.AsMap(cmdResult.Output)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Unexpected transceiver output: %v", err)
		return result, nil
	}

	interfaces, ok := transceiverData["interfaces"].(map[string]any)
	if !ok {
		result.Status = test.TestError
		result.Message = "Transceiver output missing 'interfaces' field"
		return result, nil
	}

	var optics []TransceiverReport
	emptyCount := 0
	issues := []string{}

	for intfName, raw := range interfaces {
		intf, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		r := transceiverRecord(intfName, intf)
		// EOS emits a port entry for every cage even when nothing is
		// installed. Skip the empties so the report only shows real
		// populated optics; surface the count separately.
		if r.MediaType == "" && r.VendorSN == "" {
			emptyCount++
			continue
		}
		t.validateOptic(&r, intf, &issues)
		optics = append(optics, r)
	}

	details := map[string]any{
		"populated_count": len(optics),
		"empty_count":     emptyCount,
		"transceivers":    optics,
	}
	if hottest := hottestOptic(optics); hottest != nil {
		details["hottest_port"] = hottest.Port
		details["hottest_c"] = hottest.TemperatureC
	}
	if len(issues) > 0 {
		details["issues"] = issues
	}
	result.Details = details

	switch {
	case len(optics) == 0:
		result.Message = fmt.Sprintf("No transceivers installed (%d empty cages)", emptyCount)
	case len(issues) > 0:
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Transceiver issues: %v", issues)
	default:
		result.Message = fmt.Sprintf("%d transceivers populated, all Ok", len(optics))
	}

	return result, nil
}

// transceiverRecord pulls the flat per-port fields from EOS's response.
// The detail-thresholds sub-object is read separately by validateOptic.
func transceiverRecord(port string, intf map[string]any) TransceiverReport {
	r := TransceiverReport{Port: port, Status: "ok"}
	if v, ok := intf["slot"].(string); ok {
		r.Slot = v
	}
	if v, ok := intf["mediaType"].(string); ok {
		r.MediaType = v
	}
	if v, ok := intf["vendorName"].(string); ok {
		r.VendorName = v
	}
	if v, ok := intf["vendorSn"].(string); ok {
		r.VendorSN = v
	}
	if v, ok := intf["channel"].(string); ok {
		r.Channel = v
	}
	if v, ok := intf["temperature"].(float64); ok {
		r.TemperatureC = v
	}
	if v, ok := intf["voltage"].(float64); ok {
		r.VoltageV = v
	}
	if v, ok := intf["rxPower"].(float64); ok {
		r.RxPowerDBM = v
	}
	if v, ok := intf["txPower"].(float64); ok {
		r.TxPowerDBM = v
	}
	if v, ok := intf["txBias"].(float64); ok {
		r.TxBiasMA = v
	}
	return r
}

// validateOptic runs the configured threshold checks against EOS's
// per-metric threshold sub-objects. Updates r.Status so the reporter
// can colour the row, and appends a human-readable string to issues
// for the failure message.
func (t *VerifyTransceivers) validateOptic(r *TransceiverReport, intf map[string]any, issues *[]string) {
	if t.CheckManufacturer && len(t.Manufacturers) > 0 && r.VendorName != "" {
		valid := false
		for _, allowed := range t.Manufacturers {
			if strings.Contains(strings.ToLower(r.VendorName), strings.ToLower(allowed)) {
				valid = true
				break
			}
		}
		if !valid {
			*issues = append(*issues, fmt.Sprintf("%s: unauthorized manufacturer %q", r.Port, r.VendorName))
			r.Status = "alarm"
		}
	}

	details, ok := intf["details"].(map[string]any)
	if !ok {
		return
	}

	bumpStatus := func(level string) {
		// alarm > warning > ok; never downgrade.
		switch r.Status {
		case "alarm":
			return
		case "warning":
			if level == "alarm" {
				r.Status = level
			}
		default:
			r.Status = level
		}
	}

	checkMetric := func(name, key, unit string, value float64, format string) {
		if value == 0 {
			return
		}
		hA, hW, lA, lW, ok := metricThresholds(details, key)
		if !ok {
			return
		}
		switch {
		case hA > 0 && value >= hA:
			*issues = append(*issues, fmt.Sprintf("%s: "+format+" ≥ %s high-alarm "+format, r.Port, value, unit, name, hA, unit))
			bumpStatus("alarm")
		case lA != 0 && value <= lA:
			*issues = append(*issues, fmt.Sprintf("%s: "+format+" ≤ %s low-alarm "+format, r.Port, value, unit, name, lA, unit))
			bumpStatus("alarm")
		case hW > 0 && value >= hW:
			*issues = append(*issues, fmt.Sprintf("%s: "+format+" ≥ %s high-warn "+format, r.Port, value, unit, name, hW, unit))
			bumpStatus("warning")
		case lW != 0 && value <= lW:
			*issues = append(*issues, fmt.Sprintf("%s: "+format+" ≤ %s low-warn "+format, r.Port, value, unit, name, lW, unit))
			bumpStatus("warning")
		}
	}

	if t.CheckTemperature {
		checkMetric("temperature", "temperature", "°C", r.TemperatureC, "%.1f%s")
	}
	if t.CheckVoltage {
		checkMetric("voltage", "voltage", "V", r.VoltageV, "%.2f%s")
	}
	// RX/TX power and bias get checked unconditionally — they're the
	// most useful signals for a flapping optic and have no separate
	// opt-in flag in the existing schema.
	checkMetric("rxPower", "rxPower", "dBm", r.RxPowerDBM, "%.2f %s")
	checkMetric("txPower", "txPower", "dBm", r.TxPowerDBM, "%.2f %s")
	checkMetric("txBias", "txBias", "mA", r.TxBiasMA, "%.1f %s")
}

func hottestOptic(optics []TransceiverReport) *TransceiverReport {
	if len(optics) == 0 {
		return nil
	}
	idx := 0
	for i := 1; i < len(optics); i++ {
		if optics[i].TemperatureC > optics[idx].TemperatureC {
			idx = i
		}
	}
	return &optics[idx]
}

func (t *VerifyTransceivers) ValidateInput(input any) error {
	return nil
}

// VerifyTransceiversManufacturers verifies if all transceivers come from approved manufacturers.
//
// This test validates that all optical transceivers in the device are sourced from
// manufacturers on an approved list, ensuring compliance with organizational standards
// and avoiding potential compatibility or support issues.
//
// The test performs the following checks:
//  1. Retrieves the complete transceiver inventory from the device.
//  2. Extracts manufacturer information for each transceiver.
//  3. Validates each manufacturer against the approved list.
//  4. Reports any transceivers from unauthorized manufacturers.
//
// Expected Results:
//   - Success: All transceivers are from approved manufacturers.
//   - Failure: One or more transceivers are from unauthorized manufacturers.
//   - Error: Unable to retrieve transceiver inventory or manufacturer data.
//
// Examples:
//
//   - name: VerifyTransceiversManufacturers strict validation
//     VerifyTransceiversManufacturers:
//     manufacturers: ["Arista Networks", "Finisar", "JDSU"]
//
//   - name: VerifyTransceiversManufacturers with additional vendors
//     VerifyTransceiversManufacturers:
//     manufacturers: ["Arista Networks", "Finisar", "JDSU", "Mellanox", "Intel"]
type VerifyTransceiversManufacturers struct {
	test.BaseTest
	Manufacturers []string `yaml:"manufacturers" json:"manufacturers"`
}

func NewVerifyTransceiversManufacturers(inputs map[string]any) (test.Test, error) {
	t := &VerifyTransceiversManufacturers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTransceiversManufacturers",
			TestDescription: "Verify all transceivers come from approved manufacturers",
			TestCategories:  []string{"hardware", "optics"},
		},
	}

	if inputs != nil {
		if manufacturers, ok := inputs["manufacturers"].([]any); ok {
			for _, mfg := range manufacturers {
				if mfgStr, ok := mfg.(string); ok {
					t.Manufacturers = append(t.Manufacturers, mfgStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyTransceiversManufacturers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical transceivers are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical transceivers are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show interfaces transceiver",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get transceiver data: %v", err)
		return result, nil
	}

	unauthorizedTransceivers := []string{}

	if transceiverData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := transceiverData["interfaces"].(map[string]any); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]any); ok {
					if mfgInfo, ok := intf["vendorName"].(string); ok {
						if !t.isApprovedManufacturer(mfgInfo) {
							unauthorizedTransceivers = append(unauthorizedTransceivers, fmt.Sprintf("%s: %s", intfName, mfgInfo))
						}
					} else if mfgInfo, ok := intf["manufacturer"].(string); ok {
						if !t.isApprovedManufacturer(mfgInfo) {
							unauthorizedTransceivers = append(unauthorizedTransceivers, fmt.Sprintf("%s: %s", intfName, mfgInfo))
						}
					}
				}
			}
		}
	}

	if len(unauthorizedTransceivers) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Unauthorized transceiver manufacturers found: %v", unauthorizedTransceivers)
	}

	return result, nil
}

func (t *VerifyTransceiversManufacturers) isApprovedManufacturer(manufacturer string) bool {
	for _, approved := range t.Manufacturers {
		if strings.EqualFold(manufacturer, approved) {
			return true
		}
	}
	return false
}

func (t *VerifyTransceiversManufacturers) ValidateInput(input any) error {
	if len(t.Manufacturers) == 0 {
		return fmt.Errorf("at least one approved manufacturer must be specified")
	}
	return nil
}

// VerifyTransceiversTemperature verifies if all transceivers are operating at acceptable temperatures.
//
// This test monitors transceiver temperature sensors to ensure they are operating within
// safe thermal limits. High temperatures can indicate poor ventilation, component aging,
// or environmental issues that could lead to performance degradation or failure.
//
// The test performs the following checks:
//  1. Retrieves temperature data for all transceivers.
//  2. Compares current temperatures against high alarm and warning thresholds.
//  3. Applies configurable margin to warning thresholds for early detection.
//  4. Reports transceivers approaching or exceeding thermal limits.
//
// Expected Results:
//   - Success: All transceivers are operating within acceptable temperature ranges.
//   - Failure: One or more transceivers exceed temperature thresholds or approach thermal limits.
//   - Error: Unable to retrieve transceiver temperature data.
//
// Examples:
//
//   - name: VerifyTransceiversTemperature standard check
//     VerifyTransceiversTemperature: {}
//
//   - name: VerifyTransceiversTemperature with custom margin
//     VerifyTransceiversTemperature:
//     temp_warning_margin: 10.0  # degrees Celsius margin before warning threshold
type VerifyTransceiversTemperature struct {
	test.BaseTest
	TempWarningMargin float64 `yaml:"temp_warning_margin,omitempty" json:"temp_warning_margin,omitempty"`
}

func NewVerifyTransceiversTemperature(inputs map[string]any) (test.Test, error) {
	t := &VerifyTransceiversTemperature{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTransceiversTemperature",
			TestDescription: "Verify all transceivers are operating at acceptable temperatures",
			TestCategories:  []string{"hardware", "optics", "environmental"},
		},
		TempWarningMargin: 5.0, // Default 5°C margin
	}

	if inputs != nil {
		if margin, ok := inputs["temp_warning_margin"].(float64); ok {
			t.TempWarningMargin = margin
		}
	}

	return t, nil
}

func (t *VerifyTransceiversTemperature) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical transceivers are not present
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical transceivers are not present"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show interfaces transceiver temperature",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get transceiver temperature data: %v", err)
		return result, nil
	}

	temperatureIssues := []string{}

	if tempData, ok := cmdResult.Output.(map[string]any); ok {
		if interfaces, ok := tempData["interfaces"].(map[string]any); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]any); ok {
					if err := t.checkTransceiverTemperature(intfName, intf, &temperatureIssues); err != nil {
						temperatureIssues = append(temperatureIssues, fmt.Sprintf("%s: %v", intfName, err))
					}
				}
			}
		}
	}

	if len(temperatureIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Transceiver temperature issues: %v", temperatureIssues)
	}

	return result, nil
}

func (t *VerifyTransceiversTemperature) checkTransceiverTemperature(intfName string, data map[string]any, issues *[]string) error {
	// Try different possible field names for temperature data
	tempFields := []string{"temperature", "temp", "currentTemp"}
	alarmFields := []string{"highAlarmThreshold", "tempHighAlarm", "highAlarm"}
	warningFields := []string{"highWarningThreshold", "tempHighWarning", "highWarning"}

	var currentTemp, highAlarm, highWarning float64
	var tempFound, alarmFound, warningFound bool

	// Extract current temperature
	for _, field := range tempFields {
		if temp, ok := data[field].(float64); ok {
			currentTemp = temp
			tempFound = true
			break
		}
	}

	// Extract high alarm threshold
	for _, field := range alarmFields {
		if alarm, ok := data[field].(float64); ok {
			highAlarm = alarm
			alarmFound = true
			break
		}
	}

	// Extract high warning threshold
	for _, field := range warningFields {
		if warning, ok := data[field].(float64); ok {
			highWarning = warning
			warningFound = true
			break
		}
	}

	if !tempFound {
		return fmt.Errorf("temperature data not available")
	}

	// Check against high alarm threshold
	if alarmFound && currentTemp >= highAlarm {
		*issues = append(*issues, fmt.Sprintf("%s: temperature %.1f°C exceeds high alarm threshold %.1f°C", intfName, currentTemp, highAlarm))
	}

	// Check against high warning threshold with margin
	if warningFound {
		warningWithMargin := highWarning - t.TempWarningMargin
		if currentTemp >= warningWithMargin {
			*issues = append(*issues, fmt.Sprintf("%s: temperature %.1f°C approaching warning threshold %.1f°C (margin: %.1f°C)", intfName, currentTemp, highWarning, t.TempWarningMargin))
		}
	}

	return nil
}

func (t *VerifyTransceiversTemperature) ValidateInput(input any) error {
	if t.TempWarningMargin < 0 {
		return fmt.Errorf("temperature warning margin cannot be negative")
	}
	return nil
}
