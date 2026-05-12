package device

// Anchor imports so go mod tidy keeps these direct deps in go.mod while
// Task 4 implements the actual GNMIDevice.
// Remove once gnmi.go imports these packages directly.
import (
	_ "github.com/openconfig/gnmi/proto/gnmi"
	_ "github.com/openconfig/gnmic/pkg/api"
)
