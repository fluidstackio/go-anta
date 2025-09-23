package system

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyUptime struct {
	test.BaseTest
	MinimumUptime int64 `yaml:"minimum_uptime" json:"minimum_uptime"`
}

func NewVerifyUptime(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyUptime{
		BaseTest: test.BaseTest{
			TestName:        "VerifyUptime",
			TestDescription: "Verify system uptime meets minimum requirements",
			TestCategories:  []string{"system", "availability"},
		},
		MinimumUptime: 3600,
	}

	if inputs != nil {
		if uptime, ok := inputs["minimum_uptime"].(float64); ok {
			t.MinimumUptime = int64(uptime)
		} else if uptime, ok := inputs["minimum_uptime"].(int); ok {
			t.MinimumUptime = int64(uptime)
		}
	}

	return t, nil
}

func (t *VerifyUptime) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
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
		result.Message = fmt.Sprintf("Failed to get uptime information: %v", err)
		return result, nil
	}

	var uptimeSeconds int64
	if versionData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if uptime, ok := versionData["uptime"].(float64); ok {
			uptimeSeconds = int64(uptime)
		}
	}

	if uptimeSeconds == 0 {
		result.Status = test.TestError
		result.Message = "Could not determine system uptime"
		return result, nil
	}

	uptimeHours := uptimeSeconds / 3600
	uptimeDays := uptimeHours / 24

	if uptimeSeconds < t.MinimumUptime {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Uptime %d seconds (%.1f hours) is below minimum %d seconds",
			uptimeSeconds, float64(uptimeSeconds)/3600, t.MinimumUptime)
	} else {
		result.Details = map[string]interface{}{
			"uptime_seconds": uptimeSeconds,
			"uptime_hours":   uptimeHours,
			"uptime_days":    uptimeDays,
		}
	}

	return result, nil
}

func (t *VerifyUptime) ValidateInput(input interface{}) error {
	if t.MinimumUptime < 0 {
		return fmt.Errorf("minimum uptime cannot be negative")
	}
	return nil
}