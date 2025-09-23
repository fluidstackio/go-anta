package hardware

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyInventory struct {
	test.BaseTest
	MinimumMemory   int64 `yaml:"minimum_memory,omitempty" json:"minimum_memory,omitempty"`
	MinimumFlash    int64 `yaml:"minimum_flash,omitempty" json:"minimum_flash,omitempty"`
	RequiredModules []string `yaml:"required_modules,omitempty" json:"required_modules,omitempty"`
}

func NewVerifyInventory(inputs map[string]interface{}) (test.Test, error) {
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

		if modules, ok := inputs["required_modules"].([]interface{}); ok {
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

	if versionData, ok := cmdResult.Output.(map[string]interface{}); ok {
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

	if len(t.RequiredModules) > 0 {
		invCmd := device.Command{
			Template: "show inventory",
			Format:   "json",
			UseCache: true,
		}

		invResult, err := dev.Execute(ctx, invCmd)
		if err == nil {
			if invData, ok := invResult.Output.(map[string]interface{}); ok {
				if systemInfo, ok := invData["systemInformation"].([]interface{}); ok {
					modules := make(map[string]bool)
					for _, item := range systemInfo {
						if itemData, ok := item.(map[string]interface{}); ok {
							if name, ok := itemData["name"].(string); ok {
								modules[name] = true
							}
							if model, ok := itemData["modelName"].(string); ok {
								modules[model] = true
							}
						}
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

func (t *VerifyInventory) ValidateInput(input interface{}) error {
	if t.MinimumMemory < 0 {
		return fmt.Errorf("minimum memory cannot be negative")
	}
	if t.MinimumFlash < 0 {
		return fmt.Errorf("minimum flash cannot be negative")
	}
	return nil
}