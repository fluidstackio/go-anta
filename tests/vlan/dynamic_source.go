package vlan

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyDynamicVlanSource verifies the configuration and status of dynamic VLAN sources.
//
// This test performs the following checks:
//  1. Verifies that all specified dynamic VLAN sources are configured on the device.
//  2. Validates that each dynamic VLAN source is active/enabled.
//  3. In strict mode, ensures only the specified sources are present (no additional sources).
//
// Expected Results:
//   - Success: All specified sources are present and active, with no unexpected sources in strict mode.
//   - Failure: Missing sources, inactive sources, or unexpected sources found in strict mode.
//   - Error: Unable to retrieve dynamic VLAN information from the device.
//
// Example YAML configuration:
//   - name: "VerifyDynamicVlanSource"
//     module: "vlan"
//     inputs:
//       sources:                 # Required: List of expected dynamic VLAN sources
//         - "radius"
//         - "dot1x"
//       strict: false            # Optional: Strict mode validation (default: false)

type VerifyDynamicVlanSource struct {
	test.BaseTest
	Sources []string `yaml:"sources" json:"sources"`
	Strict  bool     `yaml:"strict,omitempty" json:"strict,omitempty"`
}

func NewVerifyDynamicVlanSource(inputs map[string]any) (test.Test, error) {
	t := &VerifyDynamicVlanSource{
		BaseTest: test.BaseTest{
			TestName:        "VerifyDynamicVlanSource",
			TestDescription: "Verify dynamic VLAN allocation for specified VLAN sources",
			TestCategories:  []string{"vlan", "dynamic"},
		},
		Strict: false, // Default to non-strict mode
	}

	if inputs != nil {
		if sources, ok := inputs["sources"].([]any); ok {
			for _, s := range sources {
				if source, ok := s.(string); ok {
					t.Sources = append(t.Sources, source)
				}
			}
		}
		if strict, ok := inputs["strict"].(bool); ok {
			t.Strict = strict
		}
	}

	return t, nil
}

func (t *VerifyDynamicVlanSource) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show vlan dynamic",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get dynamic VLAN information: %v", err)
		return result, nil
	}

	issues := []string{}
	foundSources := make(map[string]bool)

	if dynamicData, ok := cmdResult.Output.(map[string]any); ok {
		// Check for dynamic VLAN sources
		if sources, ok := dynamicData["dynamicVlanSources"].([]any); ok {
			for _, source := range sources {
				if sourceInfo, ok := source.(map[string]any); ok {
					if sourceName, ok := sourceInfo["source"].(string); ok {
						foundSources[sourceName] = true

						// Check if source is active
						if status, ok := sourceInfo["status"].(string); ok {
							if status != "active" && status != "enabled" {
								issues = append(issues, fmt.Sprintf("Dynamic VLAN source '%s' is %s, expected active", sourceName, status))
							}
						}
					}
				}
			}
		}

		// Also check alternative structure
		if dynamicVlans, ok := dynamicData["dynamicVlans"].(map[string]any); ok {
			for sourceName, sourceData := range dynamicVlans {
				if source, ok := sourceData.(map[string]any); ok {
					foundSources[sourceName] = true

					if enabled, ok := source["enabled"].(bool); ok && !enabled {
						issues = append(issues, fmt.Sprintf("Dynamic VLAN source '%s' is disabled", sourceName))
					}
				}
			}
		}
	}

	// Check if all expected sources are found
	for _, expectedSource := range t.Sources {
		if !foundSources[expectedSource] {
			issues = append(issues, fmt.Sprintf("Dynamic VLAN source '%s' not found", expectedSource))
		}
	}

	// In strict mode, check if only expected sources are present
	if t.Strict {
		expectedSources := make(map[string]bool)
		for _, source := range t.Sources {
			expectedSources[source] = true
		}

		for foundSource := range foundSources {
			if !expectedSources[foundSource] {
				issues = append(issues, fmt.Sprintf("Unexpected dynamic VLAN source '%s' found in strict mode", foundSource))
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Dynamic VLAN source issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"expected_sources": t.Sources,
			"found_sources":    foundSources,
			"strict_mode":      t.Strict,
		}
	}

	return result, nil
}

func (t *VerifyDynamicVlanSource) ValidateInput(input any) error {
	if len(t.Sources) == 0 {
		return fmt.Errorf("at least one dynamic VLAN source must be specified")
	}
	return nil
}