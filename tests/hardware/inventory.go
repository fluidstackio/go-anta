package hardware

import (
	"context"
	"fmt"

	"github.com/fluidstack/go-anta/pkg/device"
	"github.com/fluidstack/go-anta/pkg/platform"
	"github.com/fluidstack/go-anta/pkg/test"
)

// VerifyInventory verifies that the device hardware inventory meets specified requirements.
//
// This test performs the following checks:
//  1. Validates minimum memory requirements if specified.
//  2. Validates minimum flash storage requirements if specified.
//  3. Verifies that all required modules are present in the inventory.
//
// Expected Results:
//   - Success: All specified hardware requirements are met.
//   - Failure: Insufficient memory/flash or required modules are missing.
//   - Error: Unable to retrieve hardware inventory information.
//
// Example YAML configuration:
//   - name: "VerifyInventory"
//     module: "hardware"
//     inputs:
//       minimum_memory: 8192  # in MB
//       minimum_flash: 4096   # in MB
//       required_modules:
//         - "DCS-7050SX3-48YC8"
//         - "PWR-460AC-F"
type VerifyInventory struct {
	test.BaseTest
	MinimumMemory    int64    `yaml:"minimum_memory,omitempty" json:"minimum_memory,omitempty"`
	MinimumFlash     int64    `yaml:"minimum_flash,omitempty" json:"minimum_flash,omitempty"`
	MinimumSupplies  int      `yaml:"minimum_supplies,omitempty" json:"minimum_supplies,omitempty"`
	RequiredModules  []string `yaml:"required_modules,omitempty" json:"required_modules,omitempty"`
}

func NewVerifyInventory(inputs map[string]any) (test.Test, error) {
	t := &VerifyInventory{
		BaseTest: test.BaseTest{
			TestName:        "VerifyInventory",
			TestDescription: "Verify hardware inventory meets requirements",
			TestCategories:  []string{"hardware", "inventory"},
		},
	}

	if inputs != nil {
		if mem, ok := inputs["minimum_memory"].(float64); ok {
			t.MinimumMemory = int64(mem)
		} else if mem, ok := inputs["minimum_memory"].(int); ok {
			t.MinimumMemory = int64(mem)
		}

		if flash, ok := inputs["minimum_flash"].(float64); ok {
			t.MinimumFlash = int64(flash)
		} else if flash, ok := inputs["minimum_flash"].(int); ok {
			t.MinimumFlash = int64(flash)
		}

		if supplies, ok := inputs["minimum_supplies"].(float64); ok {
			t.MinimumSupplies = int(supplies)
		} else if supplies, ok := inputs["minimum_supplies"].(int); ok {
			t.MinimumSupplies = supplies
		}

		if modules, ok := inputs["required_modules"].([]any); ok {
			for _, m := range modules {
				if module, ok := m.(string); ok {
					t.RequiredModules = append(t.RequiredModules, module)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyInventory) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where physical hardware inventory is not meaningful
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "physical hardware inventory validation is not meaningful"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show version",
		Format:   "json",
		UseCache: true,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get version data: %v", err)
		return result, nil
	}

	issues := []string{}

	if versionData, ok := cmdResult.Output.(map[string]any); ok {
		if t.MinimumMemory > 0 {
			if memTotal, ok := versionData["memTotal"].(float64); ok {
				memTotalMB := int64(memTotal / 1024 / 1024)
				if memTotalMB < t.MinimumMemory {
					issues = append(issues, fmt.Sprintf("Insufficient memory: %d MB < %d MB required", 
						memTotalMB, t.MinimumMemory))
				}
			}
		}

		if t.MinimumFlash > 0 {
			if flashSize, ok := versionData["flashSize"].(float64); ok {
				flashSizeMB := int64(flashSize / 1024 / 1024)
				if flashSizeMB < t.MinimumFlash {
					issues = append(issues, fmt.Sprintf("Insufficient flash: %d MB < %d MB required",
						flashSizeMB, t.MinimumFlash))
				}
			}
		}
	}

	if len(t.RequiredModules) > 0 || t.MinimumSupplies > 0 {
		invCmd := device.Command{
			Template: "show inventory",
			Format:   "json",
			UseCache: true,
		}

		invResult, err := dev.Execute(ctx, invCmd)
		if err == nil {
			if invData, ok := invResult.Output.(map[string]any); ok {
				if systemInfo, ok := invData["systemInformation"].([]any); ok {
					modules := make(map[string]bool)
					powerSupplyCount := 0

					for _, item := range systemInfo {
						if itemData, ok := item.(map[string]any); ok {
							if name, ok := itemData["name"].(string); ok {
								modules[name] = true
								// Count power supplies
								if len(name) >= 11 && name[:11] == "PowerSupply" {
									powerSupplyCount++
								}
							}
							if model, ok := itemData["modelName"].(string); ok {
								modules[model] = true
							}
						}
					}

					if t.MinimumSupplies > 0 && powerSupplyCount < t.MinimumSupplies {
						issues = append(issues, fmt.Sprintf("Insufficient power supplies: %d < %d required",
							powerSupplyCount, t.MinimumSupplies))
					}

					for _, required := range t.RequiredModules {
						if !modules[required] {
							issues = append(issues, fmt.Sprintf("Required module not found: %s", required))
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Inventory issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyInventory) ValidateInput(input any) error {
	if t.MinimumMemory < 0 {
		return fmt.Errorf("minimum memory cannot be negative")
	}
	if t.MinimumFlash < 0 {
		return fmt.Errorf("minimum flash cannot be negative")
	}
	if t.MinimumSupplies < 0 {
		return fmt.Errorf("minimum supplies cannot be negative")
	}
	return nil
}