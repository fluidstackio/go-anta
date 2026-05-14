package test

import "fmt"

// AsMap asserts that a device command's Output is a JSON object
// (map[string]interface{}) and returns it. On any mismatch it returns
// a descriptive error.
//
// Most tests need to walk JSON-decoded device output to make assertions.
// Without an explicit check, the common idiom
//
//	if data, ok := cmdResult.Output.(map[string]any); ok { ...checks... }
//
// silently skips every check when the output shape is unexpected,
// leaving result.Status at its default TestSuccess — a false positive.
// Tests should call AsMap at the top of their parse pass and surface
// errors as TestError so unexpected device responses are visible.
func AsMap(out interface{}) (map[string]interface{}, error) {
	if out == nil {
		return nil, fmt.Errorf("device output is nil")
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected JSON object, got %T", out)
	}
	return m, nil
}

