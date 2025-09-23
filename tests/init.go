package tests

import (
	"github.com/gavmckee/go-anta/internal/test"
	"github.com/gavmckee/go-anta/tests/connectivity"
	"github.com/gavmckee/go-anta/tests/hardware"
	"github.com/gavmckee/go-anta/tests/routing"
	"github.com/gavmckee/go-anta/tests/system"
)

func init() {
	RegisterAllTests()
}

func RegisterAllTests() {
	registry := test.GetRegistry()

	_ = registry.Register("connectivity", "VerifyReachability", connectivity.NewVerifyReachability)
	_ = registry.Register("connectivity", "VerifyLLDPNeighbors", connectivity.NewVerifyLLDPNeighbors)

	_ = registry.Register("hardware", "VerifyTemperature", hardware.NewVerifyTemperature)
	_ = registry.Register("hardware", "VerifyTransceivers", hardware.NewVerifyTransceivers)
	_ = registry.Register("hardware", "VerifyInventory", hardware.NewVerifyInventory)
	_ = registry.Register("hardware", "VerifyPowerSupplies", NewVerifyPowerSupplies)

	_ = registry.Register("routing", "VerifyBGPPeers", routing.NewVerifyBGPPeers)
	_ = registry.Register("routing", "VerifyOSPFNeighbors", routing.NewVerifyOSPFNeighbors)
	_ = registry.Register("routing", "VerifyStaticRoutes", routing.NewVerifyStaticRoutes)

	_ = registry.Register("system", "VerifyEOSVersion", system.NewVerifyEOSVersion)
	_ = registry.Register("system", "VerifyUptime", system.NewVerifyUptime)
	_ = registry.Register("system", "VerifyNTP", system.NewVerifyNTP)
	_ = registry.Register("system", "VerifyDNSResolution", NewVerifyDNSResolution)
}

func NewVerifyPowerSupplies(inputs map[string]interface{}) (test.Test, error) {
	return hardware.NewVerifyInventory(inputs)
}

func NewVerifyDNSResolution(inputs map[string]interface{}) (test.Test, error) {
	return system.NewVerifyNTP(inputs)
}