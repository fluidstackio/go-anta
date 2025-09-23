package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyEOSVersion struct {
	test.BaseTest
	MinimumVersion string   `yaml:"minimum_version,omitempty" json:"minimum_version,omitempty"`
	Versions       []string `yaml:"versions,omitempty" json:"versions,omitempty"`
}

func NewVerifyEOSVersion(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyEOSVersion{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEOSVersion",
			TestDescription: "Verify EOS software version meets requirements",
			TestCategories:  []string{"system", "software"},
		},
	}

	if inputs != nil {
		if minVer, ok := inputs["minimum_version"].(string); ok {
			t.MinimumVersion = minVer
		}
		if versions, ok := inputs["versions"].([]interface{}); ok {
			for _, v := range versions {
				if version, ok := v.(string); ok {
					t.Versions = append(t.Versions, version)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyEOSVersion) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
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
		result.Message = fmt.Sprintf("Failed to get version information: %v", err)
		return result, nil
	}

	currentVersion := ""
	if versionData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if version, ok := versionData["version"].(string); ok {
			currentVersion = version
		}
	}

	if currentVersion == "" {
		result.Status = test.TestError
		result.Message = "Could not determine current EOS version"
		return result, nil
	}

	if len(t.Versions) > 0 {
		found := false
		for _, allowedVersion := range t.Versions {
			if currentVersion == allowedVersion {
				found = true
				break
			}
		}
		if !found {
			result.Status = test.TestFailure
			result.Message = fmt.Sprintf("Version %s not in allowed list: %v", currentVersion, t.Versions)
			return result, nil
		}
	}

	if t.MinimumVersion != "" {
		if !isVersionGreaterOrEqual(currentVersion, t.MinimumVersion) {
			result.Status = test.TestFailure
			result.Message = fmt.Sprintf("Version %s is below minimum required version %s", 
				currentVersion, t.MinimumVersion)
			return result, nil
		}
	}

	result.Details = map[string]string{
		"current_version": currentVersion,
	}

	return result, nil
}

func (t *VerifyEOSVersion) ValidateInput(input interface{}) error {
	if t.MinimumVersion == "" && len(t.Versions) == 0 {
		return fmt.Errorf("either minimum_version or versions list must be specified")
	}
	return nil
}

func isVersionGreaterOrEqual(current, minimum string) bool {
	currentParts := parseVersion(current)
	minimumParts := parseVersion(minimum)

	for i := 0; i < len(minimumParts); i++ {
		if i >= len(currentParts) {
			return false
		}
		if currentParts[i] > minimumParts[i] {
			return true
		}
		if currentParts[i] < minimumParts[i] {
			return false
		}
	}
	return true
}

func parseVersion(version string) []int {
	version = strings.TrimSuffix(version, "F")
	version = strings.TrimSuffix(version, "M")
	
	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))
	
	for _, part := range parts {
		var num int
		fmt.Sscanf(part, "%d", &num)
		result = append(result, num)
	}
	
	return result
}