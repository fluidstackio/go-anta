package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/platform"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyUnifiedForwardingTableMode verifies the device is using the expected UFT (Unified Forwarding Table) mode.
//
// This test validates the UFT mode configuration on network devices, which determines how the
// forwarding table is partitioned between different forwarding mechanisms.
//
// Expected Results:
//   - Success: The test will pass if the device UFT mode matches the expected mode configuration.
//   - Failure: The test will fail if the UFT mode differs from the expected configuration.
//   - Error: The test will report an error if UFT mode information cannot be retrieved or parsed.
//
// Examples:
//   - name: VerifyUnifiedForwardingTableMode with numeric mode
//     VerifyUnifiedForwardingTableMode:
//       mode: 2
//
//   - name: VerifyUnifiedForwardingTableMode with flexible mode
//     VerifyUnifiedForwardingTableMode:
//       mode: "flexible"
//
//   - name: VerifyUnifiedForwardingTableMode with mode 0
//     VerifyUnifiedForwardingTableMode:
//       mode: 0
type VerifyUnifiedForwardingTableMode struct {
	test.BaseTest
	Mode any `yaml:"mode" json:"mode"` // Can be int (0,1,2,3,4) or string ("flexible")
}

func NewVerifyUnifiedForwardingTableMode(inputs map[string]any) (test.Test, error) {
	t := &VerifyUnifiedForwardingTableMode{
		BaseTest: test.BaseTest{
			TestName:        "VerifyUnifiedForwardingTableMode",
			TestDescription: "Verify device is using the expected UFT mode",
			TestCategories:  []string{"hardware", "profiles"},
		},
	}

	if inputs != nil {
		if mode, ok := inputs["mode"]; ok {
			t.Mode = mode
		}
	}

	return t, nil
}

func (t *VerifyUnifiedForwardingTableMode) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where hardware profiles are not applicable
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "UFT modes are not applicable"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show platform trident forwarding-table partition",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get UFT mode information: %v", err)
		return result, nil
	}

	// Parse the UFT mode from the output
	var actualMode any
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if partitions, ok := data["partitions"].(map[string]any); ok {
			// Look for the mode in the partitions data
			// This may vary based on EOS version, so we'll look for common keys
			for key, value := range partitions {
				if strings.Contains(strings.ToLower(key), "mode") {
					actualMode = value
					break
				}
			}

			// If mode not found in partitions, check if there's a direct mode field
			if actualMode == nil {
				if mode, exists := data["mode"]; exists {
					actualMode = mode
				}
			}
		}

		// Fallback: check for mode directly in root
		if actualMode == nil {
			if mode, exists := data["mode"]; exists {
				actualMode = mode
			}
		}
	}

	if actualMode == nil {
		result.Status = test.TestError
		result.Message = "UFT mode information not found in device output"
		return result, nil
	}

	// Compare the modes (handle both numeric and string cases)
	if !t.modesEqual(actualMode, t.Mode) {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("UFT mode mismatch: expected %v, got %v", t.Mode, actualMode)
	}

	return result, nil
}

func (t *VerifyUnifiedForwardingTableMode) modesEqual(actual, expected any) bool {
	// Handle string comparison
	if actualStr, ok := actual.(string); ok {
		if expectedStr, ok := expected.(string); ok {
			return strings.EqualFold(actualStr, expectedStr)
		}
		// Compare string actual with numeric expected
		if expectedNum, ok := expected.(int); ok {
			return actualStr == fmt.Sprintf("%d", expectedNum)
		}
		if expectedFloat, ok := expected.(float64); ok {
			return actualStr == fmt.Sprintf("%.0f", expectedFloat)
		}
	}

	// Handle numeric comparison
	if actualFloat, ok := actual.(float64); ok {
		if expectedFloat, ok := expected.(float64); ok {
			return actualFloat == expectedFloat
		}
		if expectedInt, ok := expected.(int); ok {
			return actualFloat == float64(expectedInt)
		}
		if expectedStr, ok := expected.(string); ok {
			return fmt.Sprintf("%.0f", actualFloat) == expectedStr
		}
	}

	if actualInt, ok := actual.(int); ok {
		if expectedInt, ok := expected.(int); ok {
			return actualInt == expectedInt
		}
		if expectedFloat, ok := expected.(float64); ok {
			return float64(actualInt) == expectedFloat
		}
		if expectedStr, ok := expected.(string); ok {
			return fmt.Sprintf("%d", actualInt) == expectedStr
		}
	}

	// Direct comparison as fallback
	return actual == expected
}

func (t *VerifyUnifiedForwardingTableMode) ValidateInput(input any) error {
	if t.Mode == nil {
		return fmt.Errorf("mode must be specified")
	}

	// Validate mode values
	switch mode := t.Mode.(type) {
	case int:
		if mode < 0 || mode > 4 {
			return fmt.Errorf("numeric mode must be between 0 and 4, got %d", mode)
		}
	case float64:
		intMode := int(mode)
		if mode != float64(intMode) || intMode < 0 || intMode > 4 {
			return fmt.Errorf("numeric mode must be between 0 and 4, got %v", mode)
		}
	case string:
		if !strings.EqualFold(mode, "flexible") && mode != "0" && mode != "1" && mode != "2" && mode != "3" && mode != "4" {
			return fmt.Errorf("string mode must be 'flexible' or numeric string '0'-'4', got %s", mode)
		}
	default:
		return fmt.Errorf("mode must be int, float64, or string, got %T", mode)
	}

	return nil
}

// VerifyTcamProfile verifies that the device is using the provided Ternary Content-Addressable Memory (TCAM) profile.
//
// This test validates the TCAM profile configuration on network devices, which determines how the
// TCAM memory is allocated for different types of forwarding rules and lookup tables.
//
// Expected Results:
//   - Success: The test will pass if the specified TCAM profile is running on the device.
//   - Failure: The test will fail if no TCAM profile is found or an incorrect profile is running.
//   - Error: The test will report an error if TCAM profile information cannot be retrieved or parsed.
//
// Examples:
//   - name: VerifyTcamProfile with specific profile
//     VerifyTcamProfile:
//       profile: "vxlan-routing"
//
//   - name: VerifyTcamProfile with default profile
//     VerifyTcamProfile:
//       profile: "default"
//
//   - name: VerifyTcamProfile with custom profile
//     VerifyTcamProfile:
//       profile: "custom-acl-heavy"
type VerifyTcamProfile struct {
	test.BaseTest
	Profile string `yaml:"profile" json:"profile"`
}

func NewVerifyTcamProfile(inputs map[string]any) (test.Test, error) {
	t := &VerifyTcamProfile{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTcamProfile",
			TestDescription: "Verify device is using the provided TCAM profile",
			TestCategories:  []string{"hardware", "profiles"},
		},
	}

	if inputs != nil {
		if profile, ok := inputs["profile"].(string); ok {
			t.Profile = profile
		}
	}

	return t, nil
}

func (t *VerifyTcamProfile) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where hardware profiles are not applicable
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "TCAM profiles are not applicable"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show hardware tcam profile",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get TCAM profile information: %v", err)
		return result, nil
	}

	// Parse the TCAM profile from the output
	var actualProfile string
	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Look for profile information in various possible locations
		if profile, ok := data["profile"].(string); ok {
			actualProfile = profile
		} else if profiles, ok := data["profiles"].(map[string]any); ok {
			// Look for active profile in profiles map
			for profileName, profileData := range profiles {
				if profileInfo, ok := profileData.(map[string]any); ok {
					if active, ok := profileInfo["active"].(bool); ok && active {
						actualProfile = profileName
						break
					}
				}
			}
		} else if tcamProfiles, ok := data["tcamProfiles"].(map[string]any); ok {
			// Alternative structure for TCAM profiles
			for profileName, profileData := range tcamProfiles {
				if profileInfo, ok := profileData.(map[string]any); ok {
					if active, ok := profileInfo["active"].(bool); ok && active {
						actualProfile = profileName
						break
					}
				}
			}
		}

		// Check for activeProfile field
		if actualProfile == "" {
			if activeProfile, ok := data["activeProfile"].(string); ok {
				actualProfile = activeProfile
			}
		}
	}

	if actualProfile == "" {
		result.Status = test.TestError
		result.Message = "TCAM profile information not found in device output"
		return result, nil
	}

	// Compare profiles (case-insensitive)
	if !strings.EqualFold(actualProfile, t.Profile) {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("TCAM profile mismatch: expected '%s', got '%s'", t.Profile, actualProfile)
	}

	return result, nil
}

func (t *VerifyTcamProfile) ValidateInput(input any) error {
	if t.Profile == "" {
		return fmt.Errorf("profile must be specified and non-empty")
	}

	return nil
}