package tests

import (
	"github.com/fluidstack/go-anta/pkg/test"
	"github.com/fluidstack/go-anta/tests/connectivity"
	"github.com/fluidstack/go-anta/tests/evpn"
	"github.com/fluidstack/go-anta/tests/hardware"
	"github.com/fluidstack/go-anta/tests/interfaces"
	"github.com/fluidstack/go-anta/tests/logging"
	"github.com/fluidstack/go-anta/tests/routing"
	"github.com/fluidstack/go-anta/tests/security"
	"github.com/fluidstack/go-anta/tests/services"
	"github.com/fluidstack/go-anta/tests/software"
	"github.com/fluidstack/go-anta/tests/stp"
	"github.com/fluidstack/go-anta/tests/system"
	"github.com/fluidstack/go-anta/tests/vlan"
	"github.com/fluidstack/go-anta/tests/vxlan"
)

func init() {
	RegisterAllTests()
}

func RegisterAllTests() {
	registry := test.GetRegistry()

	_ = registry.Register("connectivity", "VerifyReachability", connectivity.NewVerifyReachability)
	_ = registry.Register("connectivity", "VerifyLLDPNeighbors", connectivity.NewVerifyLLDPNeighbors)

	// EVPN Tests
	_ = registry.Register("evpn", "VerifyEVPNType5Routes", evpn.NewVerifyEVPNType5Routes)

	// Hardware Tests - All hardware tests from ANTA Python implementation
	_ = registry.Register("hardware", "VerifyTemperature", hardware.NewVerifyTemperature)
	_ = registry.Register("hardware", "VerifyTransceivers", hardware.NewVerifyTransceivers)
	_ = registry.Register("hardware", "VerifyTransceiversManufacturers", hardware.NewVerifyTransceiversManufacturers)
	_ = registry.Register("hardware", "VerifyTransceiversTemperature", hardware.NewVerifyTransceiversTemperature)
	_ = registry.Register("hardware", "VerifyInventory", hardware.NewVerifyInventory)
	_ = registry.Register("hardware", "VerifyPowerSupplies", NewVerifyPowerSupplies)
	_ = registry.Register("hardware", "VerifyUnifiedForwardingTableMode", hardware.NewVerifyUnifiedForwardingTableMode)
	_ = registry.Register("hardware", "VerifyTcamProfile", hardware.NewVerifyTcamProfile)

	// Environment Tests
	_ = registry.Register("hardware", "VerifyEnvironmentSystemCooling", hardware.NewVerifyEnvironmentSystemCooling)
	_ = registry.Register("hardware", "VerifyEnvironmentCooling", hardware.NewVerifyEnvironmentCooling)
	_ = registry.Register("hardware", "VerifyEnvironmentPower", hardware.NewVerifyEnvironmentPower)

	// Advanced Hardware Tests
	_ = registry.Register("hardware", "VerifyAdverseDrops", hardware.NewVerifyAdverseDrops)
	_ = registry.Register("hardware", "VerifySupervisorRedundancy", hardware.NewVerifySupervisorRedundancy)
	_ = registry.Register("hardware", "VerifyPCIeErrors", hardware.NewVerifyPCIeErrors)
	_ = registry.Register("hardware", "VerifyAbsenceOfLinecards", hardware.NewVerifyAbsenceOfLinecards)

	// Chassis and Module Tests
	_ = registry.Register("hardware", "VerifyChassisHealth", hardware.NewVerifyChassisHealth)
	_ = registry.Register("hardware", "VerifyHardwareCapacityUtilization", hardware.NewVerifyHardwareCapacityUtilization)
	_ = registry.Register("hardware", "VerifyModuleStatus", hardware.NewVerifyModuleStatus)

	// Interface Tests
	_ = registry.Register("interfaces", "VerifyInterfacesStatus", interfaces.NewVerifyInterfacesStatus)
	_ = registry.Register("interfaces", "VerifyInterfaceErrors", interfaces.NewVerifyInterfaceErrors)
	_ = registry.Register("interfaces", "VerifyInterfaceUtilization", interfaces.NewVerifyInterfaceUtilization)

	// Logging Tests
	_ = registry.Register("logging", "VerifySyslogLogging", logging.NewVerifySyslogLogging)
	_ = registry.Register("logging", "VerifyLoggingPersistent", logging.NewVerifyLoggingPersistent)
	_ = registry.Register("logging", "VerifyLoggingSourceIntf", logging.NewVerifyLoggingSourceIntf)
	_ = registry.Register("logging", "VerifyLoggingHosts", logging.NewVerifyLoggingHosts)
	_ = registry.Register("logging", "VerifyLoggingLogsGeneration", logging.NewVerifyLoggingLogsGeneration)
	_ = registry.Register("logging", "VerifyLoggingHostname", logging.NewVerifyLoggingHostname)
	_ = registry.Register("logging", "VerifyLoggingTimestamp", logging.NewVerifyLoggingTimestamp)
	_ = registry.Register("logging", "VerifyLoggingAccounting", logging.NewVerifyLoggingAccounting)
	_ = registry.Register("logging", "VerifyLoggingErrors", logging.NewVerifyLoggingErrors)

	// BGP Tests - All 26 BGP tests from ANTA Python implementation
	_ = registry.Register("routing", "VerifyBGPPeers", routing.NewVerifyBGPPeers)
	_ = registry.Register("routing", "VerifyBGPUnnumbered", routing.NewVerifyBGPUnnumbered)
	_ = registry.Register("routing", "VerifyBGPPeerCount", routing.NewVerifyBGPPeerCount)
	_ = registry.Register("routing", "VerifyBGPPeersHealth", routing.NewVerifyBGPPeersHealth)
	_ = registry.Register("routing", "VerifyBGPSpecificPeers", routing.NewVerifyBGPSpecificPeers)
	_ = registry.Register("routing", "VerifyBGPPeerSession", routing.NewVerifyBGPPeerSession)
	_ = registry.Register("routing", "VerifyBGPExchangedRoutes", routing.NewVerifyBGPExchangedRoutes)
	_ = registry.Register("routing", "VerifyBGPPeerMPCaps", routing.NewVerifyBGPPeerMPCaps)
	_ = registry.Register("routing", "VerifyBGPPeerASNCap", routing.NewVerifyBGPPeerASNCap)
	_ = registry.Register("routing", "VerifyBGPPeerRouteRefreshCap", routing.NewVerifyBGPPeerRouteRefreshCap)
	_ = registry.Register("routing", "VerifyBGPPeerMD5Auth", routing.NewVerifyBGPPeerMD5Auth)
	_ = registry.Register("routing", "VerifyEVPNType2Route", routing.NewVerifyEVPNType2Route)
	_ = registry.Register("routing", "VerifyBGPAdvCommunities", routing.NewVerifyBGPAdvCommunities)
	_ = registry.Register("routing", "VerifyBGPTimers", routing.NewVerifyBGPTimers)
	_ = registry.Register("routing", "VerifyBGPPeerDropStats", routing.NewVerifyBGPPeerDropStats)
	_ = registry.Register("routing", "VerifyBGPPeerUpdateErrors", routing.NewVerifyBGPPeerUpdateErrors)
	_ = registry.Register("routing", "VerifyBgpRouteMaps", routing.NewVerifyBgpRouteMaps)
	_ = registry.Register("routing", "VerifyBGPPeerRouteLimit", routing.NewVerifyBGPPeerRouteLimit)
	_ = registry.Register("routing", "VerifyBGPPeerGroup", routing.NewVerifyBGPPeerGroup)
	_ = registry.Register("routing", "VerifyBGPPeerSessionRibd", routing.NewVerifyBGPPeerSessionRibd)
	_ = registry.Register("routing", "VerifyBGPPeersHealthRibd", routing.NewVerifyBGPPeersHealthRibd)
	_ = registry.Register("routing", "VerifyBGPNlriAcceptance", routing.NewVerifyBGPNlriAcceptance)
	_ = registry.Register("routing", "VerifyBGPRoutePaths", routing.NewVerifyBGPRoutePaths)
	_ = registry.Register("routing", "VerifyBGPRouteECMP", routing.NewVerifyBGPRouteECMP)
	_ = registry.Register("routing", "VerifyBGPRedistribution", routing.NewVerifyBGPRedistribution)
	_ = registry.Register("routing", "VerifyBGPPeerTtlMultiHops", routing.NewVerifyBGPPeerTtlMultiHops)

	// BFD Tests - All 4 BFD tests from ANTA Python implementation
	_ = registry.Register("routing", "VerifyBFDSpecificPeers", routing.NewVerifyBFDSpecificPeers)
	_ = registry.Register("routing", "VerifyBFDPeersIntervals", routing.NewVerifyBFDPeersIntervals)
	_ = registry.Register("routing", "VerifyBFDPeersHealth", routing.NewVerifyBFDPeersHealth)
	_ = registry.Register("routing", "VerifyBFDPeersRegProtocols", routing.NewVerifyBFDPeersRegProtocols)

	// Other routing tests
	_ = registry.Register("routing", "VerifyOSPFNeighbors", routing.NewVerifyOSPFNeighbors)
	_ = registry.Register("routing", "VerifyStaticRoutes", routing.NewVerifyStaticRoutes)

	// Path Selection Tests
	_ = registry.Register("routing", "VerifyPathsHealth", routing.NewVerifyPathsHealth)
	_ = registry.Register("routing", "VerifySpecificPath", routing.NewVerifySpecificPath)

	// Security Tests
	_ = registry.Register("security", "VerifySSHStatus", security.NewVerifySSHStatus)
	_ = registry.Register("security", "VerifySSHIPv4Acl", security.NewVerifySSHIPv4Acl)
	_ = registry.Register("security", "VerifySSHIPv6Acl", security.NewVerifySSHIPv6Acl)
	_ = registry.Register("security", "VerifyTelnetStatus", security.NewVerifyTelnetStatus)
	_ = registry.Register("security", "VerifyAPIHttpStatus", security.NewVerifyAPIHttpStatus)
	_ = registry.Register("security", "VerifyAPIHttpsSSL", security.NewVerifyAPIHttpsSSL)
	_ = registry.Register("security", "VerifyAPIIPv4Acl", security.NewVerifyAPIIPv4Acl)
	_ = registry.Register("security", "VerifyAPIIPv6Acl", security.NewVerifyAPIIPv6Acl)

	// AAA Tests
	_ = registry.Register("security", "VerifyTacacsSourceIntf", security.NewVerifyTacacsSourceIntf)
	_ = registry.Register("security", "VerifyTacacsServers", security.NewVerifyTacacsServers)
	_ = registry.Register("security", "VerifyTacacsServerGroups", security.NewVerifyTacacsServerGroups)
	_ = registry.Register("security", "VerifyAuthenMethods", security.NewVerifyAuthenMethods)
	_ = registry.Register("security", "VerifyAuthzMethods", security.NewVerifyAuthzMethods)
	_ = registry.Register("security", "VerifyAcctDefaultMethods", security.NewVerifyAcctDefaultMethods)
	_ = registry.Register("security", "VerifyAcctConsoleMethods", security.NewVerifyAcctConsoleMethods)

	// Services Tests
	_ = registry.Register("services", "VerifyHostname", services.NewVerifyHostname)
	_ = registry.Register("services", "VerifyDNSLookup", services.NewVerifyDNSLookup)
	_ = registry.Register("services", "VerifyDNSServers", services.NewVerifyDNSServers)
	_ = registry.Register("services", "VerifyErrdisableRecovery", services.NewVerifyErrdisableRecovery)

	// Software Tests (Note: VerifyEOSVersion is in system module)
	_ = registry.Register("software", "VerifyTerminAttrVersion", software.NewVerifyTerminAttrVersion)
	_ = registry.Register("software", "VerifyEOSExtensions", software.NewVerifyEOSExtensions)

	// STP Tests
	_ = registry.Register("stp", "VerifySTPMode", stp.NewVerifySTPMode)
	_ = registry.Register("stp", "VerifySTPBlockedPorts", stp.NewVerifySTPBlockedPorts)
	_ = registry.Register("stp", "VerifySTPCounters", stp.NewVerifySTPCounters)
	_ = registry.Register("stp", "VerifySTPForwardingPorts", stp.NewVerifySTPForwardingPorts)
	_ = registry.Register("stp", "VerifySTPRootPriority", stp.NewVerifySTPRootPriority)
	_ = registry.Register("stp", "VerifyStpTopologyChanges", stp.NewVerifyStpTopologyChanges)
	_ = registry.Register("stp", "VerifySTPDisabledVlans", stp.NewVerifySTPDisabledVlans)

	_ = registry.Register("system", "VerifyEOSVersion", system.NewVerifyEOSVersion)
	_ = registry.Register("system", "VerifyUptime", system.NewVerifyUptime)
	_ = registry.Register("system", "VerifyNTP", system.NewVerifyNTP)
	_ = registry.Register("system", "VerifyDNSResolution", NewVerifyDNSResolution)
	_ = registry.Register("system", "VerifyReloadCause", system.NewVerifyReloadCause)
	_ = registry.Register("system", "VerifyCoredump", system.NewVerifyCoredump)
	_ = registry.Register("system", "VerifyAgentLogs", system.NewVerifyAgentLogs)
	_ = registry.Register("system", "VerifyCPUUtilization", system.NewVerifyCPUUtilization)
	_ = registry.Register("system", "VerifyMemoryUtilization", system.NewVerifyMemoryUtilization)
	_ = registry.Register("system", "VerifyFileSystemUtilization", system.NewVerifyFileSystemUtilization)
	_ = registry.Register("system", "VerifyMaintenance", system.NewVerifyMaintenance)
	_ = registry.Register("system", "VerifyFlashUtilization", system.NewVerifyFlashUtilization)

	// MLAG Tests
	_ = registry.Register("system", "VerifyMlagStatus", system.NewVerifyMlagStatus)
	_ = registry.Register("system", "VerifyMlagInterfaces", system.NewVerifyMlagInterfaces)
	_ = registry.Register("system", "VerifyMlagConfigSanity", system.NewVerifyMlagConfigSanity)
	_ = registry.Register("system", "VerifyMlagReloadDelay", system.NewVerifyMlagReloadDelay)
	_ = registry.Register("system", "VerifyMlagDualPrimary", system.NewVerifyMlagDualPrimary)

	// Configuration Tests
	_ = registry.Register("system", "VerifyZeroTouch", system.NewVerifyZeroTouch)
	_ = registry.Register("system", "VerifyRunningConfigDiffs", system.NewVerifyRunningConfigDiffs)
	_ = registry.Register("system", "VerifyRunningConfigLines", system.NewVerifyRunningConfigLines)

	// VLAN Tests
	_ = registry.Register("vlan", "VerifyVlanInternalPolicy", vlan.NewVerifyVlanInternalPolicy)
	_ = registry.Register("vlan", "VerifyDynamicVlanSource", vlan.NewVerifyDynamicVlanSource)
	_ = registry.Register("vlan", "VerifyVlanStatus", vlan.NewVerifyVlanStatus)

	// VXLAN Tests
	_ = registry.Register("vxlan", "VerifyVxlan1Interface", vxlan.NewVerifyVxlan1Interface)
	_ = registry.Register("vxlan", "VerifyVxlanConfigSanity", vxlan.NewVerifyVxlanConfigSanity)
	_ = registry.Register("vxlan", "VerifyVxlanVniBinding", vxlan.NewVerifyVxlanVniBinding)
	_ = registry.Register("vxlan", "VerifyVxlanVtep", vxlan.NewVerifyVxlanVtep)
	_ = registry.Register("vxlan", "VerifyVxlan1ConnSettings", vxlan.NewVerifyVxlan1ConnSettings)
}

func NewVerifyPowerSupplies(inputs map[string]any) (test.Test, error) {
	return hardware.NewVerifyInventory(inputs)
}

func NewVerifyDNSResolution(inputs map[string]any) (test.Test, error) {
	return system.NewVerifyDNSResolution(inputs)
}