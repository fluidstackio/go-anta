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
	Details    string // pre-formatted JSON, "" if no Details
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
	if res.Details != nil {
		if b, err := json.MarshalIndent(res.Details, "", "  "); err == nil {
			tv.Details = string(b)
		} else {
			tv.Details = fmt.Sprintf("%+v", res.Details)
		}
	}
	return tv
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
