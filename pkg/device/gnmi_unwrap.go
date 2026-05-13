package device

import (
	"encoding/json"
	"fmt"
)

// unwrapCLIResponse decodes JSON_IETF bytes returned by an Arista gNMI
// origin=cli Get and strips the single-key command-name wrapper if
// present, so the resulting map matches the eAPI JSON shape exactly.
//
// Arista wraps CLI responses like:
//
//	{"show version": {"modelName": "...", ...}}
//
// while eAPI returns the inner object directly inside result[0]. Both
// transports should produce identical Output values so test impls can
// remain transport-agnostic.
//
// If the JSON has a single top-level key matching the expanded command,
// the inner object is returned. Otherwise the parsed object is returned
// as-is.
func unwrapCLIResponse(raw []byte, expandedCommand string) (map[string]interface{}, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse gNMI JSON_IETF response: %w", err)
	}
	if len(parsed) != 1 {
		return parsed, nil
	}
	inner, ok := parsed[expandedCommand]
	if !ok {
		return parsed, nil
	}
	innerMap, ok := inner.(map[string]interface{})
	if !ok {
		return parsed, nil
	}
	return innerMap, nil
}
