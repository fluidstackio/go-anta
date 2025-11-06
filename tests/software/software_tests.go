package software

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluidstackio/go-anta/pkg/device"
	"github.com/fluidstackio/go-anta/pkg/test"
)

// VerifyTerminAttrVersion verifies the TerminAttr version of the device
type VerifyTerminAttrVersion struct {
	test.BaseTest
	Versions []string `yaml:"versions" json:"versions"`
}

func NewVerifyTerminAttrVersion(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyTerminAttrVersion{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTerminAttrVersion",
			TestDescription: "Verify the TerminAttr version of the device",
			TestCategories:  []string{"software", "terminattr"},
		},
	}

	if inputs != nil {
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

func (t *VerifyTerminAttrVersion) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	// First check if TerminAttr is running
	daemonCmd := device.Command{
		Template: "show daemon TerminAttr",
		Format:   "json",
		UseCache: false,
	}

	daemonResult, err := dev.Execute(ctx, daemonCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get TerminAttr daemon status: %v", err)
		return result, nil
	}

	var terminAttrRunning bool
	if daemonData, ok := daemonResult.Output.(map[string]interface{}); ok {
		if daemons, ok := daemonData["daemons"].(map[string]interface{}); ok {
			if terminAttr, ok := daemons["TerminAttr"].(map[string]interface{}); ok {
				if status, ok := terminAttr["status"].(string); ok {
					terminAttrRunning = strings.ToLower(status) == "running"
				}
			}
		}
	}

	if !terminAttrRunning {
		result.Status = test.TestFailure
		result.Message = "TerminAttr daemon is not running"
		return result, nil
	}

	// Get TerminAttr version
	versionCmd := device.Command{
		Template: "show version detail",
		Format:   "json",
		UseCache: false,
	}

	versionResult, err := dev.Execute(ctx, versionCmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get version details: %v", err)
		return result, nil
	}

	var currentVersion string
	if versionData, ok := versionResult.Output.(map[string]interface{}); ok {
		// Check for TerminAttr in details
		if details, ok := versionData["details"].(map[string]interface{}); ok {
			if terminAttr, ok := details["TerminAttr"].(map[string]interface{}); ok {
				if version, ok := terminAttr["version"].(string); ok {
					currentVersion = version
				}
			}
		}

		// Alternative: check in imageFormatVersion or similar fields
		if currentVersion == "" {
			if imageInfo, ok := versionData["imageFormatVersion"].(string); ok {
				// This might contain TerminAttr version info
				if strings.Contains(imageInfo, "TerminAttr") {
					// Parse version from string if needed
					currentVersion = imageInfo
				}
			}
		}

		// Check software images for TerminAttr
		if currentVersion == "" {
			if softwareImages, ok := versionData["softwareImages"].([]interface{}); ok {
				for _, image := range softwareImages {
					if img, ok := image.(map[string]interface{}); ok {
						if name, ok := img["name"].(string); ok && strings.Contains(name, "TerminAttr") {
							if version, ok := img["version"].(string); ok {
								currentVersion = version
								break
							}
						}
					}
				}
			}
		}
	}

	if currentVersion == "" {
		result.Status = test.TestError
		result.Message = "Could not determine TerminAttr version"
		return result, nil
	}

	// Check if current version is in the allowed list
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
			result.Message = fmt.Sprintf("TerminAttr version %s not in allowed list: %v", currentVersion, t.Versions)
			return result, nil
		}
	}

	result.Details = map[string]interface{}{
		"current_version":  currentVersion,
		"allowed_versions": t.Versions,
	}

	return result, nil
}

func (t *VerifyTerminAttrVersion) ValidateInput(input interface{}) error {
	if len(t.Versions) == 0 {
		return fmt.Errorf("at least one TerminAttr version must be specified")
	}

	for i, version := range t.Versions {
		if version == "" {
			return fmt.Errorf("version at index %d cannot be empty", i)
		}
	}

	return nil
}

// VerifyEOSExtensions verifies that all EOS extensions installed on the device are enabled for boot persistence
type VerifyEOSExtensions struct {
	test.BaseTest
}

func NewVerifyEOSExtensions(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyEOSExtensions{
		BaseTest: test.BaseTest{
			TestName:        "VerifyEOSExtensions",
			TestDescription: "Verify that all EOS extensions installed are enabled for boot persistence",
			TestCategories:  []string{"software", "extensions"},
		},
	}

	return t, nil
}

func (t *VerifyEOSExtensions) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show extensions",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get extensions information: %v", err)
		return result, nil
	}

	issues := []string{}
	extensionCount := 0
	bootPersistentCount := 0

	if extData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if extensions, ok := extData["extensions"].(map[string]interface{}); ok {
			for extName, extInfo := range extensions {
				extensionCount++

				if ext, ok := extInfo.(map[string]interface{}); ok {
					var status, presence string
					var bootPersistent bool

					if s, ok := ext["status"].(string); ok {
						status = s
					}
					if p, ok := ext["presence"].(string); ok {
						presence = p
					}
					if bp, ok := ext["bootPersistent"].(bool); ok {
						bootPersistent = bp
					}

					// Check if extension is installed/present
					if presence == "installed" || status == "installed" {
						if !bootPersistent {
							issues = append(issues, fmt.Sprintf("Extension %s is installed but not boot persistent", extName))
						} else {
							bootPersistentCount++
						}
					}
				}
			}
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("EOS extension issues: %v", issues)
	} else if extensionCount == 0 {
		// No extensions found - this could be normal
		result.Details = map[string]interface{}{
			"total_extensions":        0,
			"boot_persistent_count":   0,
			"message":                 "No EOS extensions found",
		}
	} else {
		result.Details = map[string]interface{}{
			"total_extensions":      extensionCount,
			"boot_persistent_count": bootPersistentCount,
		}
	}

	return result, nil
}

func (t *VerifyEOSExtensions) ValidateInput(input interface{}) error {
	return nil
}