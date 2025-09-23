package test

import (
	"context"
	"time"

	"github.com/gavmckee/go-anta/internal/device"
)

type Test interface {
	Name() string
	Description() string
	Categories() []string
	Commands() []device.Command
	Execute(ctx context.Context, dev device.Device) (*TestResult, error)
	ValidateInput(input interface{}) error
}

type TestStatus int

const (
	TestUnset TestStatus = iota
	TestSuccess
	TestFailure
	TestError
	TestSkipped
)

func (s TestStatus) String() string {
	switch s {
	case TestSuccess:
		return "success"
	case TestFailure:
		return "failure"
	case TestError:
		return "error"
	case TestSkipped:
		return "skipped"
	default:
		return "unset"
	}
}

type TestResult struct {
	TestName    string        `json:"test_name"`
	DeviceName  string        `json:"device_name"`
	Status      TestStatus    `json:"status"`
	Message     string        `json:"message,omitempty"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	Categories  []string      `json:"categories"`
	CustomField string        `json:"custom_field,omitempty"`
	Details     interface{}   `json:"details,omitempty"`
}

type BaseTest struct {
	TestName        string   `yaml:"name" json:"name"`
	TestDescription string   `yaml:"description" json:"description"`
	TestCategories  []string `yaml:"categories" json:"categories"`
	TestCommands    []device.Command `yaml:"commands" json:"commands"`
}

func (t *BaseTest) Name() string {
	return t.TestName
}

func (t *BaseTest) Description() string {
	return t.TestDescription
}

func (t *BaseTest) Categories() []string {
	return t.TestCategories
}

func (t *BaseTest) Commands() []device.Command {
	return t.TestCommands
}

type TestDefinition struct {
	Name       string                 `yaml:"name" json:"name"`
	Module     string                 `yaml:"module" json:"module"`
	Inputs     map[string]interface{} `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Categories []string               `yaml:"categories,omitempty" json:"categories,omitempty"`
	Tags       []string               `yaml:"tags,omitempty" json:"tags,omitempty"`
}