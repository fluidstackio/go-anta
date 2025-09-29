package stp

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifySTPMode verifies the configured STP mode for a provided list of VLANs.
//
// Expected Results:
//   - Success: The test will pass if all specified VLANs are configured with the expected STP mode.
//   - Failure: The test will fail if any VLAN has an incorrect STP mode or is not configured.
//   - Error: The test will report an error if STP configuration cannot be retrieved.
//
// Examples:
//   - name: VerifySTPMode for specific VLANs
//     VerifySTPMode:
//       mode: "mstp"
//       vlans: [10, 20, 30]
type VerifySTPMode struct {
	test.BaseTest
	Mode  string `yaml:"mode" json:"mode"`
	Vlans []int  `yaml:"vlans" json:"vlans"`
}

func NewVerifySTPMode(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifySTPMode{
		BaseTest: test.BaseTest{
			TestName:        "VerifySTPMode",
			TestDescription: "Verify the configured STP mode for a provided list of VLAN(s)",
			TestCategories:  []string{"stp", "mode"},
		},
	}

	if inputs != nil {
		if mode, ok := inputs["mode"].(string); ok {
			t.Mode = mode
		}
		if vlans, ok := inputs["vlans"].([]interface{}); ok {
			for _, v := range vlans {
				if vlan, ok := v.(float64); ok {
					t.Vlans = append(t.Vlans, int(vlan))
				} else if vlan, ok := v.(int); ok {
					t.Vlans = append(t.Vlans, vlan)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifySTPMode) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP information: %v", err)
		return result, nil
	}

	issues := []string{}

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if mode, ok := stpData["protocolMode"].(string); ok {
			if mode != t.Mode {
				issues = append(issues, fmt.Sprintf("STP mode is '%s', expected '%s'", mode, t.Mode))
			}
		}

		if len(t.Vlans) > 0 {
			if instances, ok := stpData["instances"].(map[string]interface{}); ok {
				for _, vlanID := range t.Vlans {
					vlanStr := fmt.Sprintf("%d", vlanID)
					if instance, ok := instances[vlanStr].(map[string]interface{}); ok {
						if instanceMode, ok := instance["protocolMode"].(string); ok {
							if instanceMode != t.Mode {
								issues = append(issues, fmt.Sprintf("VLAN %d STP mode is '%s', expected '%s'", vlanID, instanceMode, t.Mode))
							}
						}
					} else {
						issues = append(issues, fmt.Sprintf("VLAN %d not found in STP instances", vlanID))
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP mode issues: %v", issues)
	}

	return result, nil
}

func (t *VerifySTPMode) ValidateInput(input interface{}) error {
	if t.Mode == "" {
		return fmt.Errorf("STP mode must be specified")
	}
	validModes := []string{"mstp", "rstp", "rapidPvst", "pvst"}
	for _, mode := range validModes {
		if t.Mode == mode {
			return nil
		}
	}
	return fmt.Errorf("invalid STP mode '%s', must be one of: %v", t.Mode, validModes)
}

// VerifySTPBlockedPorts verifies there are no STP blocked ports
type VerifySTPBlockedPorts struct {
	test.BaseTest
}

func NewVerifySTPBlockedPorts(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifySTPBlockedPorts{
		BaseTest: test.BaseTest{
			TestName:        "VerifySTPBlockedPorts",
			TestDescription: "Verify there are no STP blocked ports",
			TestCategories:  []string{"stp", "blocked-ports"},
		},
	}
	return t, nil
}

func (t *VerifySTPBlockedPorts) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree blockedports",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP blocked ports: %v", err)
		return result, nil
	}

	issues := []string{}
	blockedPorts := []string{}

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if instances, ok := stpData["spanningTreeInstances"].([]interface{}); ok {
			for _, instance := range instances {
				if inst, ok := instance.(map[string]interface{}); ok {
					if interfaces, ok := inst["interfaces"].(map[string]interface{}); ok {
						for intfName, intfData := range interfaces {
							if intf, ok := intfData.(map[string]interface{}); ok {
								if state, ok := intf["state"].(string); ok && strings.ToLower(state) == "blocking" {
									blockedPorts = append(blockedPorts, intfName)
								}
							}
						}
					}
				}
			}
		}
	}

	if len(blockedPorts) > 0 {
		issues = append(issues, fmt.Sprintf("Found blocked STP ports: %v", blockedPorts))
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP blocked ports found: %v", issues)
	}

	return result, nil
}

func (t *VerifySTPBlockedPorts) ValidateInput(input interface{}) error {
	return nil
}

// VerifySTPCounters verifies there are no errors in STP BPDU packets
type VerifySTPCounters struct {
	test.BaseTest
	Interfaces        []string `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
	IgnoredInterfaces []string `yaml:"ignored_interfaces,omitempty" json:"ignored_interfaces,omitempty"`
}

func NewVerifySTPCounters(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifySTPCounters{
		BaseTest: test.BaseTest{
			TestName:        "VerifySTPCounters",
			TestDescription: "Verify there are no errors in STP BPDU packets",
			TestCategories:  []string{"stp", "counters"},
		},
	}

	if inputs != nil {
		if interfaces, ok := inputs["interfaces"].([]interface{}); ok {
			for _, i := range interfaces {
				if intf, ok := i.(string); ok {
					t.Interfaces = append(t.Interfaces, intf)
				}
			}
		}
		if ignoredIntfs, ok := inputs["ignored_interfaces"].([]interface{}); ok {
			for _, i := range ignoredIntfs {
				if intf, ok := i.(string); ok {
					t.IgnoredInterfaces = append(t.IgnoredInterfaces, intf)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifySTPCounters) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree counters",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP counters: %v", err)
		return result, nil
	}

	issues := []string{}

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if interfaces, ok := stpData["interfaces"].(map[string]interface{}); ok {
			for intfName, intfData := range interfaces {
				// Skip ignored interfaces
				if contains(t.IgnoredInterfaces, intfName) {
					continue
				}

				// Check only specific interfaces if provided
				if len(t.Interfaces) > 0 && !contains(t.Interfaces, intfName) {
					continue
				}

				if intf, ok := intfData.(map[string]interface{}); ok {
					if counters, ok := intf["counters"].(map[string]interface{}); ok {
						// Check for BPDU errors
						if bpduErrors, ok := counters["bpduTaggedOther"].(float64); ok && bpduErrors > 0 {
							issues = append(issues, fmt.Sprintf("Interface %s has %g BPDU errors", intfName, bpduErrors))
						}
						if bpduInvalid, ok := counters["invalidBpdus"].(float64); ok && bpduInvalid > 0 {
							issues = append(issues, fmt.Sprintf("Interface %s has %g invalid BPDUs", intfName, bpduInvalid))
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP counter errors: %v", issues)
	}

	return result, nil
}

func (t *VerifySTPCounters) ValidateInput(input interface{}) error {
	return nil
}

// VerifySTPForwardingPorts verifies that all interfaces are in a forwarding state for provided VLAN(s)
type VerifySTPForwardingPorts struct {
	test.BaseTest
	Vlans []int `yaml:"vlans" json:"vlans"`
}

func NewVerifySTPForwardingPorts(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifySTPForwardingPorts{
		BaseTest: test.BaseTest{
			TestName:        "VerifySTPForwardingPorts",
			TestDescription: "Verify that all interfaces are in a forwarding state for provided VLAN(s)",
			TestCategories:  []string{"stp", "forwarding"},
		},
	}

	if inputs != nil {
		if vlans, ok := inputs["vlans"].([]interface{}); ok {
			for _, v := range vlans {
				if vlan, ok := v.(float64); ok {
					t.Vlans = append(t.Vlans, int(vlan))
				} else if vlan, ok := v.(int); ok {
					t.Vlans = append(t.Vlans, vlan)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifySTPForwardingPorts) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP information: %v", err)
		return result, nil
	}

	issues := []string{}

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if instances, ok := stpData["instances"].(map[string]interface{}); ok {
			for _, vlanID := range t.Vlans {
				vlanStr := fmt.Sprintf("%d", vlanID)
				if instance, ok := instances[vlanStr].(map[string]interface{}); ok {
					if interfaces, ok := instance["interfaces"].(map[string]interface{}); ok {
						for intfName, intfData := range interfaces {
							if intf, ok := intfData.(map[string]interface{}); ok {
								if state, ok := intf["state"].(string); ok {
									if strings.ToLower(state) != "forwarding" {
										issues = append(issues, fmt.Sprintf("Interface %s in VLAN %d is in '%s' state, expected 'forwarding'", intfName, vlanID, state))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP forwarding issues: %v", issues)
	}

	return result, nil
}

func (t *VerifySTPForwardingPorts) ValidateInput(input interface{}) error {
	if len(t.Vlans) == 0 {
		return fmt.Errorf("at least one VLAN must be specified")
	}
	return nil
}

// VerifySTPRootPriority verifies the STP root priority for provided VLAN or MST instance ID(s)
type VerifySTPRootPriority struct {
	test.BaseTest
	Priority  int   `yaml:"priority" json:"priority"`
	Instances []int `yaml:"instances,omitempty" json:"instances,omitempty"`
}

func NewVerifySTPRootPriority(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifySTPRootPriority{
		BaseTest: test.BaseTest{
			TestName:        "VerifySTPRootPriority",
			TestDescription: "Verify the STP root priority for provided VLAN or MST instance ID(s)",
			TestCategories:  []string{"stp", "root-priority"},
		},
	}

	if inputs != nil {
		if priority, ok := inputs["priority"].(float64); ok {
			t.Priority = int(priority)
		} else if priority, ok := inputs["priority"].(int); ok {
			t.Priority = priority
		}

		if instances, ok := inputs["instances"].([]interface{}); ok {
			for _, i := range instances {
				if instance, ok := i.(float64); ok {
					t.Instances = append(t.Instances, int(instance))
				} else if instance, ok := i.(int); ok {
					t.Instances = append(t.Instances, instance)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifySTPRootPriority) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP information: %v", err)
		return result, nil
	}

	issues := []string{}

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if instances, ok := stpData["instances"].(map[string]interface{}); ok {
			if len(t.Instances) > 0 {
				for _, instanceID := range t.Instances {
					instanceStr := fmt.Sprintf("%d", instanceID)
					if instance, ok := instances[instanceStr].(map[string]interface{}); ok {
						if rootBridge, ok := instance["rootBridge"].(map[string]interface{}); ok {
							if priority, ok := rootBridge["priority"].(float64); ok {
								if int(priority) != t.Priority {
									issues = append(issues, fmt.Sprintf("Instance %d root priority is %d, expected %d", instanceID, int(priority), t.Priority))
								}
							}
						}
					}
				}
			} else {
				// Check all instances if none specified
				for instanceStr, instanceData := range instances {
					if instance, ok := instanceData.(map[string]interface{}); ok {
						if rootBridge, ok := instance["rootBridge"].(map[string]interface{}); ok {
							if priority, ok := rootBridge["priority"].(float64); ok {
								if int(priority) != t.Priority {
									issues = append(issues, fmt.Sprintf("Instance %s root priority is %d, expected %d", instanceStr, int(priority), t.Priority))
								}
							}
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP root priority issues: %v", issues)
	}

	return result, nil
}

func (t *VerifySTPRootPriority) ValidateInput(input interface{}) error {
	if t.Priority < 0 || t.Priority > 65535 {
		return fmt.Errorf("priority must be between 0 and 65535")
	}
	return nil
}

// VerifyStpTopologyChanges verifies the number of topology changes is below a threshold
type VerifyStpTopologyChanges struct {
	test.BaseTest
	Threshold int `yaml:"threshold" json:"threshold"`
}

func NewVerifyStpTopologyChanges(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyStpTopologyChanges{
		BaseTest: test.BaseTest{
			TestName:        "VerifyStpTopologyChanges",
			TestDescription: "Verify the number of STP topology changes is below a threshold",
			TestCategories:  []string{"stp", "topology-changes"},
		},
		Threshold: 10, // Default threshold
	}

	if inputs != nil {
		if threshold, ok := inputs["threshold"].(float64); ok {
			t.Threshold = int(threshold)
		} else if threshold, ok := inputs["threshold"].(int); ok {
			t.Threshold = threshold
		}
	}

	return t, nil
}

func (t *VerifyStpTopologyChanges) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree counters",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP counters: %v", err)
		return result, nil
	}

	issues := []string{}
	totalChanges := 0

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if interfaces, ok := stpData["interfaces"].(map[string]interface{}); ok {
			for intfName, intfData := range interfaces {
				if intf, ok := intfData.(map[string]interface{}); ok {
					if counters, ok := intf["counters"].(map[string]interface{}); ok {
						if changes, ok := counters["topologyChanges"].(float64); ok {
							totalChanges += int(changes)
							if int(changes) > t.Threshold {
								issues = append(issues, fmt.Sprintf("Interface %s has %d topology changes (threshold: %d)", intfName, int(changes), t.Threshold))
							}
						}
					}
				}
			}
		}
	}

	if totalChanges > t.Threshold {
		issues = append(issues, fmt.Sprintf("Total topology changes %d exceeds threshold %d", totalChanges, t.Threshold))
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP topology change issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyStpTopologyChanges) ValidateInput(input interface{}) error {
	if t.Threshold < 0 {
		return fmt.Errorf("threshold must be non-negative")
	}
	return nil
}

// VerifySTPDisabledVlans verifies the STP disabled VLAN(s)
type VerifySTPDisabledVlans struct {
	test.BaseTest
	Vlans []int `yaml:"vlans" json:"vlans"`
}

func NewVerifySTPDisabledVlans(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifySTPDisabledVlans{
		BaseTest: test.BaseTest{
			TestName:        "VerifySTPDisabledVlans",
			TestDescription: "Verify the STP disabled VLAN(s)",
			TestCategories:  []string{"stp", "disabled-vlans"},
		},
	}

	if inputs != nil {
		if vlans, ok := inputs["vlans"].([]interface{}); ok {
			for _, v := range vlans {
				if vlan, ok := v.(float64); ok {
					t.Vlans = append(t.Vlans, int(vlan))
				} else if vlan, ok := v.(int); ok {
					t.Vlans = append(t.Vlans, vlan)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifySTPDisabledVlans) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show spanning-tree",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get STP information: %v", err)
		return result, nil
	}

	issues := []string{}

	if stpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		// Check if STP is disabled for specified VLANs
		for _, vlanID := range t.Vlans {
			vlanStr := fmt.Sprintf("%d", vlanID)

			// Check if VLAN exists in instances (if it does, STP is enabled)
			if instances, ok := stpData["instances"].(map[string]interface{}); ok {
				if _, exists := instances[vlanStr]; exists {
					issues = append(issues, fmt.Sprintf("VLAN %d has STP enabled, expected disabled", vlanID))
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("STP disabled VLAN issues: %v", issues)
	}

	return result, nil
}

func (t *VerifySTPDisabledVlans) ValidateInput(input interface{}) error {
	if len(t.Vlans) == 0 {
		return fmt.Errorf("at least one VLAN must be specified")
	}
	return nil
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}