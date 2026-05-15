package test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// CustomInputKeys lets a test report additional accepted top-level input
// keys beyond what reflection on its struct's yaml tags would discover.
// Useful for tests whose constructor reads aliased keys (e.g. accepts
// both "udp_port" and "udpPort") or whose fields aren't yaml-tagged for
// some other reason. Tests without unusual key shapes don't need to
// implement this — the default reflection-based discovery is enough.
type CustomInputKeys interface {
	InputKeys() []string
}

// ValidateInputKeys errors if `inputs` contains any top-level key that
// isn't reachable from the test struct's yaml tags. Catches the common
// silent-failure mode where a user mistypes an input key
// (`peer_addres:` instead of `peer_address:`) — the constructor would
// drop it silently and the test would either pass vacuously or fail
// with a misleading "no peers found" error.
//
// This is best-effort: it only checks the top-level inputs map. Typos
// nested inside list/map entries (e.g. inside `bgp_peers[i]`) are not
// caught here. Per-element validation belongs in the constructor or
// ValidateInput where the per-element schema is known.
func ValidateInputKeys(inputs map[string]any, t Test) error {
	if len(inputs) == 0 {
		return nil
	}

	allowed := collectYAMLKeys(t)
	if c, ok := t.(CustomInputKeys); ok {
		for _, k := range c.InputKeys() {
			allowed[k] = struct{}{}
		}
	}
	// If the struct exposes no yaml-tagged fields at all, refuse to
	// flag anything — these tests read inputs ad-hoc and a reflection
	// pass would have nothing to compare against.
	if len(allowed) == 0 {
		return nil
	}

	var unknown []string
	for k := range inputs {
		if _, ok := allowed[k]; !ok {
			unknown = append(unknown, k)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	allowedList := keysOf(allowed)
	sort.Strings(allowedList)
	return fmt.Errorf("unknown input key(s) %v; valid keys are: %v", unknown, allowedList)
}

// collectYAMLKeys walks the test struct's exported fields and collects
// the yaml tag name for each. Embedded structs are flattened only when
// their fields contribute yaml-tagged input keys — BaseTest itself
// carries no yaml tags so it contributes nothing, which is what we
// want (TestName/Description/Categories must not appear under inputs).
func collectYAMLKeys(t Test) map[string]struct{} {
	out := map[string]struct{}{}
	collectFromValue(reflect.ValueOf(t), out)
	return out
}

func collectFromValue(v reflect.Value, out map[string]struct{}) {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	rt := v.Type()
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		if f.Anonymous {
			// Embedded BaseTest carries framework metadata (name,
			// description, categories, commands) — those live in the
			// outer test-definition YAML, not in the inputs block, so
			// they must not become accepted input keys.
			if f.Type.Name() == "BaseTest" {
				continue
			}
			// Other embedded structs flatten as yaml.v3 would.
			collectFromValue(v.Field(i), out)
			continue
		}
		tag := f.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
}

func keysOf(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Input typed-getter helpers.
//
// Background: most NewVerify* constructors today write the equivalent of
//
//	if v, ok := inputs["foo"].(float64); ok { t.Foo = int(v) }
//	else if v, ok := inputs["foo"].(int);     ok { t.Foo = v }
//
// which silently drops the key when the user wrote `foo: "three"`
// (string). The two paths below surface that as an error so the test
// reports it instead of running with a zero value. New tests should
// prefer these helpers; existing ad-hoc reads still work.

// GetInt reads inputs[key] into *dst. If the key is absent, dst is
// left untouched. If the key is present but neither an int nor a
// float64, returns an error naming the key and observed type.
func GetInt(inputs map[string]any, key string, dst *int) error {
	raw, ok := inputs[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case float64:
		*dst = int(v)
	case int:
		*dst = v
	case int64:
		*dst = int(v)
	default:
		return fmt.Errorf("%s: expected number, got %T", key, raw)
	}
	return nil
}

// GetString reads inputs[key] into *dst. Same semantics as GetInt.
func GetString(inputs map[string]any, key string, dst *string) error {
	raw, ok := inputs[key]
	if !ok {
		return nil
	}
	s, ok := raw.(string)
	if !ok {
		return fmt.Errorf("%s: expected string, got %T", key, raw)
	}
	*dst = s
	return nil
}

// GetBool reads inputs[key] into *dst.
func GetBool(inputs map[string]any, key string, dst *bool) error {
	raw, ok := inputs[key]
	if !ok {
		return nil
	}
	b, ok := raw.(bool)
	if !ok {
		return fmt.Errorf("%s: expected bool, got %T", key, raw)
	}
	*dst = b
	return nil
}

// GetStringSlice reads inputs[key] as a list of strings into *dst.
func GetStringSlice(inputs map[string]any, key string, dst *[]string) error {
	raw, ok := inputs[key]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("%s: expected list, got %T", key, raw)
	}
	out := make([]string, 0, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return fmt.Errorf("%s[%d]: expected string, got %T", key, i, item)
		}
		out = append(out, s)
	}
	*dst = out
	return nil
}
