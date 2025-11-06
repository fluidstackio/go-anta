package interfaces

import (
	"context"
	"fmt"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyInterfaceErrors verifies that interface error counters are within acceptable limits.
//
// Expected Results:
//   - Success: The test will pass if all interface error counters are within the specified thresholds.
//   - Failure: The test will fail if any interface has error counters above the specified thresholds.
//   - Error: The test will report an error if interface error statistics cannot be retrieved.
//
// Examples:
//   - name: VerifyInterfaceErrors with specific thresholds
//     VerifyInterfaceErrors:
//       interfaces:
//         - name: "Ethernet1/1"
//           fcs_errors: 0
//           in_errors: 100  # Allow some input errors
//         - name: "Ethernet2/1"
//           symbol_errors: 5
//
//   - name: VerifyInterfaceErrors check all interfaces
//     VerifyInterfaceErrors:
//       check_all: true  # Check all interfaces with default thresholds (0)
type VerifyInterfaceErrors struct {
	test.BaseTest
	Interfaces []InterfaceErrorThresholds `yaml:"interfaces,omitempty" json:"interfaces,omitempty"`
	CheckAll   bool                       `yaml:"check_all,omitempty" json:"check_all,omitempty"`
}

type InterfaceErrorThresholds struct {
	Name             string `yaml:"name" json:"name"`
	FcsErrors        int    `yaml:"fcs_errors,omitempty" json:"fcs_errors,omitempty"`
	AlignmentErrors  int    `yaml:"alignment_errors,omitempty" json:"alignment_errors,omitempty"`
	SymbolErrors     int    `yaml:"symbol_errors,omitempty" json:"symbol_errors,omitempty"`
	InErrors         int    `yaml:"in_errors,omitempty" json:"in_errors,omitempty"`
	OutErrors        int    `yaml:"out_errors,omitempty" json:"out_errors,omitempty"`
	FrameTooShorts   int    `yaml:"frame_too_shorts,omitempty" json:"frame_too_shorts,omitempty"`
	FrameTooLongs    int    `yaml:"frame_too_longs,omitempty" json:"frame_too_longs,omitempty"`
}

func NewVerifyInterfaceErrors(inputs map[string]any) (test.Test, error) {
	t := &VerifyInterfaceErrors{
		BaseTest: test.BaseTest{
			TestName:        "VerifyInterfaceErrors",
			TestDescription: "Verify interface error counters are within acceptable limits",
			TestCategories:  []string{"interfaces", "errors"},
		},
		CheckAll: true, // Default to checking all interfaces
	}

	if inputs != nil {
		if checkAll, ok := inputs["check_all"].(bool); ok {
			t.CheckAll = checkAll
		}

		if interfaces, ok := inputs["interfaces"].([]any); ok {
			t.CheckAll = false // If specific interfaces provided, don't check all
			for _, i := range interfaces {
				if intfMap, ok := i.(map[string]any); ok {
					intf := InterfaceErrorThresholds{}

					if name, ok := intfMap["name"].(string); ok {
						intf.Name = name
					}
					if fcs, ok := intfMap["fcs_errors"].(float64); ok {
						intf.FcsErrors = int(fcs)
					}
					if align, ok := intfMap["alignment_errors"].(float64); ok {
						intf.AlignmentErrors = int(align)
					}
					if symbol, ok := intfMap["symbol_errors"].(float64); ok {
						intf.SymbolErrors = int(symbol)
					}
					if inErr, ok := intfMap["in_errors"].(float64); ok {
						intf.InErrors = int(inErr)
					}
					if outErr, ok := intfMap["out_errors"].(float64); ok {
						intf.OutErrors = int(outErr)
					}
					if tooShorts, ok := intfMap["frame_too_shorts"].(float64); ok {
						intf.FrameTooShorts = int(tooShorts)
					}
					if tooLongs, ok := intfMap["frame_too_longs"].(float64); ok {
						intf.FrameTooLongs = int(tooLongs)
					}

					t.Interfaces = append(t.Interfaces, intf)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyInterfaceErrors) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show interfaces counters errors",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get interface error counters: %v", err)
		return result, nil
	}

	deviceErrors := make(map[string]InterfaceErrorCounters)

	if errorData, ok := cmdResult.Output.(map[string]any); ok {
		if counters, ok := errorData["interfaceErrorCounters"].(map[string]any); ok {
			for intfName, intfCounters := range counters {
				if counters, ok := intfCounters.(map[string]any); ok {
					errorCounters := InterfaceErrorCounters{
						Interface: intfName,
					}

					if fcs, ok := counters["fcsErrors"].(float64); ok {
						errorCounters.FcsErrors = int(fcs)
					}
					if align, ok := counters["alignmentErrors"].(float64); ok {
						errorCounters.AlignmentErrors = int(align)
					}
					if symbol, ok := counters["symbolErrors"].(float64); ok {
						errorCounters.SymbolErrors = int(symbol)
					}
					if inErr, ok := counters["inErrors"].(float64); ok {
						errorCounters.InErrors = int(inErr)
					}
					if outErr, ok := counters["outErrors"].(float64); ok {
						errorCounters.OutErrors = int(outErr)
					}
					if tooShorts, ok := counters["frameTooShorts"].(float64); ok {
						errorCounters.FrameTooShorts = int(tooShorts)
					}
					if tooLongs, ok := counters["frameTooLongs"].(float64); ok {
						errorCounters.FrameTooLongs = int(tooLongs)
					}

					deviceErrors[intfName] = errorCounters
				}
			}
		}
	}

	failures := []string{}

	if t.CheckAll && len(t.Interfaces) == 0 {
		// Check all interfaces with default thresholds (0 for all error types)
		for intfName, counters := range deviceErrors {
			if failures := t.checkInterfaceErrors(intfName, counters, InterfaceErrorThresholds{Name: intfName}); len(failures) > 0 {
				for _, failure := range failures {
					failures = append(failures, failure)
				}
			}
		}
	} else {
		// Check specified interfaces
		for _, expectedIntf := range t.Interfaces {
			if deviceCounter, found := deviceErrors[expectedIntf.Name]; found {
				if intfFailures := t.checkInterfaceErrors(expectedIntf.Name, deviceCounter, expectedIntf); len(intfFailures) > 0 {
					failures = append(failures, intfFailures...)
				}
			} else {
				failures = append(failures, fmt.Sprintf("Interface %s not found", expectedIntf.Name))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Interface error counter failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyInterfaceErrors) checkInterfaceErrors(intfName string, actual InterfaceErrorCounters, thresholds InterfaceErrorThresholds) []string {
	failures := []string{}

	if actual.FcsErrors > thresholds.FcsErrors {
		failures = append(failures, fmt.Sprintf("%s: FCS errors %d > %d", intfName, actual.FcsErrors, thresholds.FcsErrors))
	}
	if actual.AlignmentErrors > thresholds.AlignmentErrors {
		failures = append(failures, fmt.Sprintf("%s: alignment errors %d > %d", intfName, actual.AlignmentErrors, thresholds.AlignmentErrors))
	}
	if actual.SymbolErrors > thresholds.SymbolErrors {
		failures = append(failures, fmt.Sprintf("%s: symbol errors %d > %d", intfName, actual.SymbolErrors, thresholds.SymbolErrors))
	}
	if actual.InErrors > thresholds.InErrors {
		failures = append(failures, fmt.Sprintf("%s: input errors %d > %d", intfName, actual.InErrors, thresholds.InErrors))
	}
	if actual.OutErrors > thresholds.OutErrors {
		failures = append(failures, fmt.Sprintf("%s: output errors %d > %d", intfName, actual.OutErrors, thresholds.OutErrors))
	}
	if actual.FrameTooShorts > thresholds.FrameTooShorts {
		failures = append(failures, fmt.Sprintf("%s: frame too shorts %d > %d", intfName, actual.FrameTooShorts, thresholds.FrameTooShorts))
	}
	if actual.FrameTooLongs > thresholds.FrameTooLongs {
		failures = append(failures, fmt.Sprintf("%s: frame too longs %d > %d", intfName, actual.FrameTooLongs, thresholds.FrameTooLongs))
	}

	return failures
}

func (t *VerifyInterfaceErrors) ValidateInput(input any) error {
	if !t.CheckAll && len(t.Interfaces) == 0 {
		return fmt.Errorf("either check_all must be true or specific interfaces must be provided")
	}

	for i, intf := range t.Interfaces {
		if intf.Name == "" {
			return fmt.Errorf("interface at index %d has no name", i)
		}
	}

	return nil
}

type InterfaceErrorCounters struct {
	Interface       string
	FcsErrors       int
	AlignmentErrors int
	SymbolErrors    int
	InErrors        int
	OutErrors       int
	FrameTooShorts  int
	FrameTooLongs   int
}