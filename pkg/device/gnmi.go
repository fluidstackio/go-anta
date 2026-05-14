package device

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	gnmiapi "github.com/openconfig/gnmic/pkg/api"
	gnmitarget "github.com/openconfig/gnmic/pkg/api/target"

	"github.com/fluidstackio/go-anta/internal/logger"
)

// GNMIDevice implements the Device interface against an Arista EOS
// device using gNMI gRPC with origin=cli Get requests. The JSON shape
// inside the response matches eAPI exactly (after wrapper-stripping
// via unwrapCLIResponse), so tests written against EOSDevice work
// unchanged against GNMIDevice.
type GNMIDevice struct {
	BaseDevice
	target *gnmitarget.Target
	cache  *CommandCache
}

// NewGNMIDevice constructs a gNMI-backed device. Callers should prefer
// device.New(cfg) which handles default ports and dispatch.
func NewGNMIDevice(config DeviceConfig) *GNMIDevice {
	if config.Port == 0 {
		config.Port = 6030
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	dev := &GNMIDevice{
		BaseDevice: BaseDevice{
			Config: config,
			State:  ConnectionStateClosed,
		},
	}
	if !config.DisableCache {
		dev.cache = NewCommandCache(128, 60*time.Second)
	}
	return dev
}

func (d *GNMIDevice) Connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.State == ConnectionStateConnected || d.State == ConnectionStateEstablished {
		logger.Debugf("Device %s already connected", d.Config.Name)
		return nil
	}

	d.State = ConnectionStateConnecting

	addr := net.JoinHostPort(d.Config.Host, strconv.Itoa(d.Config.Port))
	opts := []gnmiapi.TargetOption{
		gnmiapi.Address(addr),
		gnmiapi.Username(d.Config.Username),
		gnmiapi.Password(d.Config.Password),
		gnmiapi.Timeout(d.Config.Timeout),
	}
	switch {
	case d.Config.Plaintext:
		// Plaintext gRPC: no TLS at all. Matches gnmic's --insecure flag.
		opts = append(opts, gnmiapi.Insecure(true))
	case d.Config.Insecure:
		// TLS but skip certificate verification. Matches eAPI's
		// InsecureSkipVerify semantics for this codebase.
		opts = append(opts, gnmiapi.SkipVerify(true))
	}

	target, err := gnmiapi.NewTarget(opts...)
	if err != nil {
		d.State = ConnectionStateError
		return fmt.Errorf("build gNMI target for %s: %w", d.Config.Name, err)
	}
	if err := target.CreateGNMIClient(ctx); err != nil {
		d.State = ConnectionStateError
		return fmt.Errorf("dial gNMI for %s: %w", d.Config.Name, err)
	}
	d.target = target
	d.State = ConnectionStateEstablished
	d.ConnectionTime = time.Now()

	// Probe with `show version` to populate HardwareModel. We call the
	// lockless executeOnTarget helper while still holding the write lock
	// — going through the public Execute (which takes RLock) would force
	// an unsafe unlock/relock dance around the probe. Probe failure is
	// non-fatal: the connection stays, Model just remains empty.
	result, probeErr := d.executeOnTarget(ctx, target, Command{Template: "show version", Format: "json"})
	if probeErr == nil {
		if m, ok := result.Output.(map[string]interface{}); ok {
			if model, ok := m["modelName"].(string); ok {
				d.Model = model
			}
		}
	} else {
		logger.Warnf("show version probe on %s failed: %v", d.Config.Name, probeErr)
	}

	logger.Infof("Successfully connected to %s via gNMI (Model: %s)", d.Config.Name, d.Model)
	return nil
}

func (d *GNMIDevice) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.State = ConnectionStateClosed
	if d.cache != nil {
		d.cache.Clear()
	}
	if d.target != nil {
		_ = d.target.Close()
		d.target = nil
	}
	return nil
}

// Execute issues a single gNMI CLI Get for cmd and returns the parsed result.
//
// End-to-end coverage lives in the env-var-gated integration smoke test in
// gnmi_integration_test.go (Task 7); a unit-level mock of gnmic's concrete
// Target type would require an interface abstraction not justified by any
// current production need.
func (d *GNMIDevice) Execute(ctx context.Context, cmd Command) (*CommandResult, error) {
	d.mu.RLock()
	if d.State != ConnectionStateEstablished {
		d.mu.RUnlock()
		return nil, fmt.Errorf("device %s is not connected", d.Config.Name)
	}
	target := d.target
	d.mu.RUnlock()

	if target == nil {
		// Disconnect raced with this Execute call between the RLock release
		// and the target capture. Surface a connect-flavored error rather
		// than a generic nil-target message.
		return nil, fmt.Errorf("device %s: disconnected during execute", d.Config.Name)
	}

	if cmd.UseCache && d.cache != nil {
		if cached := d.cache.Get(d.cacheKey(cmd)); cached != nil {
			cached.Cached = true
			return cached, nil
		}
	}

	result, err := d.executeOnTarget(ctx, target, cmd)
	if err != nil {
		return nil, err
	}

	if cmd.UseCache && d.cache != nil {
		d.cache.Set(d.cacheKey(cmd), result)
	}
	return result, nil
}

// executeOnTarget issues a single gNMI Get against the given target,
// without consulting d.mu, d.cache, or d.State. The caller is
// responsible for ensuring the target is non-nil and the device is in a
// usable state. Used by both Execute (which checks state + cache first)
// and Connect's show-version probe (which holds the write lock).
func (d *GNMIDevice) executeOnTarget(ctx context.Context, target *gnmitarget.Target, cmd Command) (*CommandResult, error) {
	start := time.Now()
	expanded := d.expandTemplate(cmd)

	// Both "" and "json" map to json_ietf; only "text" requests ASCII.
	// ExecuteBatch replicates this mapping — keep them in sync.
	encoding := "json_ietf"
	if cmd.Format == "text" {
		encoding = "ascii"
	}

	req, err := gnmiapi.NewGetRequest(
		gnmiapi.Path(fmt.Sprintf("cli:/%s", expanded)),
		gnmiapi.Encoding(encoding),
	)
	if err != nil {
		return nil, fmt.Errorf("build gNMI Get for %q: %w", expanded, err)
	}

	resp, err := target.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("gNMI Get for %q: %w", expanded, err)
	}

	output, err := extractCLIOutput(resp, expanded, encoding)
	if err != nil {
		return nil, fmt.Errorf("device %s: %w", d.Config.Name, err)
	}

	return &CommandResult{
		Command:   cmd,
		Output:    output,
		Duration:  time.Since(start),
		Timestamp: time.Now(),
	}, nil
}

// extractCLIOutput pulls the first Update value out of a gNMI GetResponse
// and converts it to a shape compatible with EOSDevice.Execute's Output:
//
//   - JSON_IETF -> map[string]interface{} with the command-name wrapper
//     stripped via unwrapCLIResponse.
//   - ASCII     -> string of the raw CLI output.
//
// JSON-less commands that still return ASCII even when JSON was requested
// fall back to the ASCII value rather than erroring.
func extractCLIOutput(resp *gnmipb.GetResponse, expanded, encoding string) (interface{}, error) {
	if resp == nil || len(resp.Notification) == 0 || len(resp.Notification[0].Update) == 0 {
		return nil, fmt.Errorf("gNMI Get for %q returned no notifications", expanded)
	}
	val := resp.Notification[0].Update[0].Val
	if val == nil {
		return nil, fmt.Errorf("gNMI Get for %q returned nil TypedValue", expanded)
	}
	return extractTypedValue(val, expanded, encoding)
}

// extractTypedValue is the value-only extractor reused by both Execute
// (single-path) and ExecuteBatch (multi-path) once Task 6 lands.
func extractTypedValue(val *gnmipb.TypedValue, expanded, encoding string) (interface{}, error) {
	switch encoding {
	case "ascii":
		// Empty output is a valid response (e.g. a command that ran successfully
		// and produced no text); return "" rather than erroring.
		if s := val.GetAsciiVal(); s != "" {
			return s, nil
		}
		return "", nil
	case "json_ietf":
		raw := val.GetJsonIetfVal()
		if len(raw) == 0 {
			// Some commands without a JSON form may still return ASCII;
			// surface that gracefully rather than failing.
			if s := val.GetAsciiVal(); s != "" {
				return s, nil
			}
			return nil, fmt.Errorf("gNMI returned empty JSON_IETF value for %q", expanded)
		}
		return unwrapCLIResponse(raw, expanded)
	default:
		return nil, fmt.Errorf("unsupported encoding %q", encoding)
	}
}

// expandTemplate substitutes {key} placeholders in cmd.Template with
// values from cmd.Params. Mirrors EOSDevice.expandTemplate so each
// transport file stays self-contained.
func (d *GNMIDevice) expandTemplate(cmd Command) string {
	cmdStr := cmd.Template
	for key, value := range cmd.Params {
		placeholder := fmt.Sprintf("{%s}", key)
		cmdStr = strings.ReplaceAll(cmdStr, placeholder, fmt.Sprint(value))
	}
	return cmdStr
}

// cacheKey builds the per-device cache key including all fields that
// affect the response (template, params via expansion, version, revision,
// format). Same shape as EOSDevice.cacheKey.
func (d *GNMIDevice) cacheKey(cmd Command) string {
	return fmt.Sprintf("%s|v=%s|r=%d|f=%s", d.expandTemplate(cmd), cmd.Version, cmd.Revision, cmd.Format)
}

// ExecuteBatch issues a single gNMI Get with one path per command,
// grouped by encoding when callers mix Format values inside a batch.
//
// End-to-end coverage of this path lives in the env-var-gated
// integration smoke test in gnmi_integration_test.go (added in Task 7
// of the gNMI transport plan); a unit-level mock of gnmic's concrete
// Target type would require an interface abstraction not justified by
// any current production need.
//
// One gNMI Get can carry multiple paths but only one encoding. If
// callers mix json and text formats inside a batch we split into one
// request per encoding. This mirrors the eAPI batch contract:
//   - Cached results bypass the network entirely.
//   - Each network-fetched result has Duration set (per-command share
//     of the batch wall time for that encoding group).
//   - If the response is shorter than expected (a prior command in the
//     batch errored on the device), the slot is filled with a
//     CommandResult whose Error is set — callers never see nil slots.
func (d *GNMIDevice) ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error) {
	d.mu.RLock()
	if d.State != ConnectionStateEstablished {
		d.mu.RUnlock()
		return nil, fmt.Errorf("device %s is not connected", d.Config.Name)
	}
	target := d.target
	d.mu.RUnlock()

	if target == nil {
		return nil, fmt.Errorf("device %s has no active gNMI target", d.Config.Name)
	}

	results := make([]*CommandResult, len(cmds))

	// pending tracks commands that need to hit the wire, with their
	// original index so we can map responses back into results in order.
	type pending struct {
		index    int
		cmd      Command
		expanded string
		encoding string
	}
	var pendings []pending

	for i, cmd := range cmds {
		if cmd.UseCache && d.cache != nil {
			if cached := d.cache.Get(d.cacheKey(cmd)); cached != nil {
				cached.Cached = true
				results[i] = cached
				continue
			}
		}
		expanded := d.expandTemplate(cmd)
		// Both "" and "json" map to json_ietf; only "text" requests ASCII.
		// Keep this in sync with Execute's identical block.
		encoding := "json_ietf"
		if cmd.Format == "text" {
			encoding = "ascii"
		}
		pendings = append(pendings, pending{
			index:    i,
			cmd:      cmd,
			expanded: expanded,
			encoding: encoding,
		})
	}

	if len(pendings) == 0 {
		return results, nil
	}

	// gNMI Get takes one encoding per request, but supports multiple
	// paths. If callers mix json/text we split into one request per
	// encoding.
	byEncoding := map[string][]pending{}
	for _, p := range pendings {
		byEncoding[p.encoding] = append(byEncoding[p.encoding], p)
	}

	// Sort keys for deterministic dispatch order — output is already
	// in input order via p.index, but consistent network ordering keeps
	// timestamps and per-cmd duration spread reproducible.
	encodings := make([]string, 0, len(byEncoding))
	for enc := range byEncoding {
		encodings = append(encodings, enc)
	}
	sort.Strings(encodings)

	for _, encoding := range encodings {
		group := byEncoding[encoding]
		opts := []gnmiapi.GNMIOption{gnmiapi.Encoding(encoding)}
		for _, p := range group {
			opts = append(opts, gnmiapi.Path(fmt.Sprintf("cli:/%s", p.expanded)))
		}
		req, err := gnmiapi.NewGetRequest(opts...)
		if err != nil {
			return nil, fmt.Errorf("build gNMI batch Get (%s): %w", encoding, err)
		}

		start := time.Now()
		resp, err := target.Get(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("device %s: gNMI batch Get (%s): %w", d.Config.Name, encoding, err)
		}
		perCmd := time.Since(start) / time.Duration(len(group))

		// Response notifications are in the same order as requested
		// paths (Arista behavior). Map back to input indexes.
		for i, p := range group {
			result := &CommandResult{
				Command:   p.cmd,
				Duration:  perCmd,
				Timestamp: time.Now(),
			}

			if i >= len(resp.Notification) || len(resp.Notification[i].Update) == 0 {
				// Short response: a prior command in the batch errored
				// and stopped further processing. Fill the slot with an
				// error rather than leaving it nil so callers don't
				// nil-deref downstream.
				result.Error = fmt.Errorf("device %s: gNMI batch returned no response for %q", d.Config.Name, p.expanded)
				results[p.index] = result
				continue
			}

			val := resp.Notification[i].Update[0].Val
			if val == nil {
				result.Error = fmt.Errorf("device %s: gNMI batch returned nil TypedValue for %q", d.Config.Name, p.expanded)
				results[p.index] = result
				continue
			}

			output, err := extractTypedValue(val, p.expanded, p.encoding)
			if err != nil {
				result.Error = fmt.Errorf("device %s: %w", d.Config.Name, err)
				results[p.index] = result
				continue
			}
			result.Output = output
			results[p.index] = result

			if p.cmd.UseCache && d.cache != nil {
				d.cache.Set(d.cacheKey(p.cmd), result)
			}
		}
	}

	return results, nil
}

// Refresh re-probes the device with show version and refreshes the
// HardwareModel and LastRefresh timestamp. Symmetric with EOSDevice.Refresh.
func (d *GNMIDevice) Refresh(ctx context.Context) error {
	result, err := d.Execute(ctx, Command{Template: "show version", Format: "json"})
	if err != nil {
		return fmt.Errorf("device %s: refresh failed: %w", d.Config.Name, err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if m, ok := result.Output.(map[string]interface{}); ok {
		if model, ok := m["modelName"].(string); ok {
			d.Model = model
		}
	}
	d.LastRefresh = time.Now()
	return nil
}
