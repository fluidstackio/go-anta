package platform

import (
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VirtualPlatforms defines the list of virtual/lab platforms that should skip hardware tests
// Matches Python ANTA skip_on_platforms decorator: ["cEOSLab", "vEOS-lab", "cEOSCloudLab", "vEOS"]
var VirtualPlatforms = []string{
	"cEOSLab",
	"vEOS-lab",
	"cEOSCloudLab",
	"vEOS",
}

// IsVirtualPlatform checks if the given hardware model represents a virtual/lab platform
func IsVirtualPlatform(hardwareModel string) bool {
	for _, platform := range VirtualPlatforms {
		if strings.Contains(hardwareModel, platform) {
			return true
		}
	}
	return false
}

// SkipOnVirtualPlatforms creates a TestResult with TestSkipped status if running on virtual platforms
// Returns nil if the platform is not virtual (test should continue)
// Returns a TestResult with TestSkipped status if the platform is virtual (test should be skipped)
func SkipOnVirtualPlatforms(dev device.Device, testName string, categories []string, reason string) *test.TestResult {
	if !IsVirtualPlatform(dev.HardwareModel()) {
		return nil // Don't skip, continue with test
	}

	// Default reason if none provided
	if reason == "" {
		reason = "not supported on virtual platforms"
	}

	return &test.TestResult{
		TestName:   testName,
		DeviceName: dev.Name(),
		Status:     test.TestSkipped,
		Categories: categories,
		Message:    fmt.Sprintf("Test skipped: %s (platform: %s)", reason, dev.HardwareModel()),
	}
}

// SkipOnSpecificPlatforms creates a TestResult with TestSkipped status if running on specified platforms
// Returns nil if the platform is not in the skip list (test should continue)
// Returns a TestResult with TestSkipped status if the platform should be skipped
func SkipOnSpecificPlatforms(dev device.Device, testName string, categories []string, skipPlatforms []string, reason string) *test.TestResult {
	hardwareModel := dev.HardwareModel()

	for _, platform := range skipPlatforms {
		if strings.Contains(hardwareModel, platform) {
			// Default reason if none provided
			if reason == "" {
				reason = fmt.Sprintf("not supported on platform %s", platform)
			}

			return &test.TestResult{
				TestName:   testName,
				DeviceName: dev.Name(),
				Status:     test.TestSkipped,
				Categories: categories,
				Message:    fmt.Sprintf("Test skipped: %s (platform: %s)", reason, hardwareModel),
			}
		}
	}

	return nil // Don't skip, continue with test
}