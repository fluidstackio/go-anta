package hardware

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/platform"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyChassisHealth verifies the health of the hardware chassis components.
//
// This test validates the overall health status of chassis components including
// power supplies, fan trays, temperature sensors, and other critical hardware
// subsystems that affect device reliability and performance.
//
// The test performs the following checks:
//  1. Retrieves cooling system status including fan trays and cooling health.
//  2. Validates power supply status and overall power system health.
//  3. Checks temperature sensors across all card slots and modules.
//  4. Reports any component failures, warnings, or alerts.
//
// Expected Results:
//   - Success: All chassis components are healthy and operational.
//   - Failure: One or more chassis components report failures or warnings.
//   - Error: Unable to retrieve chassis health information.
//
// Examples:
//
//   - name: "VerifyChassisHealth"
//     module: "hardware"
//     categories: ["hardware", "chassis"]
//     inputs: {}
//
//   - name: "VerifyChassisHealth"
//     module: "hardware"
//     categories: ["hardware", "chassis"]
//     inputs:
//     check_all_subsystems: true
type VerifyChassisHealth struct {
	test.BaseTest
	CheckAllSubsystems bool `yaml:"check_all_subsystems,omitempty" json:"check_all_subsystems,omitempty"`
}

func NewVerifyChassisHealth(inputs map[string]any) (test.Test, error) {
	t := &VerifyChassisHealth{
		BaseTest: test.BaseTest{
			TestName:        "VerifyChassisHealth",
			TestDescription: "Verify health of hardware chassis components",
			TestCategories:  []string{"hardware", "chassis"},
		},
		CheckAllSubsystems: true,
	}

	if inputs != nil {
		if checkAll, ok := inputs["check_all_subsystems"].(bool); ok {
			t.CheckAllSubsystems = checkAll
		}
	}

	return t, nil
}

func (t *VerifyChassisHealth) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where chassis health is not applicable
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "chassis health monitoring is not applicable"); skipResult != nil {
		return skipResult, nil
	}

	healthIssues := []string{}

	// Check cooling (fans)
	coolingCmd := device.Command{
		Template: "show system environment cooling",
		Format:   "json",
		UseCache: false,
	}
	coolingResult, err := dev.Execute(ctx, coolingCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get cooling data: %v", err)
		return result, nil
	}
	if coolingData, ok := coolingResult.Output.(map[string]any); ok {
		t.checkCoolingHealth(coolingData, &healthIssues)
	}

	// Check power supplies
	powerCmd := device.Command{
		Template: "show system environment power",
		Format:   "json",
		UseCache: false,
	}
	powerResult, err := dev.Execute(ctx, powerCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get power data: %v", err)
		return result, nil
	}
	if powerData, ok := powerResult.Output.(map[string]any); ok {
		t.checkPowerHealth(powerData, &healthIssues)
	}

	// Check temperature sensors
	tempCmd := device.Command{
		Template: "show system environment temperature",
		Format:   "json",
		UseCache: false,
	}
	tempResult, err := dev.Execute(ctx, tempCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get temperature data: %v", err)
		return result, nil
	}
	if tempData, ok := tempResult.Output.(map[string]any); ok {
		t.checkTemperatureHealth(tempData, &healthIssues)
	}

	if len(healthIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Chassis health issues: %v", healthIssues)
	}

	return result, nil
}

func (t *VerifyChassisHealth) checkCoolingHealth(coolingData map[string]any, issues *[]string) {
	// Check overall cooling status
	if systemStatus, ok := coolingData["systemStatus"].(string); ok {
		if !strings.EqualFold(systemStatus, "coolingOk") {
			*issues = append(*issues, fmt.Sprintf("Cooling system status: %s", systemStatus))
		}
	}

	// Check individual fan trays
	if fanTrays, ok := coolingData["fanTraySlots"].(map[string]any); ok {
		for fanTrayName, fanTrayData := range fanTrays {
			if fanTray, ok := fanTrayData.(map[string]any); ok {
				if status, ok := fanTray["status"].(string); ok {
					if !strings.EqualFold(status, "ok") {
						*issues = append(*issues, fmt.Sprintf("Fan tray %s: %s", fanTrayName, status))
					}
				}
			}
		}
	}

	// Check if requested for additional subsystem checks
	if t.CheckAllSubsystems {
		if powerSupplies, ok := coolingData["powerSupplySlots"].(map[string]any); ok {
			for psName, psData := range powerSupplies {
				if ps, ok := psData.(map[string]any); ok {
					if fanStatus, ok := ps["fanStatus"].(string); ok {
						if !strings.EqualFold(fanStatus, "ok") {
							*issues = append(*issues, fmt.Sprintf("Power supply %s fan: %s", psName, fanStatus))
						}
					}
				}
			}
		}
	}
}

func (t *VerifyChassisHealth) checkPowerHealth(powerData map[string]any, issues *[]string) {
	// Check overall power status
	if systemStatus, ok := powerData["systemStatus"].(string); ok {
		if !strings.EqualFold(systemStatus, "powerOk") {
			*issues = append(*issues, fmt.Sprintf("Power system status: %s", systemStatus))
		}
	}

	// Check individual power supplies
	if powerSupplies, ok := powerData["powerSupplies"].(map[string]any); ok {
		for psName, psData := range powerSupplies {
			if ps, ok := psData.(map[string]any); ok {
				if state, ok := ps["state"].(string); ok {
					if !strings.EqualFold(state, "ok") && !strings.EqualFold(state, "powerGood") {
						*issues = append(*issues, fmt.Sprintf("Power supply %s: %s", psName, state))
					}
				}
			}
		}
	}
}

func (t *VerifyChassisHealth) checkTemperatureHealth(tempData map[string]any, issues *[]string) {
	// Check overall system status
	if systemStatus, ok := tempData["systemStatus"].(string); ok {
		if !strings.EqualFold(systemStatus, "temperatureOk") {
			*issues = append(*issues, fmt.Sprintf("Temperature system status: %s", systemStatus))
		}
	}

	// Check temperature sensors
	if cardSlots, ok := tempData["cardSlots"].(map[string]any); ok {
		for cardName, cardData := range cardSlots {
			if card, ok := cardData.(map[string]any); ok {
				if tempSensors, ok := card["tempSensors"].(map[string]any); ok {
					for sensorName, sensorData := range tempSensors {
						if sensor, ok := sensorData.(map[string]any); ok {
							if hwStatus, ok := sensor["hwStatus"].(string); ok {
								if !strings.EqualFold(hwStatus, "ok") {
									*issues = append(*issues, fmt.Sprintf("Temperature sensor %s/%s: %s", cardName, sensorName, hwStatus))
								}
							}
							if alertCount, ok := sensor["alertCount"].(float64); ok {
								if alertCount > 0 {
									*issues = append(*issues, fmt.Sprintf("Temperature sensor %s/%s: %d alerts", cardName, sensorName, int(alertCount)))
								}
							}
						}
					}
				}
			}
		}
	}

	// Check power supply temperature sensors if CheckAllSubsystems is enabled
	if t.CheckAllSubsystems {
		if powerSupplies, ok := tempData["powerSupplySlots"].(map[string]any); ok {
			for psName, psData := range powerSupplies {
				if ps, ok := psData.(map[string]any); ok {
					if tempSensors, ok := ps["tempSensors"].(map[string]any); ok {
						for sensorName, sensorData := range tempSensors {
							if sensor, ok := sensorData.(map[string]any); ok {
								if hwStatus, ok := sensor["hwStatus"].(string); ok {
									if !strings.EqualFold(hwStatus, "ok") {
										*issues = append(*issues, fmt.Sprintf("Power supply %s temp sensor %s: %s", psName, sensorName, hwStatus))
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func (t *VerifyChassisHealth) ValidateInput(input any) error {
	// No specific validation required
	return nil
}

type VerifyHardwareCapacityUtilization struct {
	test.BaseTest
	MaxUtilizationPct    int  `yaml:"max_utilization_pct,omitempty" json:"max_utilization_pct,omitempty"`
	CheckForwardingTable bool `yaml:"check_forwarding_table,omitempty" json:"check_forwarding_table,omitempty"`
	CheckRouteTable      bool `yaml:"check_route_table,omitempty" json:"check_route_table,omitempty"`
	CheckAclTable        bool `yaml:"check_acl_table,omitempty" json:"check_acl_table,omitempty"`
}

func NewVerifyHardwareCapacityUtilization(inputs map[string]any) (test.Test, error) {
	t := &VerifyHardwareCapacityUtilization{
		BaseTest: test.BaseTest{
			TestName:        "VerifyHardwareCapacityUtilization",
			TestDescription: "Verify hardware capacity utilization",
			TestCategories:  []string{"hardware", "capacity"},
		},
		MaxUtilizationPct:    90, // Default: 90% threshold
		CheckForwardingTable: true,
		CheckRouteTable:      true,
		CheckAclTable:        true,
	}

	if inputs != nil {
		if maxUtil, ok := inputs["max_utilization_pct"].(float64); ok {
			t.MaxUtilizationPct = int(maxUtil)
		} else if maxUtil, ok := inputs["max_utilization_pct"].(int); ok {
			t.MaxUtilizationPct = maxUtil
		}
		if checkFT, ok := inputs["check_forwarding_table"].(bool); ok {
			t.CheckForwardingTable = checkFT
		}
		if checkRT, ok := inputs["check_route_table"].(bool); ok {
			t.CheckRouteTable = checkRT
		}
		if checkACL, ok := inputs["check_acl_table"].(bool); ok {
			t.CheckAclTable = checkACL
		}
	}

	return t, nil
}

func (t *VerifyHardwareCapacityUtilization) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where hardware capacity is not meaningful
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "hardware capacity monitoring is not meaningful"); skipResult != nil {
		return skipResult, nil
	}

	utilizationIssues := []string{}

	// Check forwarding table utilization
	if t.CheckForwardingTable {
		t.checkForwardingTableUtilization(ctx, dev, &utilizationIssues)
	}

	// Check route table utilization
	if t.CheckRouteTable {
		t.checkRouteTableUtilization(ctx, dev, &utilizationIssues)
	}

	// Check ACL table utilization
	if t.CheckAclTable {
		t.checkAclTableUtilization(ctx, dev, &utilizationIssues)
	}

	if len(utilizationIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Hardware capacity utilization issues: %v", utilizationIssues)
	}

	return result, nil
}

func (t *VerifyHardwareCapacityUtilization) checkForwardingTableUtilization(ctx context.Context, dev device.Device, issues *[]string) {
	cmd := device.Command{
		Template: "show platform trident forwarding-table summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		*issues = append(*issues, fmt.Sprintf("Failed to get forwarding table data: %v", err))
		return
	}

	if ftData, ok := cmdResult.Output.(map[string]any); ok {
		if summary, ok := ftData["summary"].(map[string]any); ok {
			if used, ok := summary["used"].(float64); ok {
				if total, ok := summary["total"].(float64); ok {
					utilization := int((used / total) * 100)
					if utilization > t.MaxUtilizationPct {
						*issues = append(*issues, fmt.Sprintf("Forwarding table utilization: %d%% exceeds threshold %d%%", utilization, t.MaxUtilizationPct))
					}
				}
			}
		}
	}
}

func (t *VerifyHardwareCapacityUtilization) checkRouteTableUtilization(ctx context.Context, dev device.Device, issues *[]string) {
	cmd := device.Command{
		Template: "show ip route summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		*issues = append(*issues, fmt.Sprintf("Failed to get route table data: %v", err))
		return
	}

	if routeData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := routeData["vrfs"].(map[string]any); ok {
			for vrfName, vrfData := range vrfs {
				if vrf, ok := vrfData.(map[string]any); ok {
					if routeTypes, ok := vrf["routes"].(map[string]any); ok {
						totalRoutes := 0
						for _, typeData := range routeTypes {
							if routes, ok := typeData.(float64); ok {
								totalRoutes += int(routes)
							}
						}
						// Use a reasonable estimate for maximum routes (device-specific)
						maxRoutes := 1000000 // This would need to be device-specific
						utilization := int((float64(totalRoutes) / float64(maxRoutes)) * 100)
						if utilization > t.MaxUtilizationPct {
							*issues = append(*issues, fmt.Sprintf("Route table utilization in VRF %s: %d%% exceeds threshold %d%%", vrfName, utilization, t.MaxUtilizationPct))
						}
					}
				}
			}
		}
	}
}

func (t *VerifyHardwareCapacityUtilization) checkAclTableUtilization(ctx context.Context, dev device.Device, issues *[]string) {
	cmd := device.Command{
		Template: "show platform trident tcam summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		*issues = append(*issues, fmt.Sprintf("Failed to get TCAM data: %v", err))
		return
	}

	if tcamData, ok := cmdResult.Output.(map[string]any); ok {
		if tables, ok := tcamData["tables"].(map[string]any); ok {
			for tableName, tableData := range tables {
				if table, ok := tableData.(map[string]any); ok {
					if used, ok := table["used"].(float64); ok {
						if total, ok := table["total"].(float64); ok {
							utilization := int((used / total) * 100)
							if utilization > t.MaxUtilizationPct {
								*issues = append(*issues, fmt.Sprintf("ACL table %s utilization: %d%% exceeds threshold %d%%", tableName, utilization, t.MaxUtilizationPct))
							}
						}
					}
				}
			}
		}
	}
}

func (t *VerifyHardwareCapacityUtilization) ValidateInput(input any) error {
	if t.MaxUtilizationPct < 0 || t.MaxUtilizationPct > 100 {
		return fmt.Errorf("maximum utilization percentage must be between 0 and 100")
	}
	return nil
}

// VerifyModuleStatus verifies the operational status and power stability of all modules in a modular chassis.
//
// This test validates that all installed modules (linecards, supervisors, fabric modules, etc.)
// in a modular chassis are operational and stable. Module status monitoring is critical for
// detecting hardware failures and ensuring system reliability.
//
// The test performs the following checks:
//  1. Retrieves status for all installed modules in the chassis.
//  2. Validates that modules are in operational states.
//  3. Checks for power stability and proper module initialization.
//  4. Reports any modules with failures or warnings.
//
// Expected Results:
//   - Success: All installed modules are operational and stable.
//   - Failure: One or more modules report failures, warnings, or unstable states.
//   - Error: Unable to retrieve module status information.
//
// Examples:
//
//   - name: VerifyModuleStatus basic check
//     VerifyModuleStatus: {}
//
//   - name: VerifyModuleStatus with power validation
//     VerifyModuleStatus:
//     check_power_status: true
//     check_temperature: true
type VerifyModuleStatus struct {
	test.BaseTest
	CheckPowerStatus bool `yaml:"check_power_status,omitempty" json:"check_power_status,omitempty"`
	CheckTemperature bool `yaml:"check_temperature,omitempty" json:"check_temperature,omitempty"`
}

func NewVerifyModuleStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifyModuleStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifyModuleStatus",
			TestDescription: "Verify operational status and power stability of all modules",
			TestCategories:  []string{"hardware", "modules"},
		},
		CheckPowerStatus: true,
		CheckTemperature: false,
	}

	if inputs != nil {
		if checkPower, ok := inputs["check_power_status"].(bool); ok {
			t.CheckPowerStatus = checkPower
		}
		if checkTemp, ok := inputs["check_temperature"].(bool); ok {
			t.CheckTemperature = checkTemp
		}
	}

	return t, nil
}

func (t *VerifyModuleStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Skip on virtual/lab platforms where modules are not applicable
	if skipResult := platform.SkipOnVirtualPlatforms(dev, t.Name(), t.Categories(), "modular chassis modules are not applicable"); skipResult != nil {
		return skipResult, nil
	}

	cmd := device.Command{
		Template: "show module",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get module data: %v", err)
		return result, nil
	}

	moduleIssues := []string{}

	if moduleData, ok := cmdResult.Output.(map[string]any); ok {
		if modules, ok := moduleData["modules"].(map[string]any); ok {
			for moduleName, moduleInfo := range modules {
				if module, ok := moduleInfo.(map[string]any); ok {
					t.checkModuleStatus(moduleName, module, &moduleIssues)
				}
			}
		}
	}

	if len(moduleIssues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Module status issues: %v", moduleIssues)
	}

	return result, nil
}

func (t *VerifyModuleStatus) checkModuleStatus(moduleName string, moduleData map[string]any, issues *[]string) {
	// Check operational status
	if status, ok := moduleData["status"].(string); ok {
		if !strings.EqualFold(status, "ok") && !strings.EqualFold(status, "active") && !strings.EqualFold(status, "standby") {
			*issues = append(*issues, fmt.Sprintf("Module %s: status %s", moduleName, status))
		}
	}

	// Check power status if requested
	if t.CheckPowerStatus {
		if powerStatus, ok := moduleData["powerStatus"].(string); ok {
			if !strings.EqualFold(powerStatus, "powerGood") && !strings.EqualFold(powerStatus, "ok") {
				*issues = append(*issues, fmt.Sprintf("Module %s: power status %s", moduleName, powerStatus))
			}
		}
	}

	// Check temperature if requested
	if t.CheckTemperature {
		if tempStatus, ok := moduleData["temperatureStatus"].(string); ok {
			if !strings.EqualFold(tempStatus, "ok") {
				*issues = append(*issues, fmt.Sprintf("Module %s: temperature status %s", moduleName, tempStatus))
			}
		}
	}

	// Check for module errors
	if errors, ok := moduleData["errors"].([]any); ok {
		if len(errors) > 0 {
			*issues = append(*issues, fmt.Sprintf("Module %s: %d errors detected", moduleName, len(errors)))
		}
	}
}

func (t *VerifyModuleStatus) ValidateInput(input any) error {
	// No specific validation required
	return nil
}
