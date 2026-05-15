package interfaces

import (
	"testing"
)

func TestHasErrors(t *testing.T) {
	if (InterfaceErrorCounters{Interface: "Et1"}).hasErrors() {
		t.Error("zero counters should report no errors")
	}
	if !(InterfaceErrorCounters{Interface: "Et1", FcsErrors: 1}).hasErrors() {
		t.Error("non-zero FCS should flag")
	}
	if !(InterfaceErrorCounters{Interface: "Et1", InErrors: 1}).hasErrors() {
		t.Error("non-zero InErrors should flag")
	}
}
