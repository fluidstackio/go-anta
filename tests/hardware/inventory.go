package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/platform"
	"github.com/fluidstackio/go-anta/pkg/test"
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
//     minimum_memory: 8192  # in MB
//     minimum_flash: 4096   # in MB
//     required_modules:
//   - "DCS-7050SX3-48YC8"
//   - "PWR-460AC-F"
type VerifyInventory struct {
	test.BaseTest
	MinimumMemory   int64    `yaml:"minimum_memory,omitempty" json:"minimum_memory,omitempty"`
	MinimumFlash    int64    `yaml:"minimum_flash,omitempty" json:"minimum_flash,omitempty"`
	MinimumSupplies int      `yaml:"minimum_supplies,omitempty" json:"minimum_supplies,omitempty"`
	RequiredModules []string `yaml:"required_modules,omitempty" json:"required_modules,omitempty"`
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

// InventoryModule is one row from `show inventory`'s
// systemInformation list — chassis, supervisors, line cards,
// power supplies, fan trays.
type InventoryModule struct {
	Name        string `json:"name"`
	Model       string `json:"model,omitempty"`
	Serial      string `json:"serial,omitempty"`
	HwRevision  string `json:"hw_revision,omitempty"`
	Description string `json:"description,omitempty"`
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

	versionData, err := test.AsMap(cmdResult.Output)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Unexpected version output: %v", err)
		return result, nil
	}

	issues := []string{}
	details := map[string]any{}

	if model, ok := versionData["modelName"].(string); ok {
		details["model"] = model
	}
	if v, ok := versionData["version"].(string); ok {
		details["eos_version"] = v
	}
	if v, ok := versionData["serialNumber"].(string); ok {
		details["serial_number"] = v
	}
	if v, ok := versionData["systemMacAddress"].(string); ok {
		details["system_mac"] = v
	}

	var memTotalMB int64
	if memTotal, ok := versionData["memTotal"].(float64); ok {
		// EOS reports memTotal in kilobytes (`/proc/meminfo` convention),
		// not bytes.
		memTotalMB = int64(memTotal / 1024)
		details["memory_mb"] = memTotalMB
	}
	if t.MinimumMemory > 0 && memTotalMB > 0 && memTotalMB < t.MinimumMemory {
		issues = append(issues, fmt.Sprintf("Insufficient memory: %d MB < %d MB required",
			memTotalMB, t.MinimumMemory))
	}

	// Always read flash size (not just when MinimumFlash is configured) —
	// it's useful information for the report even without a threshold.
	fsCmd := device.Command{
		Template: "show file systems",
		Format:   "json",
		UseCache: true,
	}
	if fsResult, err := dev.Execute(ctx, fsCmd); err == nil {
		if fsData, err := test.AsMap(fsResult.Output); err == nil {
			var flashSizeMB int64 = -1
			if entries, ok := fsData["fileSystems"].([]any); ok {
				for _, e := range entries {
					entry, ok := e.(map[string]any)
					if !ok {
						continue
					}
					if prefix, _ := entry["prefix"].(string); prefix == "flash:" {
						if size, ok := entry["size"].(float64); ok {
							flashSizeMB = int64(size / 1024)
						}
						break
					}
				}
			}
			if flashSizeMB >= 0 {
				details["flash_mb"] = flashSizeMB
			}
			if t.MinimumFlash > 0 {
				switch {
				case flashSizeMB < 0:
					issues = append(issues, "flash: filesystem not found in `show file systems`")
				case flashSizeMB < t.MinimumFlash:
					issues = append(issues, fmt.Sprintf("Insufficient flash: %d MB < %d MB required",
						flashSizeMB, t.MinimumFlash))
				}
			}
		}
	}

	// Always read the inventory list so the report can render the modules
	// table; used to be gated on RequiredModules/MinimumSupplies being set.
	var modulesList []InventoryModule
	moduleNames := map[string]bool{}
	powerSupplyCount := 0
	invCmd := device.Command{
		Template: "show inventory",
		Format:   "json",
		UseCache: true,
	}
	if invResult, err := dev.Execute(ctx, invCmd); err == nil {
		if invData, ok := invResult.Output.(map[string]any); ok {
			if systemInfo, ok := invData["systemInformation"].([]any); ok {
				for _, item := range systemInfo {
					m, ok := item.(map[string]any)
					if !ok {
						continue
					}
					row := InventoryModule{}
					if v, ok := m["name"].(string); ok {
						row.Name = v
						moduleNames[v] = true
						if strings.HasPrefix(v, "PowerSupply") {
							powerSupplyCount++
						}
					}
					if v, ok := m["modelName"].(string); ok {
						row.Model = v
						moduleNames[v] = true
					}
					if v, ok := m["serialNumber"].(string); ok {
						row.Serial = v
					}
					if v, ok := m["hardwareRevision"].(string); ok {
						row.HwRevision = v
					}
					if v, ok := m["description"].(string); ok {
						row.Description = v
					}
					modulesList = append(modulesList, row)
				}
			}
		}
	}
	if len(modulesList) > 0 {
		details["modules"] = modulesList
		details["module_count"] = len(modulesList)
		details["power_supply_count"] = powerSupplyCount
	}

	if t.MinimumSupplies > 0 && powerSupplyCount < t.MinimumSupplies {
		issues = append(issues, fmt.Sprintf("Insufficient power supplies: %d < %d required",
			powerSupplyCount, t.MinimumSupplies))
	}
	for _, required := range t.RequiredModules {
		if !moduleNames[required] {
			issues = append(issues, fmt.Sprintf("Required module not found: %s", required))
		}
	}

	if len(issues) > 0 {
		details["issues"] = issues
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Inventory issues: %v", issues)
	} else {
		result.Message = fmt.Sprintf("%s · EOS %s · %d modules, %d PSUs",
			details["model"], details["eos_version"], len(modulesList), powerSupplyCount)
	}
	result.Details = details

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
