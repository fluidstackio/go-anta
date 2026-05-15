package test

import (
	"context"
	"strings"
	"testing"

	"github.com/fluidstackio/go-anta/pkg/device"
)

// Local fakes — exercising ValidateInputKeys without registering real tests.

type fakeNoFields struct{ BaseTest }

func (f *fakeNoFields) Execute(_ context.Context, _ device.Device) (*TestResult, error) {
	return nil, nil
}
func (f *fakeNoFields) ValidateInput(_ any) error { return nil }

type fakeWithFields struct {
	BaseTest
	Hosts  []string `yaml:"hosts" json:"hosts"`
	Repeat int      `yaml:"repeat,omitempty"`
	NoTag  string   // no yaml tag — must not contribute
}

func (f *fakeWithFields) Execute(_ context.Context, _ device.Device) (*TestResult, error) {
	return nil, nil
}
func (f *fakeWithFields) ValidateInput(_ any) error { return nil }

type fakeWithCustom struct {
	BaseTest
	Hosts []string `yaml:"hosts"`
}

func (f *fakeWithCustom) Execute(_ context.Context, _ device.Device) (*TestResult, error) {
	return nil, nil
}
func (f *fakeWithCustom) ValidateInput(_ any) error { return nil }
func (f *fakeWithCustom) InputKeys() []string       { return []string{"udp_port"} }

func TestValidateInputKeys_NoFieldsSkips(t *testing.T) {
	// A test with no yaml-tagged fields shouldn't reject arbitrary
	// inputs — there's nothing to validate against, and rejecting
	// would break ad-hoc inputs constructors.
	err := ValidateInputKeys(map[string]any{"foo": 1, "bar": 2}, &fakeNoFields{})
	if err != nil {
		t.Errorf("expected nil for no-fields test, got %v", err)
	}
}

func TestValidateInputKeys_AcceptsTaggedKeys(t *testing.T) {
	err := ValidateInputKeys(map[string]any{"hosts": []any{"a"}, "repeat": 3}, &fakeWithFields{})
	if err != nil {
		t.Errorf("expected nil for valid keys, got %v", err)
	}
}

func TestValidateInputKeys_RejectsTypo(t *testing.T) {
	err := ValidateInputKeys(map[string]any{"hostz": []any{"a"}}, &fakeWithFields{})
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
	if !strings.Contains(err.Error(), "hostz") {
		t.Errorf("error should name the typo, got: %v", err)
	}
	if !strings.Contains(err.Error(), "hosts") {
		t.Errorf("error should list the valid keys, got: %v", err)
	}
}

func TestValidateInputKeys_IgnoresUntagged(t *testing.T) {
	// Fields without a yaml tag must not become accepted keys.
	err := ValidateInputKeys(map[string]any{"NoTag": "x"}, &fakeWithFields{})
	if err == nil {
		t.Error("expected error for untagged field name used as input key, got nil")
	}
}

func TestValidateInputKeys_CustomKeysAccepted(t *testing.T) {
	err := ValidateInputKeys(map[string]any{"hosts": []any{}, "udp_port": 4789}, &fakeWithCustom{})
	if err != nil {
		t.Errorf("expected nil with InputKeys() extension, got %v", err)
	}
}

func TestValidateInputKeys_EmptyInputsAlwaysOK(t *testing.T) {
	if err := ValidateInputKeys(nil, &fakeWithFields{}); err != nil {
		t.Errorf("nil inputs should not error, got %v", err)
	}
	if err := ValidateInputKeys(map[string]any{}, &fakeWithFields{}); err != nil {
		t.Errorf("empty inputs should not error, got %v", err)
	}
}

func TestGetInt(t *testing.T) {
	var got int
	if err := GetInt(map[string]any{"n": float64(5)}, "n", &got); err != nil || got != 5 {
		t.Errorf("float64 5 → got %d err %v", got, err)
	}
	got = 0
	if err := GetInt(map[string]any{"n": 5}, "n", &got); err != nil || got != 5 {
		t.Errorf("int 5 → got %d err %v", got, err)
	}
	got = 99
	if err := GetInt(map[string]any{}, "n", &got); err != nil || got != 99 {
		t.Errorf("absent key should leave dst alone, got %d err %v", got, err)
	}
	got = 0
	if err := GetInt(map[string]any{"n": "five"}, "n", &got); err == nil {
		t.Error("string should reject")
	} else if !strings.Contains(err.Error(), "n:") || !strings.Contains(err.Error(), "string") {
		t.Errorf("error should name key and observed type, got %v", err)
	}
}

func TestGetString(t *testing.T) {
	var got string
	if err := GetString(map[string]any{"s": "x"}, "s", &got); err != nil || got != "x" {
		t.Errorf("string x → got %q err %v", got, err)
	}
	got = ""
	if err := GetString(map[string]any{"s": 42}, "s", &got); err == nil {
		t.Error("int should reject for string field")
	}
}

func TestGetBool(t *testing.T) {
	var got bool
	if err := GetBool(map[string]any{"b": true}, "b", &got); err != nil || got != true {
		t.Errorf("true → got %v err %v", got, err)
	}
	got = true
	if err := GetBool(map[string]any{}, "b", &got); err != nil || got != true {
		t.Errorf("absent key should leave dst alone, got %v err %v", got, err)
	}
	if err := GetBool(map[string]any{"b": "yes"}, "b", &got); err == nil {
		t.Error("string should reject for bool field")
	}
}

func TestGetStringSlice(t *testing.T) {
	var got []string
	err := GetStringSlice(map[string]any{"xs": []any{"a", "b"}}, "xs", &got)
	if err != nil || len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("expected [a b], got %v err %v", got, err)
	}
	got = nil
	err = GetStringSlice(map[string]any{"xs": []any{"a", 42}}, "xs", &got)
	if err == nil {
		t.Error("mixed types should reject")
	} else if !strings.Contains(err.Error(), "xs[1]") {
		t.Errorf("error should name the bad index, got %v", err)
	}
}
