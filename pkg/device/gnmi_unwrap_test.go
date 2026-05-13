package device

import (
	"reflect"
	"testing"
)

func TestUnwrapCLIResponse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		command string
		want    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "strips matching command-name wrapper",
			raw:     `{"show version":{"modelName":"DCS-7280","version":"4.34.4M"}}`,
			command: "show version",
			want:    map[string]interface{}{"modelName": "DCS-7280", "version": "4.34.4M"},
		},
		{
			name:    "leaves unwrapped JSON alone",
			raw:     `{"modelName":"DCS-7280","version":"4.34.4M"}`,
			command: "show version",
			want:    map[string]interface{}{"modelName": "DCS-7280", "version": "4.34.4M"},
		},
		{
			name:    "wrapper key that does not match command is preserved",
			raw:     `{"unexpected":{"x":1}}`,
			command: "show version",
			want:    map[string]interface{}{"unexpected": map[string]interface{}{"x": float64(1)}},
		},
		{
			name:    "multi-key top level is not unwrapped",
			raw:     `{"show version":{"x":1},"extra":2}`,
			command: "show version",
			want:    map[string]interface{}{"show version": map[string]interface{}{"x": float64(1)}, "extra": float64(2)},
		},
		{
			name:    "wrapper value that is not an object is preserved",
			raw:     `{"show version":"raw text"}`,
			command: "show version",
			want:    map[string]interface{}{"show version": "raw text"},
		},
		{
			name:    "invalid JSON returns error",
			raw:     `not json`,
			command: "show version",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unwrapCLIResponse([]byte(tc.raw), tc.command)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil; result=%v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
