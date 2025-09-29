package vxlan

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyVxlanConfigSanity verifies that there are no VXLAN configuration inconsistencies.
//
// This test performs the following checks:
//  1. Verifies the overall VXLAN configuration sanity status.
//  2. Checks category-specific sanity checks (localVtep, mlag, pd).
//  3. Validates that there are no configuration inconsistencies reported.
//  4. Ensures there are no errors in the VXLAN configuration.
//
// Expected Results:
//   - Success: All VXLAN configuration sanity checks pass with no inconsistencies or errors.
//   - Failure: Configuration sanity check fails, inconsistencies are found, or errors are present.
//   - Error: Unable to retrieve VXLAN configuration sanity information from the device.
//
// Example YAML configuration:
//   - name: "VerifyVxlanConfigSanity"
//     module: "vxlan"
//     # No additional inputs required

type VerifyVxlanConfigSanity struct {
	test.BaseTest
}

func NewVerifyVxlanConfigSanity(inputs map[string]any) (test.Test, error) {
	t := &VerifyVxlanConfigSanity{
		BaseTest: test.BaseTest{
			TestName:        "VerifyVxlanConfigSanity",
			TestDescription: "Verify there are no VXLAN config-sanity inconsistencies",
			TestCategories:  []string{"vxlan", "configuration"},
		},
	}

	return t, nil
}

func (t *VerifyVxlanConfigSanity) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show vxlan config-sanity",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get VXLAN config sanity: %v", err)
		return result, nil
	}

	issues := []string{}

	if sanityData, ok := cmdResult.Output.(map[string]any); ok {
		// Check overall sanity status
		if sanityCheck, ok := sanityData["sanityCheck"].(string); ok {
			if sanityCheck != "passed" && sanityCheck != "ok" {
				issues = append(issues, fmt.Sprintf("VXLAN config sanity check failed: %s", sanityCheck))
			}
		}

		// Check category-specific sanity checks
		categories := []string{"localVtep", "mlag", "pd"}

		for _, category := range categories {
			if categoryData, ok := sanityData[category].(map[string]any); ok {
				if status, ok := categoryData["status"].(string); ok {
					if status != "passed" && status != "ok" {
						issues = append(issues, fmt.Sprintf("VXLAN %s sanity check failed: %s", category, status))
					}
				}

				// Check for specific inconsistencies
				if inconsistencies, ok := categoryData["inconsistencies"].([]any); ok && len(inconsistencies) > 0 {
					for _, inconsistency := range inconsistencies {
						if inc, ok := inconsistency.(map[string]any); ok {
							var incType, description string
							if t, ok := inc["type"].(string); ok {
								incType = t
							}
							if desc, ok := inc["description"].(string); ok {
								description = desc
							}
							issues = append(issues, fmt.Sprintf("VXLAN %s inconsistency - %s: %s", category, incType, description))
						}
					}
				}
			}
		}

		// Check for general inconsistencies
		if inconsistencies, ok := sanityData["inconsistencies"].([]any); ok && len(inconsistencies) > 0 {
			for _, inconsistency := range inconsistencies {
				if inc, ok := inconsistency.(string); ok {
					issues = append(issues, fmt.Sprintf("VXLAN config inconsistency: %s", inc))
				} else if inc, ok := inconsistency.(map[string]any); ok {
					var incType, description string
					if t, ok := inc["type"].(string); ok {
						incType = t
					}
					if desc, ok := inc["description"].(string); ok {
						description = desc
					}
					issues = append(issues, fmt.Sprintf("VXLAN inconsistency - %s: %s", incType, description))
				}
			}
		}

		// Check for errors in the output
		if errors, ok := sanityData["errors"].([]any); ok && len(errors) > 0 {
			for _, errorItem := range errors {
				if errStr, ok := errorItem.(string); ok {
					issues = append(issues, fmt.Sprintf("VXLAN config error: %s", errStr))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("VXLAN config sanity issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyVxlanConfigSanity) ValidateInput(input any) error {
	return nil
}