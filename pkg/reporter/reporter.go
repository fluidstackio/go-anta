// Package reporter renders a test run to HTML.
//
// The old multi-format reporter (csv/json/markdown/table) was nuked
// in favor of one rich, self-contained HTML report. The output is a
// single file that opens in any browser with no external assets — no
// CDN-loaded fonts, no JS framework, no separate stylesheet — so it
// works in air-gapped environments and can be emailed as an artifact.
package reporter

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/fluidstackio/go-anta/pkg/test"
)

// DeviceInfo carries per-device metadata the report header surfaces.
// The runner populates this once per device. Model and EOSVersion
// come from the live Device after Connect succeeds; host/transport/
// port/tags from the inventory. Connected is true iff the gRPC/HTTPS
// handshake (and any post-connect probe) completed.
type DeviceInfo struct {
	Name         string   `json:"name"`
	Host         string   `json:"host"`
	Transport    string   `json:"transport"`
	Port         int      `json:"port"`
	Model        string   `json:"model,omitempty"`
	EOSVersion   string   `json:"eos_version,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Connected    bool     `json:"connected"`
	ConnectError string   `json:"connect_error,omitempty"`
}

// Report is the full input to Render. Callers build it from the run
// inputs (devices + catalog) plus the slice of TestResult the runner
// returns.
type Report struct {
	Title     string            `json:"title,omitempty"`
	Started   time.Time         `json:"started"`
	Completed time.Time         `json:"completed"`
	Devices   []DeviceInfo      `json:"devices"`
	Results   []test.TestResult `json:"results"`
}

// Render writes a self-contained HTML report to w.
func Render(w io.Writer, r *Report) error {
	view := newReportView(r)
	return reportTemplate.ExecuteTemplate(w, "report", view)
}

// ----------------------------------------------------------------------
// Internal view model: server-side grouping of results by device, plus
// JSON-pretty-printed Details. Keeps the template logic-free.
// ----------------------------------------------------------------------

type reportView struct {
	Title     string
	Started   string
	Completed string
	Duration  string
	Totals    statsView
	Devices   []deviceView
}

type deviceView struct {
	Info     DeviceInfo
	HostPort string
	Status   string // "connected" | "disconnected"
	Tests    []testView
	Stats    statsView
}

type testView struct {
	Name       string
	Status     string // "success" | "failure" | "error" | "skipped" | "unset"
	StatusText string // "SUCCESS" | "FAILURE" | ...
	Message    string
	Categories []string
	Duration   string
	Blocks     []detailBlock // structured detail sections; rendered as tables/dls
	Details    string        // JSON fallback when Details isn't a recognised shape
}

// detailBlock is one rendered section under a test's Details. Kind
// selects the template path; only the matching fields are populated.
type detailBlock struct {
	Kind        string // "fans" | "psus" | "temps" | "optics" | "ifaceErrors" | "summary" | "issues" | "json"
	Title       string
	Fans        []fanRow
	PSUs        []psuRow
	Temps       []tempRow
	Optics      []opticRow
	IfaceErrors []ifaceErrorRow
	KV          []kvRow
	Items       []string // for "issues"
	JSON        string   // for "json"
}

type fanRow struct {
	Container      string
	Name           string
	Label          string
	Status         string
	StatusClass    string
	ActualSpeedPct int
	ConfiguredPct  int
}

type tempRow struct {
	Container   string
	Name        string
	Description string
	CurrentC    string // pre-formatted "32.5 °C"
	OverheatC   string // pre-formatted threshold or "—"
	CriticalC   string
	HeadroomC   string // overheat - current, "10.0 °C" or ""
	Status      string
	StatusClass string
	BarPct      int // 0..100 ratio of current/overheat
}

type psuRow struct {
	Slot        string
	Model       string
	State       string
	StatusClass string
	InputV      string
	OutputState string
	OutputPower string // formatted "287 W"
	Capacity    string // formatted "3000 W"
	LoadPct     string // formatted "9%" (output / capacity)
}

type opticRow struct {
	Port        string
	Media       string
	VendorSN    string
	VendorName  string
	Channel     string
	Temperature string // "29.3 °C"
	Voltage     string // "3.34 V"
	RxPower     string // "-10.7 dBm"
	TxPower     string
	TxBias      string
	Status      string
	StatusClass string
}

type ifaceErrorRow struct {
	Interface       string
	InErrors        int
	OutErrors       int
	FcsErrors       int
	AlignmentErrors int
	SymbolErrors    int
	FrameTooShorts  int
	FrameTooLongs   int
	Total           int
}

type kvRow struct {
	Label string
	Value string
}

type statsView struct {
	Total      int
	Success    int
	Failure    int
	Error      int
	Skipped    int
	SuccessPct string // pre-formatted ("100.0%")
}

func newReportView(r *Report) reportView {
	if r == nil {
		r = &Report{}
	}
	out := reportView{
		Title:     r.Title,
		Started:   r.Started.Format(time.RFC3339),
		Completed: r.Completed.Format(time.RFC3339),
		Duration:  r.Completed.Sub(r.Started).Truncate(time.Millisecond).String(),
	}
	if out.Title == "" {
		out.Title = "go-anta test run"
	}

	// Group results by device. Preserve inventory order for the
	// devices we know about; tack any "phantom" devices (results for
	// names not in r.Devices) onto the end.
	byDevice := map[string][]test.TestResult{}
	for _, res := range r.Results {
		byDevice[res.DeviceName] = append(byDevice[res.DeviceName], res)
	}
	seen := map[string]bool{}
	for _, d := range r.Devices {
		out.Devices = append(out.Devices, buildDeviceView(d, byDevice[d.Name]))
		seen[d.Name] = true
	}
	var orphans []string
	for name := range byDevice {
		if !seen[name] {
			orphans = append(orphans, name)
		}
	}
	sort.Strings(orphans)
	for _, name := range orphans {
		out.Devices = append(out.Devices, buildDeviceView(DeviceInfo{Name: name}, byDevice[name]))
	}

	// Roll device stats up to totals.
	for _, d := range out.Devices {
		out.Totals.Total += d.Stats.Total
		out.Totals.Success += d.Stats.Success
		out.Totals.Failure += d.Stats.Failure
		out.Totals.Error += d.Stats.Error
		out.Totals.Skipped += d.Stats.Skipped
	}
	out.Totals.SuccessPct = pct(out.Totals.Success, out.Totals.Total)
	return out
}

func buildDeviceView(info DeviceInfo, results []test.TestResult) deviceView {
	hostPort := info.Host
	if info.Port > 0 {
		hostPort = fmt.Sprintf("%s:%d", info.Host, info.Port)
	}
	dv := deviceView{
		Info:     info,
		HostPort: hostPort,
		Status:   "disconnected",
	}
	if info.Connected {
		dv.Status = "connected"
	}

	sort.SliceStable(results, func(i, j int) bool {
		// Failures + errors first so the eye lands on the problem;
		// then alphabetical within each tier for stable output.
		if statusRank(results[i].Status) != statusRank(results[j].Status) {
			return statusRank(results[i].Status) < statusRank(results[j].Status)
		}
		return results[i].TestName < results[j].TestName
	})

	for _, res := range results {
		dv.Tests = append(dv.Tests, buildTestView(res))
		switch res.Status {
		case test.TestSuccess:
			dv.Stats.Success++
		case test.TestFailure:
			dv.Stats.Failure++
		case test.TestError:
			dv.Stats.Error++
		case test.TestSkipped:
			dv.Stats.Skipped++
		}
		dv.Stats.Total++
	}
	dv.Stats.SuccessPct = pct(dv.Stats.Success, dv.Stats.Total)
	return dv
}

func buildTestView(res test.TestResult) testView {
	tv := testView{
		Name:       res.TestName,
		Status:     statusSlug(res.Status),
		StatusText: strings.ToUpper(statusSlug(res.Status)),
		Message:    res.Message,
		Categories: res.Categories,
		Duration:   res.Duration.Truncate(time.Millisecond).String(),
	}
	tv.Blocks, tv.Details = renderDetails(res.Details)
	return tv
}

// renderDetails inspects a TestResult.Details payload and extracts
// any recognised shapes (fans, power supplies, key-value summaries,
// issue lists) into structured blocks. Unknown content falls through
// to a pretty-printed JSON block so nothing is hidden.
//
// Recognised top-level keys when Details is a map:
//
//	fans            → []fanRow  → "fans" block, rendered as a table
//	power_supplies  → []psuRow  → "psus" block, rendered as a table
//	issues          → []string  → "issues" block, rendered as a list
//	any other scalar → kv pair → "summary" block, rendered as a dl
//
// Anything we can't make sense of (Details isn't a map, or contains
// nested structures we don't know) becomes a JSON block at the end.
func renderDetails(d any) (blocks []detailBlock, jsonFallback string) {
	if d == nil {
		return nil, ""
	}

	// Normalise via a JSON round-trip so we get map[string]any /
	// []any regardless of whether the test populated Details with a
	// typed struct or an ad-hoc map.
	raw, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Sprintf("%+v", d)
	}
	var top any
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, string(raw)
	}
	m, ok := top.(map[string]any)
	if !ok {
		// Non-object Details (a bare list or scalar) — JSON-dump it.
		b, _ := json.MarshalIndent(d, "", "  ")
		return nil, string(b)
	}

	consumed := map[string]bool{}

	if fans, ok := m["fans"].([]any); ok {
		blocks = append(blocks, fansBlock(fans))
		consumed["fans"] = true
	}
	if psus, ok := m["power_supplies"].([]any); ok {
		blocks = append(blocks, psusBlock(psus))
		consumed["power_supplies"] = true
	}
	if temps, ok := m["temperatures"].([]any); ok {
		blocks = append(blocks, tempsBlock(temps))
		consumed["temperatures"] = true
	}
	if optics, ok := m["transceivers"].([]any); ok {
		blocks = append(blocks, opticsBlock(optics))
		consumed["transceivers"] = true
	}
	if errs, ok := m["interface_errors"].([]any); ok {
		blocks = append(blocks, ifaceErrorsBlock(errs))
		consumed["interface_errors"] = true
	}
	if issues, ok := m["issues"].([]any); ok && len(issues) > 0 {
		var list []string
		for _, item := range issues {
			if s, ok := item.(string); ok {
				list = append(list, s)
			}
		}
		if len(list) > 0 {
			blocks = append(blocks, detailBlock{Kind: "issues", Title: "Issues", Items: list})
			consumed["issues"] = true
		}
	}

	// Everything else that's scalar becomes a summary key-value row.
	var kvs []kvRow
	keys := make([]string, 0, len(m))
	for k := range m {
		if !consumed[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var leftovers map[string]any
	for _, k := range keys {
		v := m[k]
		switch v.(type) {
		case string, float64, bool, int, int64, nil:
			kvs = append(kvs, kvRow{Label: humanize(k), Value: fmt.Sprintf("%v", v)})
		default:
			if leftovers == nil {
				leftovers = map[string]any{}
			}
			leftovers[k] = v
		}
	}
	if len(kvs) > 0 {
		blocks = append([]detailBlock{{Kind: "summary", Title: "Summary", KV: kvs}}, blocks...)
	}
	if len(leftovers) > 0 {
		b, _ := json.MarshalIndent(leftovers, "", "  ")
		blocks = append(blocks, detailBlock{Kind: "json", Title: "Other", JSON: string(b)})
	}
	return blocks, ""
}

func fansBlock(items []any) detailBlock {
	out := detailBlock{Kind: "fans", Title: "Fans"}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := fanRow{
			Container: strVal(m, "container"),
			Name:      strVal(m, "name"),
			Label:     strVal(m, "label"),
			Status:    strVal(m, "status"),
		}
		row.StatusClass = statusClassFor(row.Status)
		if v, ok := m["actual_speed_pct"].(float64); ok {
			row.ActualSpeedPct = int(v)
		}
		if v, ok := m["configured_pct"].(float64); ok {
			row.ConfiguredPct = int(v)
		}
		out.Fans = append(out.Fans, row)
	}
	return out
}

func psusBlock(items []any) detailBlock {
	out := detailBlock{Kind: "psus", Title: "Power supplies"}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := psuRow{
			Slot:        strVal(m, "slot"),
			Model:       strVal(m, "model"),
			State:       strVal(m, "state"),
			OutputState: strVal(m, "output_state"),
		}
		row.StatusClass = statusClassFor(row.State)
		if v, ok := m["input_voltage"].(float64); ok {
			row.InputV = fmt.Sprintf("%.1f V", v)
		}
		power, _ := m["output_power_w"].(float64)
		capacity, _ := m["capacity_w"].(float64)
		if power > 0 {
			row.OutputPower = fmt.Sprintf("%.0f W", power)
		}
		if capacity > 0 {
			row.Capacity = fmt.Sprintf("%.0f W", capacity)
			if power > 0 {
				row.LoadPct = fmt.Sprintf("%.0f%%", 100*power/capacity)
			}
		}
		out.PSUs = append(out.PSUs, row)
	}
	return out
}

func tempsBlock(items []any) detailBlock {
	out := detailBlock{Kind: "temps", Title: "Temperatures"}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := tempRow{
			Container:   strVal(m, "container"),
			Name:        strVal(m, "name"),
			Description: strVal(m, "description"),
		}
		cur, _ := m["current_c"].(float64)
		overheat, _ := m["overheat_c"].(float64)
		critical, _ := m["critical_c"].(float64)
		if cur > 0 {
			row.CurrentC = fmt.Sprintf("%.1f °C", cur)
		}
		if overheat > 0 {
			row.OverheatC = fmt.Sprintf("%.0f °C", overheat)
			if cur > 0 {
				row.HeadroomC = fmt.Sprintf("%.1f °C", overheat-cur)
				row.BarPct = int(100 * cur / overheat)
				if row.BarPct < 0 {
					row.BarPct = 0
				}
				if row.BarPct > 100 {
					row.BarPct = 100
				}
			}
		}
		if critical > 0 {
			row.CriticalC = fmt.Sprintf("%.0f °C", critical)
		}
		// Status priority:
		//   inAlert true OR hwStatus != ok → failure
		//   current ≥ overheat              → error (warning tier)
		//   otherwise                       → success
		hwStatus := strVal(m, "status")
		inAlert, _ := m["in_alert"].(bool)
		row.Status = hwStatus
		switch {
		case inAlert, hwStatus != "" && !strings.EqualFold(hwStatus, "ok"):
			row.StatusClass = "failure"
			if row.Status == "" {
				row.Status = "alert"
			}
		case overheat > 0 && cur >= overheat:
			row.StatusClass = "error"
			row.Status = "near overheat"
		default:
			row.StatusClass = "success"
			if row.Status == "" {
				row.Status = "ok"
			}
		}
		out.Temps = append(out.Temps, row)
	}
	return out
}

func ifaceErrorsBlock(items []any) detailBlock {
	out := detailBlock{Kind: "ifaceErrors", Title: "Interface error counters"}
	intField := func(m map[string]any, key string) int {
		if v, ok := m[key].(float64); ok {
			return int(v)
		}
		return 0
	}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := ifaceErrorRow{
			Interface:       strVal(m, "interface"),
			InErrors:        intField(m, "in_errors"),
			OutErrors:       intField(m, "out_errors"),
			FcsErrors:       intField(m, "fcs_errors"),
			AlignmentErrors: intField(m, "alignment_errors"),
			SymbolErrors:    intField(m, "symbol_errors"),
			FrameTooShorts:  intField(m, "frame_too_shorts"),
			FrameTooLongs:   intField(m, "frame_too_longs"),
		}
		row.Total = row.InErrors + row.OutErrors + row.FcsErrors + row.AlignmentErrors +
			row.SymbolErrors + row.FrameTooShorts + row.FrameTooLongs
		out.IfaceErrors = append(out.IfaceErrors, row)
	}
	return out
}

func opticsBlock(items []any) detailBlock {
	out := detailBlock{Kind: "optics", Title: "Transceivers"}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		row := opticRow{
			Port:       strVal(m, "port"),
			Media:      strVal(m, "media_type"),
			VendorSN:   strVal(m, "vendor_sn"),
			VendorName: strVal(m, "vendor_name"),
			Channel:    strVal(m, "channel"),
		}
		if v, ok := m["temperature_c"].(float64); ok && v != 0 {
			row.Temperature = fmt.Sprintf("%.1f °C", v)
		}
		if v, ok := m["voltage_v"].(float64); ok && v != 0 {
			row.Voltage = fmt.Sprintf("%.2f V", v)
		}
		if v, ok := m["rx_power_dbm"].(float64); ok && v != 0 {
			row.RxPower = fmt.Sprintf("%.2f dBm", v)
		}
		if v, ok := m["tx_power_dbm"].(float64); ok && v != 0 {
			row.TxPower = fmt.Sprintf("%.2f dBm", v)
		}
		if v, ok := m["tx_bias_ma"].(float64); ok && v != 0 {
			row.TxBias = fmt.Sprintf("%.1f mA", v)
		}
		row.Status = strVal(m, "status")
		switch row.Status {
		case "alarm":
			row.StatusClass = "failure"
		case "warning":
			row.StatusClass = "error"
		case "", "ok":
			row.StatusClass = "success"
			if row.Status == "" {
				row.Status = "ok"
			}
		default:
			row.StatusClass = ""
		}
		out.Optics = append(out.Optics, row)
	}
	return out
}

func strVal(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// statusClassFor maps EOS-flavoured status strings to a CSS class so
// row pills render in the right colour. Anything we don't recognise
// is treated as neutral (no class).
func statusClassFor(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ok", "running", "powerlossok", "powerlostok", "good":
		return "success"
	case "failed", "fail", "error":
		return "failure"
	case "":
		return ""
	default:
		// Anything that isn't an obvious ok/fail (e.g. "uninitialized")
		// is shown in the warning tier.
		return "error"
	}
}

// humanize turns snake_case keys into Title Case labels for the
// summary table. "ambient_temperature_c" → "Ambient temperature c".
func humanize(s string) string {
	parts := strings.Split(s, "_")
	if len(parts) == 0 {
		return s
	}
	parts[0] = strings.ToUpper(parts[0][:1]) + parts[0][1:]
	return strings.Join(parts, " ")
}

func statusSlug(s test.TestStatus) string {
	switch s {
	case test.TestSuccess:
		return "success"
	case test.TestFailure:
		return "failure"
	case test.TestError:
		return "error"
	case test.TestSkipped:
		return "skipped"
	default:
		return "unset"
	}
}

// statusRank sorts results so the most-actionable show first inside a
// device: errors > failures > successes > skipped.
func statusRank(s test.TestStatus) int {
	switch s {
	case test.TestError:
		return 0
	case test.TestFailure:
		return 1
	case test.TestSuccess:
		return 2
	case test.TestSkipped:
		return 3
	}
	return 4
}

func pct(n, total int) string {
	if total == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(n)/float64(total))
}

// ----------------------------------------------------------------------
// Embedded template
// ----------------------------------------------------------------------

//go:embed report.tmpl.html
var reportTemplateSrc string

var reportTemplate = template.Must(template.New("report").Parse(reportTemplateSrc))

// RenderToBytes is a convenience for callers that want the raw HTML
// (e.g., for tests). Equivalent to Render-into-a-buffer.
func RenderToBytes(r *Report) ([]byte, error) {
	var buf bytes.Buffer
	if err := Render(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
