package system

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/fluidstack/go-anta/pkg/device"
	"github.com/fluidstack/go-anta/pkg/test"
)

// VerifyZeroTouch verifies that ZeroTouch is disabled on the device.
//
// ZeroTouch is a feature that allows devices to be provisioned automatically on first boot.
// In production environments, it's typically recommended to disable ZeroTouch to prevent
// unauthorized configuration changes and ensure controlled device provisioning.
//
// The test performs the following checks:
//   1. Retrieves the ZeroTouch status from the device.
//   2. Verifies that ZeroTouch mode is set to 'disabled'.
//   3. Reports success if disabled, failure if enabled.
//
// Expected Results:
//   - Success: The test will pass if ZeroTouch is disabled.
//   - Failure: The test will fail if ZeroTouch is enabled.
//   - Error: The test will report an error if ZeroTouch status cannot be retrieved.
//
// Examples:
//   - name: VerifyZeroTouch basic check
//     VerifyZeroTouch: {}
//
//   - name: VerifyZeroTouch ensure disabled
//     VerifyZeroTouch:
//       # No parameters needed - validates ZeroTouch is disabled
type VerifyZeroTouch struct {
	test.BaseTest
}

func NewVerifyZeroTouch(inputs map[string]any) (test.Test, error) {
	t := &VerifyZeroTouch{
		BaseTest: test.BaseTest{
			TestName:        "VerifyZeroTouch",
			TestDescription: "Verify ZeroTouch is disabled",
			TestCategories:  []string{"system", "configuration"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyZeroTouch) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show zerotouch",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get ZeroTouch status: %v", err)
		return result, nil
	}

	var zeroTouchEnabled bool
	var zeroTouchMode string

	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Check for different possible response formats
		if mode, ok := data["mode"].(string); ok {
			zeroTouchMode = mode
		} else if enabled, ok := data["enabled"].(bool); ok {
			zeroTouchEnabled = enabled
			if enabled {
				zeroTouchMode = "enabled"
			} else {
				zeroTouchMode = "disabled"
			}
		} else if status, ok := data["status"].(string); ok {
			zeroTouchMode = status
		} else if zerotouch, ok := data["zerotouch"].(map[string]any); ok {
			// Nested structure
			if mode, ok := zerotouch["mode"].(string); ok {
				zeroTouchMode = mode
			} else if enabled, ok := zerotouch["enabled"].(bool); ok {
				zeroTouchEnabled = enabled
				if enabled {
					zeroTouchMode = "enabled"
				} else {
					zeroTouchMode = "disabled"
				}
			}
		}
	}

	// Check if ZeroTouch is enabled
	if zeroTouchEnabled || strings.EqualFold(zeroTouchMode, "enabled") || strings.EqualFold(zeroTouchMode, "active") {
		result.Status = test.TestFailure
		result.Message = "ZeroTouch is enabled - should be disabled for security"
	} else if zeroTouchMode == "" {
		result.Status = test.TestError
		result.Message = "Unable to determine ZeroTouch status from device response"
	}

	return result, nil
}

func (t *VerifyZeroTouch) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifyRunningConfigDiffs verifies there is no difference between running-config and startup-config.
//
// This test ensures configuration persistence by checking that the running configuration
// has been saved to the startup configuration. Differences between these configurations
// could result in unexpected behavior after a device reload.
//
// The test performs the following checks:
//   1. Compares the running configuration against the startup configuration.
//   2. Identifies any differences between the two configurations.
//   3. Reports specific configuration lines that differ.
//
// Expected Results:
//   - Success: The test will pass if running and startup configurations are identical.
//   - Failure: The test will fail if any differences exist between the configurations.
//   - Error: The test will report an error if configuration comparison cannot be performed.
//
// Examples:
//   - name: VerifyRunningConfigDiffs basic check
//     VerifyRunningConfigDiffs: {}
//
//   - name: VerifyRunningConfigDiffs ensure saved
//     VerifyRunningConfigDiffs:
//       # No parameters needed - validates configs are synchronized
type VerifyRunningConfigDiffs struct {
	test.BaseTest
}

func NewVerifyRunningConfigDiffs(inputs map[string]any) (test.Test, error) {
	t := &VerifyRunningConfigDiffs{
		BaseTest: test.BaseTest{
			TestName:        "VerifyRunningConfigDiffs",
			TestDescription: "Verify no differences between running and startup config",
			TestCategories:  []string{"system", "configuration"},
		},
	}

	// No input parameters required for this test
	return t, nil
}

func (t *VerifyRunningConfigDiffs) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show running-config diffs",
		Format:   "text",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get config differences: %v", err)
		return result, nil
	}

	// Check the output for differences
	var configDiffs string
	switch output := cmdResult.Output.(type) {
	case string:
		configDiffs = strings.TrimSpace(output)
	case map[string]any:
		// If JSON output, look for differences field
		if diffs, ok := output["differences"].(string); ok {
			configDiffs = strings.TrimSpace(diffs)
		} else if diffs, ok := output["diffs"].(string); ok {
			configDiffs = strings.TrimSpace(diffs)
		} else if message, ok := output["message"].(string); ok {
			configDiffs = strings.TrimSpace(message)
		}
	}

	// Check if there are differences
	if configDiffs != "" && !strings.Contains(strings.ToLower(configDiffs), "no difference") && !strings.Contains(configDiffs, "---") {
		// If output contains actual differences (not just empty diff markers)
		lines := strings.Split(configDiffs, "\n")
		significantDiffs := []string{}

		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Filter out empty lines and diff markers
			if line != "" && !strings.HasPrefix(line, "---") && !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "@@") {
				significantDiffs = append(significantDiffs, line)
			}
		}

		if len(significantDiffs) > 0 {
			result.Status = test.TestFailure
			result.Message = fmt.Sprintf("Configuration differences found: %d lines differ between running and startup config", len(significantDiffs))

			// Include first few differences in the message for context
			if len(significantDiffs) > 5 {
				result.Message += fmt.Sprintf(". First 5 differences: %v", significantDiffs[:5])
			} else {
				result.Message += fmt.Sprintf(". Differences: %v", significantDiffs)
			}
		}
	}

	return result, nil
}

func (t *VerifyRunningConfigDiffs) ValidateInput(input any) error {
	// No input validation required for this test
	return nil
}

// VerifyRunningConfigLines verifies that specific configuration lines exist in the running configuration.
//
// This test searches the running configuration for specified regular expression patterns,
// allowing validation of critical configuration elements, security settings, or
// compliance requirements.
//
// The test performs the following checks:
//   1. Retrieves the complete running configuration from the device.
//   2. Searches for each specified regex pattern in the configuration.
//   3. Reports which patterns are found and which are missing.
//
// Expected Results:
//   - Success: The test will pass if all specified regex patterns are found in the configuration.
//   - Failure: The test will fail if any specified pattern is missing from the configuration.
//   - Error: The test will report an error if configuration cannot be retrieved or regex compilation fails.
//
// Examples:
//   - name: VerifyRunningConfigLines security settings
//     VerifyRunningConfigLines:
//       regex_patterns:
//         - "^aaa authentication login default"
//         - "^ip ssh version 2"
//         - "^banner motd"
//
//   - name: VerifyRunningConfigLines NTP configuration
//     VerifyRunningConfigLines:
//       regex_patterns:
//         - "^ntp server [0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+"
//         - "^clock timezone"
type VerifyRunningConfigLines struct {
	test.BaseTest
	RegexPatterns []string `yaml:"regex_patterns" json:"regex_patterns"`
}

func NewVerifyRunningConfigLines(inputs map[string]any) (test.Test, error) {
	t := &VerifyRunningConfigLines{
		BaseTest: test.BaseTest{
			TestName:        "VerifyRunningConfigLines",
			TestDescription: "Verify specific configuration lines exist in running config",
			TestCategories:  []string{"system", "configuration"},
		},
	}

	if inputs != nil {
		if patterns, ok := inputs["regex_patterns"].([]any); ok {
			for _, pattern := range patterns {
				if patternStr, ok := pattern.(string); ok {
					t.RegexPatterns = append(t.RegexPatterns, patternStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyRunningConfigLines) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	if len(t.RegexPatterns) == 0 {
		result.Status = test.TestError
		result.Message = "No regex patterns specified for verification"
		return result, nil
	}

	cmd := device.Command{
		Template: "show running-config",
		Format:   "text",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get running configuration: %v", err)
		return result, nil
	}

	// Get the configuration text
	var configText string
	switch output := cmdResult.Output.(type) {
	case string:
		configText = output
	case map[string]any:
		// If JSON output, look for configuration field
		if config, ok := output["configuration"].(string); ok {
			configText = config
		} else if config, ok := output["config"].(string); ok {
			configText = config
		} else if config, ok := output["runningConfig"].(string); ok {
			configText = config
		}
	}

	if configText == "" {
		result.Status = test.TestError
		result.Message = "Unable to retrieve running configuration text"
		return result, nil
	}

	// Split configuration into lines for line-by-line matching
	configLines := strings.Split(configText, "\n")

	// Check each regex pattern
	missingPatterns := []string{}
	for _, pattern := range t.RegexPatterns {
		// Compile the regex
		re, err := regexp.Compile(pattern)
		if err != nil {
			result.Status = test.TestError
			result.Message = fmt.Sprintf("Invalid regex pattern '%s': %v", pattern, err)
			return result, nil
		}

		// Search for the pattern in configuration lines
		found := false
		for _, line := range configLines {
			if re.MatchString(line) {
				found = true
				break
			}
		}

		if !found {
			missingPatterns = append(missingPatterns, pattern)
		}
	}

	if len(missingPatterns) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Configuration patterns not found: %v", missingPatterns)
	}

	return result, nil
}

func (t *VerifyRunningConfigLines) ValidateInput(input any) error {
	if len(t.RegexPatterns) == 0 {
		return fmt.Errorf("at least one regex pattern must be specified")
	}

	// Validate that each pattern is a valid regex
	for i, pattern := range t.RegexPatterns {
		if pattern == "" {
			return fmt.Errorf("regex pattern at index %d is empty", i)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern at index %d '%s': %v", i, pattern, err)
		}
	}

	return nil
}