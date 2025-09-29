package interfaces

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyInterfaceUtilization verifies that interface utilization is within acceptable limits.
//
// Expected Results:
//   - Success: The test will pass if all interface utilization is below the specified thresholds.
//   - Failure: The test will fail if any interface has utilization above its threshold.
//   - Error: The test will report an error if interface utilization data cannot be retrieved.
//
// Notes:
//   - Only full-duplex interfaces are checked; half-duplex interfaces are skipped.
//   - Utilization is calculated as the maximum of input and output rates against interface bandwidth.
//
// Examples:
//   - name: VerifyInterfaceUtilization global threshold
//     VerifyInterfaceUtilization:
//       threshold: 75.0  # Global threshold for all interfaces
//
//   - name: VerifyInterfaceUtilization per-interface thresholds
//     VerifyInterfaceUtilization:
//       threshold: 70.0
//       interfaces:
//         - name: "Ethernet1/1"
//           threshold: 80.0  # Interface-specific threshold
//         - name: "Ethernet2/1"
//           # Uses global threshold if not specified
type VerifyInterfaceUtilization struct {
	test.BaseTest
	Threshold  float64                         `yaml:"threshold,omitempty" json:"threshold,omitempty"`
	Interfaces []InterfaceUtilizationSettings `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
	CheckAll   bool                            `yaml:"check_all,omitempty" json:"check_all,omitempty"`
}

type InterfaceUtilizationSettings struct {
	Name      string  `yaml:"name" json:"name"`
	Threshold float64 `yaml:"threshold,omitempty" json:"threshold,omitempty"` // Interface-specific threshold
}

func NewVerifyInterfaceUtilization(inputs map[string]any) (test.Test, error) {
	t := &VerifyInterfaceUtilization{
		BaseTest: test.BaseTest{
			TestName:        "VerifyInterfaceUtilization",
			TestDescription: "Verify interface utilization is within acceptable limits",
			TestCategories:  []string{"interfaces", "utilization"},
		},
		Threshold: 75.0, // Default threshold
		CheckAll:  true, // Default to checking all interfaces
	}

	if inputs != nil {
		if threshold, ok := inputs["threshold"].(float64); ok {
			t.Threshold = threshold
		}

		if checkAll, ok := inputs["check_all"].(bool); ok {
			t.CheckAll = checkAll
		}

		if interfaces, ok := inputs["interfaces"].([]any); ok {
			t.CheckAll = false // If specific interfaces provided, don't check all
			for _, i := range interfaces {
				if intfMap, ok := i.(map[string]any); ok {
					intf := InterfaceUtilizationSettings{}

					if name, ok := intfMap["name"].(string); ok {
						intf.Name = name
					}
					if threshold, ok := intfMap["threshold"].(float64); ok {
						intf.Threshold = threshold
					} else {
						intf.Threshold = t.Threshold // Use global threshold if not specified
					}

					t.Interfaces = append(t.Interfaces, intf)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyInterfaceUtilization) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// Get interface rates
	ratesCmd := device.Command{
		Template: "show interfaces counters rates",
		Format:   "json",
		UseCache: false,
	}

	ratesResult, err := dev.Execute(ctx, ratesCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get interface rates: %v", err)
		return result, nil
	}

	// Get interface status (for bandwidth and duplex info)
	statusCmd := device.Command{
		Template: "show interfaces status",
		Format:   "json",
		UseCache: false,
	}

	statusResult, err := dev.Execute(ctx, statusCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get interface status: %v", err)
		return result, nil
	}

	// Parse interface rates
	interfaceRates := make(map[string]InterfaceRates)
	if ratesData, ok := ratesResult.Output.(map[string]any); ok {
		if interfaces, ok := ratesData["interfaces"].(map[string]any); ok {
			for intfName, intfData := range interfaces {
				if data, ok := intfData.(map[string]any); ok {
					rates := InterfaceRates{
						Interface: intfName,
					}

					if inRate, ok := data["inBpsRate"].(float64); ok {
						rates.InBpsRate = inRate
					}
					if outRate, ok := data["outBpsRate"].(float64); ok {
						rates.OutBpsRate = outRate
					}

					interfaceRates[intfName] = rates
				}
			}
		}
	}

	// Parse interface status
	interfaceStatus := make(map[string]InterfaceStatusInfo)
	if statusData, ok := statusResult.Output.(map[string]any); ok {
		if statuses, ok := statusData["interfaceStatuses"].(map[string]any); ok {
			for intfName, intfData := range statuses {
				if data, ok := intfData.(map[string]any); ok {
					status := InterfaceStatusInfo{
						Interface: intfName,
					}

					if bandwidth, ok := data["bandwidth"].(float64); ok {
						status.Bandwidth = bandwidth
					}
					if duplex, ok := data["duplex"].(string); ok {
						status.Duplex = duplex
					}
					if linkStatus, ok := data["linkStatus"].(string); ok {
						status.LinkStatus = linkStatus
					}

					interfaceStatus[intfName] = status
				}
			}
		}
	}

	failures := []string{}

	if t.CheckAll && len(t.Interfaces) == 0 {
		// Check all interfaces with global threshold
		for intfName, rates := range interfaceRates {
			if status, found := interfaceStatus[intfName]; found {
				if failure := t.checkInterfaceUtilization(intfName, rates, status, t.Threshold); failure != "" {
					failures = append(failures, failure)
				}
			}
		}
	} else {
		// Check specified interfaces
		for _, expectedIntf := range t.Interfaces {
			if rates, foundRates := interfaceRates[expectedIntf.Name]; foundRates {
				if status, foundStatus := interfaceStatus[expectedIntf.Name]; foundStatus {
					if failure := t.checkInterfaceUtilization(expectedIntf.Name, rates, status, expectedIntf.Threshold); failure != "" {
						failures = append(failures, failure)
					}
				} else {
					failures = append(failures, fmt.Sprintf("Interface %s status not found", expectedIntf.Name))
				}
			} else {
				failures = append(failures, fmt.Sprintf("Interface %s rates not found", expectedIntf.Name))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Interface utilization failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyInterfaceUtilization) checkInterfaceUtilization(intfName string, rates InterfaceRates, status InterfaceStatusInfo, threshold float64) string {
	// Skip interfaces that are not connected or not full-duplex
	if status.LinkStatus != "connected" {
		return "" // Skip disconnected interfaces
	}

	if status.Duplex != "duplexFull" {
		return "" // Skip non-full-duplex interfaces
	}

	if status.Bandwidth <= 0 {
		return fmt.Sprintf("%s: invalid bandwidth %f", intfName, status.Bandwidth)
	}

	// Calculate utilization percentage
	// Use the higher of input or output rates for utilization calculation
	maxRate := rates.InBpsRate
	if rates.OutBpsRate > maxRate {
		maxRate = rates.OutBpsRate
	}

	utilizationPercent := (maxRate / status.Bandwidth) * 100

	if utilizationPercent > threshold {
		return fmt.Sprintf("%s: utilization %.2f%% > %.2f%% (rate: %.0f bps, bandwidth: %.0f bps)",
			intfName, utilizationPercent, threshold, maxRate, status.Bandwidth)
	}

	return ""
}

func (t *VerifyInterfaceUtilization) ValidateInput(input any) error {
	if t.Threshold <= 0 || t.Threshold > 100 {
		return fmt.Errorf("threshold must be between 0 and 100")
	}

	if !t.CheckAll && len(t.Interfaces) == 0 {
		return fmt.Errorf("either check_all must be true or specific interfaces must be provided")
	}

	for i, intf := range t.Interfaces {
		if intf.Name == "" {
			return fmt.Errorf("interface at index %d has no name", i)
		}
		if intf.Threshold <= 0 || intf.Threshold > 100 {
			return fmt.Errorf("interface %s: threshold must be between 0 and 100", intf.Name)
		}
	}

	return nil
}

type InterfaceRates struct {
	Interface  string
	InBpsRate  float64
	OutBpsRate float64
}

type InterfaceStatusInfo struct {
	Interface  string
	Bandwidth  float64
	Duplex     string
	LinkStatus string
}