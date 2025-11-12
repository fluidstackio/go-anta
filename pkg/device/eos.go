package device

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	
	"github.com/fluidstackio/go-anta/internal/logger"
)

type EOSDevice struct {
	BaseDevice
	client    *http.Client
	cache     *CommandCache
	mu        sync.RWMutex
	requestID int
}

func NewEOSDevice(config DeviceConfig) *EOSDevice {
	if config.Port == 0 {
		config.Port = 443
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	// Configure TLS for compatibility with Arista EOS devices
	// Many Arista devices use older TLS versions and cipher suites
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.Insecure,
		MinVersion:         tls.VersionTLS10, // Support older TLS versions for compatibility
		MaxVersion:         0,                 // Allow any TLS version (0 means use Go's default max)
		// Include legacy cipher suites for older EOS versions
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
	}

	tr := &http.Transport{
		TLSClientConfig:       tlsConfig,
		DisableKeepAlives:     true,  // Disable connection reuse
		DisableCompression:    true,  // Disable compression
		MaxIdleConns:          1,     // Limit idle connections
		MaxIdleConnsPerHost:   1,     // Limit idle connections per host
		IdleConnTimeout:       10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   config.Timeout,
	}

	device := &EOSDevice{
		BaseDevice: BaseDevice{
			Config: config,
			State:  ConnectionStateClosed,
		},
		client: client,
	}

	if !config.DisableCache {
		device.cache = NewCommandCache(128, 60*time.Second)
	}

	return device
}

func (d *EOSDevice) Connect(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.State == ConnectionStateConnected || d.State == ConnectionStateEstablished {
		logger.Debugf("Device %s already connected", d.Config.Name)
		return nil
	}

	logger.Infof("Connecting to device %s (%s:%d)", d.Config.Name, d.Config.Host, d.Config.Port)
	d.State = ConnectionStateConnecting

	testCmd := Command{
		Template: "show version",
		Format:   "json",
	}

	logger.Debugf("Sending test command to %s", d.Config.Name)
	result, err := d.executeCommand(ctx, testCmd)
	if err != nil {
		d.State = ConnectionStateError
		logger.Errorf("Failed to connect to %s: %v", d.Config.Name, err)
		return fmt.Errorf("failed to connect to %s: %w", d.Config.Host, err)
	}

	d.State = ConnectionStateConnected
	d.ConnectionTime = time.Now()

	if versionData, ok := result.Output.(map[string]interface{}); ok {
		if model, ok := versionData["modelName"].(string); ok {
			d.Model = model
		}
	}

	d.State = ConnectionStateEstablished
	logger.Infof("Successfully connected to %s (Model: %s)", d.Config.Name, d.Model)
	return nil
}

func (d *EOSDevice) Disconnect() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.State = ConnectionStateClosed
	if d.cache != nil {
		d.cache.Clear()
	}
	return nil
}

func (d *EOSDevice) Execute(ctx context.Context, cmd Command) (*CommandResult, error) {
	d.mu.RLock()
	if d.State != ConnectionStateEstablished {
		d.mu.RUnlock()
		return nil, fmt.Errorf("device %s is not connected", d.Config.Name)
	}
	d.mu.RUnlock()

	if cmd.UseCache && d.cache != nil {
		if cached := d.cache.Get(cmd.Template); cached != nil {
			cached.Cached = true
			return cached, nil
		}
	}

	result, err := d.executeCommand(ctx, cmd)
	if err != nil {
		return nil, err
	}

	if cmd.UseCache && d.cache != nil {
		d.cache.Set(cmd.Template, result)
	}

	return result, nil
}

func (d *EOSDevice) ExecuteBatch(ctx context.Context, cmds []Command) ([]*CommandResult, error) {
	d.mu.RLock()
	if d.State != ConnectionStateEstablished {
		d.mu.RUnlock()
		return nil, fmt.Errorf("device %s is not connected", d.Config.Name)
	}
	d.mu.RUnlock()

	results := make([]*CommandResult, len(cmds))
	commands := make([]map[string]interface{}, 0, len(cmds))

	for i, cmd := range cmds {
		if cmd.UseCache && d.cache != nil {
			if cached := d.cache.Get(cmd.Template); cached != nil {
				cached.Cached = true
				results[i] = cached
				continue
			}
		}

		cmdStr := d.expandTemplate(cmd)
		commands = append(commands, map[string]interface{}{
			"cmd":     cmdStr,
			"version": cmd.Version,
			"format":  cmd.Format,
		})
	}

	if len(commands) == 0 {
		return results, nil
	}

	batchResult, err := d.executeBatchCommands(ctx, commands)
	if err != nil {
		return nil, err
	}

	batchIdx := 0
	for i, cmd := range cmds {
		if results[i] != nil {
			continue
		}

		if batchIdx < len(batchResult) {
			result := &CommandResult{
				Command:   cmd,
				Output:    batchResult[batchIdx],
				Timestamp: time.Now(),
			}
			results[i] = result

			if cmd.UseCache && d.cache != nil {
				d.cache.Set(cmd.Template, result)
			}
			batchIdx++
		}
	}

	return results, nil
}

func (d *EOSDevice) Refresh(ctx context.Context) error {
	cmd := Command{
		Template: "show version",
		Format:   "json",
		UseCache: false,
	}

	result, err := d.Execute(ctx, cmd)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if versionData, ok := result.Output.(map[string]interface{}); ok {
		if model, ok := versionData["modelName"].(string); ok {
			d.Model = model
		}
	}

	d.LastRefresh = time.Now()
	return nil
}

func (d *EOSDevice) executeCommand(ctx context.Context, cmd Command) (*CommandResult, error) {
	cmdStr := d.expandTemplate(cmd)
	logger.Debugf("Executing command on %s: %s", d.Config.Name, cmdStr)
	
	logger.Debugf("Creating JSON-RPC payload for %s", d.Config.Name)
	// Use the simple format that works with curl
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "runCmds",
		"params": map[string]interface{}{
			"version": 1,
			"cmds":    []string{cmdStr}, // Use simple string array instead of objects
			"format":  "json",
		},
		"id": "1", // Use string ID instead of int
	}
	logger.Debugf("Payload created: %+v", payload)

	start := time.Now()
	logger.Debugf("About to call sendRequest for %s", d.Config.Name)
	logger.Debugf("Payload to be sent: %+v", payload)
	response, err := d.sendRequest(ctx, payload)
	duration := time.Since(start)
	logger.Debugf("sendRequest completed for %s in %v", d.Config.Name, duration)

	if err != nil {
		logger.Errorf("Command failed on %s: %v", d.Config.Name, err)
		return &CommandResult{
			Command:   cmd,
			Error:     err,
			Duration:  duration,
			Timestamp: time.Now(),
		}, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return &CommandResult{
			Command:   cmd,
			Error:     err,
			Duration:  duration,
			Timestamp: time.Now(),
		}, err
	}

	if errorData, ok := result["error"]; ok {
		err := fmt.Errorf("eAPI error: %v", errorData)
		return &CommandResult{
			Command:   cmd,
			Error:     err,
			Duration:  duration,
			Timestamp: time.Now(),
		}, err
	}

	if resultData, ok := result["result"].([]interface{}); ok && len(resultData) > 0 {
		return &CommandResult{
			Command:   cmd,
			Output:    resultData[0],
			Duration:  duration,
			Timestamp: time.Now(),
		}, nil
	}

	return &CommandResult{
		Command:   cmd,
		Output:    result["result"],
		Duration:  duration,
		Timestamp: time.Now(),
	}, nil
}

func (d *EOSDevice) executeBatchCommands(ctx context.Context, commands []map[string]interface{}) ([]interface{}, error) {
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "runCmds",
		"params": map[string]interface{}{
			"version": 1,
			"cmds":    commands,
			"format":  "json",
		},
		"id": d.getRequestID(),
	}

	response, err := d.sendRequest(ctx, payload)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, err
	}

	if errorData, ok := result["error"]; ok {
		return nil, fmt.Errorf("eAPI error: %v", errorData)
	}

	if resultData, ok := result["result"].([]interface{}); ok {
		return resultData, nil
	}

	return nil, fmt.Errorf("unexpected response format")
}

func (d *EOSDevice) sendRequest(ctx context.Context, payload interface{}) ([]byte, error) {
	logger.Debugf("Starting sendRequest for %s", d.Config.Name)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf("Failed to marshal JSON for %s: %v", d.Config.Name, err)
		return nil, err
	}
	logger.Debugf("JSON payload marshaled for %s, size: %d bytes", d.Config.Name, len(jsonData))
	
	// Dump the JSON payload for debugging
	logger.Infof("JSON Request for %s:\n%s", d.Config.Name, string(jsonData))

	url := fmt.Sprintf("https://%s:%d/command-api", d.Config.Host, d.Config.Port)
	logger.Debugf("Creating HTTP request for %s to %s", d.Config.Name, url)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(d.Config.Username, d.Config.Password)

	logger.Debugf("Making HTTP request to %s with username: %s", url, d.Config.Username)
	
	// Generate curl command for manual testing
	curlCmd := fmt.Sprintf("curl -k -X POST %s -H \"Content-Type: application/json\" -u \"%s:%s\" -d '%s'", 
		url, d.Config.Username, d.Config.Password, string(jsonData))
	logger.Infof("Equivalent curl command:\n%s", curlCmd)
	
	// Create a shorter timeout context for the HTTP request
	httpCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req = req.WithContext(httpCtx)
	
	logger.Debugf("About to execute HTTP client.Do() for %s", d.Config.Name)
	resp, err := d.client.Do(req)
	logger.Debugf("HTTP client.Do() completed for %s", d.Config.Name)
	if err != nil {
		logger.Errorf("HTTP request failed to %s: %v", url, err)
		return nil, err
	}
	logger.Debugf("Received HTTP response from %s: status=%d", url, resp.StatusCode)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (d *EOSDevice) expandTemplate(cmd Command) string {
	cmdStr := cmd.Template
	for key, value := range cmd.Params {
		placeholder := fmt.Sprintf("{%s}", key)
		cmdStr = strings.ReplaceAll(cmdStr, placeholder, fmt.Sprint(value))
	}
	return cmdStr
}

func (d *EOSDevice) getRequestID() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requestID++
	return d.requestID
}