package security

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifyTacacsSourceIntf verifies TACACS source interface configuration.
//
// This test validates that TACACS+ is configured to use a specific source interface
// for communication with TACACS+ servers. Using a dedicated source interface ensures
// consistent and predictable communication paths for authentication traffic.
//
// The test performs the following checks:
//   1. Retrieves the TACACS configuration from the device.
//   2. Verifies that TACACS is configured for the specified VRF.
//   3. Validates that the specified interface is set as the source interface.
//
// Expected Results:
//   - Success: The test will pass if the specified interface is configured as the TACACS source in the given VRF.
//   - Failure: The test will fail if the interface is not configured or incorrect.
//   - Error: The test will report an error if TACACS configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyTacacsSourceIntf management interface
//     VerifyTacacsSourceIntf:
//       interface: "Management1"
//       vrf: "MGMT"
//
//   - name: VerifyTacacsSourceIntf loopback
//     VerifyTacacsSourceIntf:
//       interface: "Loopback0"
//       vrf: "default"
type VerifyTacacsSourceIntf struct {
	test.BaseTest
	Interface string `yaml:"interface" json:"interface"`
	VRF       string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyTacacsSourceIntf(inputs map[string]any) (test.Test, error) {
	t := &VerifyTacacsSourceIntf{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTacacsSourceIntf",
			TestDescription: "Verify TACACS source interface configuration",
			TestCategories:  []string{"security", "aaa"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if intf, ok := inputs["interface"].(string); ok {
			t.Interface = intf
		}
		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifyTacacsSourceIntf) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show tacacs",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get TACACS configuration: %v", err)
		return result, nil
	}

	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Check for VRF configuration
		if vrfData, ok := data["vrfs"].(map[string]any); ok {
			if vrfConfig, ok := vrfData[t.VRF].(map[string]any); ok {
				if sourceIntf, ok := vrfConfig["sourceIntf"].(string); ok {
					if sourceIntf != t.Interface {
						result.Status = test.TestFailure
						result.Message = fmt.Sprintf("TACACS source interface for VRF %s: expected '%s', got '%s'", t.VRF, t.Interface, sourceIntf)
					}
				} else {
					result.Status = test.TestFailure
					result.Message = fmt.Sprintf("No TACACS source interface configured for VRF %s", t.VRF)
				}
			} else {
				result.Status = test.TestFailure
				result.Message = fmt.Sprintf("VRF %s not configured for TACACS", t.VRF)
			}
		} else {
			result.Status = test.TestError
			result.Message = "Unable to parse TACACS VRF configuration"
		}
	}

	return result, nil
}

func (t *VerifyTacacsSourceIntf) ValidateInput(input any) error {
	if t.Interface == "" {
		return fmt.Errorf("interface must be specified")
	}
	return nil
}

// VerifyTacacsServers verifies TACACS server configurations.
//
// This test validates that specific TACACS+ servers are configured in the specified VRF.
// Proper TACACS+ server configuration is essential for centralized authentication,
// authorization, and accounting services.
//
// The test performs the following checks:
//   1. Retrieves the TACACS server configuration from the device.
//   2. Verifies that all specified servers are configured in the given VRF.
//   3. Validates that each server is present and properly configured.
//
// Expected Results:
//   - Success: The test will pass if all specified servers are configured in the VRF.
//   - Failure: The test will fail if any server is missing from the configuration.
//   - Error: The test will report an error if TACACS configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyTacacsServers production servers
//     VerifyTacacsServers:
//       servers:
//         - "10.1.1.10"
//         - "10.1.1.11"
//       vrf: "MGMT"
//
//   - name: VerifyTacacsServers default VRF
//     VerifyTacacsServers:
//       servers:
//         - "192.168.1.100"
//         - "192.168.1.101"
type VerifyTacacsServers struct {
	test.BaseTest
	Servers []string `yaml:"servers" json:"servers"`
	VRF     string   `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyTacacsServers(inputs map[string]any) (test.Test, error) {
	t := &VerifyTacacsServers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTacacsServers",
			TestDescription: "Verify TACACS servers are configured",
			TestCategories:  []string{"security", "aaa"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if servers, ok := inputs["servers"].([]any); ok {
			for _, server := range servers {
				if serverStr, ok := server.(string); ok {
					t.Servers = append(t.Servers, serverStr)
				}
			}
		}
		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifyTacacsServers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show tacacs",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get TACACS configuration: %v", err)
		return result, nil
	}

	configuredServers := []string{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if vrfData, ok := data["vrfs"].(map[string]any); ok {
			if vrfConfig, ok := vrfData[t.VRF].(map[string]any); ok {
				if servers, ok := vrfConfig["servers"].([]any); ok {
					for _, server := range servers {
						if serverMap, ok := server.(map[string]any); ok {
							if serverAddr, ok := serverMap["host"].(string); ok {
								configuredServers = append(configuredServers, serverAddr)
							}
						} else if serverStr, ok := server.(string); ok {
							configuredServers = append(configuredServers, serverStr)
						}
					}
				}
			}
		}
	}

	// Check if all expected servers are configured
	missingServers := []string{}
	for _, expectedServer := range t.Servers {
		found := false
		for _, configuredServer := range configuredServers {
			if expectedServer == configuredServer {
				found = true
				break
			}
		}
		if !found {
			missingServers = append(missingServers, expectedServer)
		}
	}

	if len(missingServers) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("TACACS servers not configured in VRF %s: %v", t.VRF, missingServers)
	}

	return result, nil
}

func (t *VerifyTacacsServers) ValidateInput(input any) error {
	if len(t.Servers) == 0 {
		return fmt.Errorf("at least one server must be specified")
	}
	return nil
}

// VerifyTacacsServerGroups verifies TACACS server group configurations.
//
// This test validates that specific TACACS+ server groups are configured on the device.
// Server groups allow organizing TACACS+ servers for redundancy and load balancing.
//
// The test performs the following checks:
//   1. Retrieves the TACACS server group configuration from the device.
//   2. Verifies that all specified server groups exist.
//   3. Validates the presence of each configured group.
//
// Expected Results:
//   - Success: The test will pass if all specified server groups are configured.
//   - Failure: The test will fail if any server group is missing from the configuration.
//   - Error: The test will report an error if TACACS configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyTacacsServerGroups production groups
//     VerifyTacacsServerGroups:
//       groups:
//         - "TACACS_PRIMARY"
//         - "TACACS_BACKUP"
//
//   - name: VerifyTacacsServerGroups single group
//     VerifyTacacsServerGroups:
//       groups:
//         - "TACACS_SERVERS"
type VerifyTacacsServerGroups struct {
	test.BaseTest
	Groups []string `yaml:"groups" json:"groups"`
}

func NewVerifyTacacsServerGroups(inputs map[string]any) (test.Test, error) {
	t := &VerifyTacacsServerGroups{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTacacsServerGroups",
			TestDescription: "Verify TACACS server groups are configured",
			TestCategories:  []string{"security", "aaa"},
		},
	}

	if inputs != nil {
		if groups, ok := inputs["groups"].([]any); ok {
			for _, group := range groups {
				if groupStr, ok := group.(string); ok {
					t.Groups = append(t.Groups, groupStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyTacacsServerGroups) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show tacacs",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get TACACS configuration: %v", err)
		return result, nil
	}

	configuredGroups := []string{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		if groups, ok := data["serverGroups"].(map[string]any); ok {
			for groupName := range groups {
				configuredGroups = append(configuredGroups, groupName)
			}
		}
	}

	// Check if all expected groups are configured
	missingGroups := []string{}
	for _, expectedGroup := range t.Groups {
		found := false
		for _, configuredGroup := range configuredGroups {
			if expectedGroup == configuredGroup {
				found = true
				break
			}
		}
		if !found {
			missingGroups = append(missingGroups, expectedGroup)
		}
	}

	if len(missingGroups) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("TACACS server groups not configured: %v", missingGroups)
	}

	return result, nil
}

func (t *VerifyTacacsServerGroups) ValidateInput(input any) error {
	if len(t.Groups) == 0 {
		return fmt.Errorf("at least one group must be specified")
	}
	return nil
}

// VerifyAuthenMethods verifies AAA authentication method configurations.
//
// This test validates that the specified authentication methods are configured
// for different authentication types (login, enable, dot1x). Proper authentication
// method configuration ensures secure access control to the device.
//
// The test performs the following checks:
//   1. Retrieves the AAA authentication method configuration from the device.
//   2. Verifies that the specified methods are configured for each authentication type.
//   3. Validates that the method order matches the expected configuration.
//
// Expected Results:
//   - Success: The test will pass if all specified authentication methods match the configuration.
//   - Failure: The test will fail if any method configuration doesn't match the expected setup.
//   - Error: The test will report an error if AAA configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyAuthenMethods complete setup
//     VerifyAuthenMethods:
//       login:
//         - "group tacacs+"
//         - "local"
//       enable:
//         - "group tacacs+"
//         - "local"
//       dot1x:
//         - "group radius"
//
//   - name: VerifyAuthenMethods local only
//     VerifyAuthenMethods:
//       login:
//         - "local"
//       enable:
//         - "local"
type VerifyAuthenMethods struct {
	test.BaseTest
	Login  []string `yaml:"login,omitempty" json:"login,omitempty"`
	Enable []string `yaml:"enable,omitempty" json:"enable,omitempty"`
	Dot1x  []string `yaml:"dot1x,omitempty" json:"dot1x,omitempty"`
}

func NewVerifyAuthenMethods(inputs map[string]any) (test.Test, error) {
	t := &VerifyAuthenMethods{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAuthenMethods",
			TestDescription: "Verify AAA authentication methods",
			TestCategories:  []string{"security", "aaa"},
		},
	}

	if inputs != nil {
		if login, ok := inputs["login"].([]any); ok {
			for _, method := range login {
				if methodStr, ok := method.(string); ok {
					t.Login = append(t.Login, methodStr)
				}
			}
		}
		if enable, ok := inputs["enable"].([]any); ok {
			for _, method := range enable {
				if methodStr, ok := method.(string); ok {
					t.Enable = append(t.Enable, methodStr)
				}
			}
		}
		if dot1x, ok := inputs["dot1x"].([]any); ok {
			for _, method := range dot1x {
				if methodStr, ok := method.(string); ok {
					t.Dot1x = append(t.Dot1x, methodStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyAuthenMethods) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show aaa methods authentication",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get AAA authentication methods: %v", err)
		return result, nil
	}

	failures := []string{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Check login methods
		if len(t.Login) > 0 {
			if !t.verifyMethods(data, "login", "default", t.Login) {
				failures = append(failures, fmt.Sprintf("Login methods mismatch: expected %v", t.Login))
			}
		}

		// Check enable methods
		if len(t.Enable) > 0 {
			if !t.verifyMethods(data, "enable", "default", t.Enable) {
				failures = append(failures, fmt.Sprintf("Enable methods mismatch: expected %v", t.Enable))
			}
		}

		// Check dot1x methods
		if len(t.Dot1x) > 0 {
			if !t.verifyMethods(data, "dot1x", "default", t.Dot1x) {
				failures = append(failures, fmt.Sprintf("Dot1x methods mismatch: expected %v", t.Dot1x))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Authentication method failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyAuthenMethods) verifyMethods(data map[string]any, authType, listName string, expected []string) bool {
	if authData, ok := data[authType].(map[string]any); ok {
		if methods, ok := authData[listName].(map[string]any); ok {
			if methodList, ok := methods["methods"].([]any); ok {
				if len(methodList) != len(expected) {
					return false
				}
				for i, method := range methodList {
					if methodStr, ok := method.(string); ok {
						if methodStr != expected[i] {
							return false
						}
					} else {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func (t *VerifyAuthenMethods) ValidateInput(input any) error {
	if len(t.Login) == 0 && len(t.Enable) == 0 && len(t.Dot1x) == 0 {
		return fmt.Errorf("at least one authentication type must be specified")
	}
	return nil
}

// VerifyAuthzMethods verifies AAA authorization method configurations.
//
// This test validates that the specified authorization methods are configured
// for different authorization types (commands, exec). Proper authorization
// method configuration ensures appropriate privilege control.
//
// The test performs the following checks:
//   1. Retrieves the AAA authorization method configuration from the device.
//   2. Verifies that the specified methods are configured for each authorization type.
//   3. Validates that the method order matches the expected configuration.
//
// Expected Results:
//   - Success: The test will pass if all specified authorization methods match the configuration.
//   - Failure: The test will fail if any method configuration doesn't match the expected setup.
//   - Error: The test will report an error if AAA configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyAuthzMethods complete setup
//     VerifyAuthzMethods:
//       commands:
//         - "group tacacs+"
//         - "local"
//       exec:
//         - "group tacacs+"
//         - "local"
//
//   - name: VerifyAuthzMethods local only
//     VerifyAuthzMethods:
//       commands:
//         - "local"
//       exec:
//         - "local"
type VerifyAuthzMethods struct {
	test.BaseTest
	CommandMethods []string `yaml:"commands,omitempty" json:"commands,omitempty"`
	ExecMethods    []string `yaml:"exec,omitempty" json:"exec,omitempty"`
}

func NewVerifyAuthzMethods(inputs map[string]any) (test.Test, error) {
	t := &VerifyAuthzMethods{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAuthzMethods",
			TestDescription: "Verify AAA authorization methods",
			TestCategories:  []string{"security", "aaa"},
		},
	}

	if inputs != nil {
		if commands, ok := inputs["commands"].([]any); ok {
			for _, method := range commands {
				if methodStr, ok := method.(string); ok {
					t.CommandMethods = append(t.CommandMethods, methodStr)
				}
			}
		}
		if exec, ok := inputs["exec"].([]any); ok {
			for _, method := range exec {
				if methodStr, ok := method.(string); ok {
					t.ExecMethods = append(t.ExecMethods, methodStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyAuthzMethods) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show aaa methods authorization",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get AAA authorization methods: %v", err)
		return result, nil
	}

	failures := []string{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Check commands methods
		if len(t.CommandMethods) > 0 {
			if !t.verifyAuthzMethodList(data, "commands", "default", t.CommandMethods) {
				failures = append(failures, fmt.Sprintf("Commands methods mismatch: expected %v", t.CommandMethods))
			}
		}

		// Check exec methods
		if len(t.ExecMethods) > 0 {
			if !t.verifyAuthzMethodList(data, "exec", "default", t.ExecMethods) {
				failures = append(failures, fmt.Sprintf("Exec methods mismatch: expected %v", t.ExecMethods))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Authorization method failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyAuthzMethods) verifyAuthzMethodList(data map[string]any, authzType, listName string, expected []string) bool {
	if authzData, ok := data[authzType].(map[string]any); ok {
		if methods, ok := authzData[listName].(map[string]any); ok {
			if methodList, ok := methods["methods"].([]any); ok {
				if len(methodList) != len(expected) {
					return false
				}
				for i, method := range methodList {
					if methodStr, ok := method.(string); ok {
						if methodStr != expected[i] {
							return false
						}
					} else {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func (t *VerifyAuthzMethods) ValidateInput(input any) error {
	if len(t.CommandMethods) == 0 && len(t.ExecMethods) == 0 {
		return fmt.Errorf("at least one authorization type must be specified")
	}
	return nil
}

// VerifyAcctDefaultMethods verifies AAA accounting default method configurations.
//
// This test validates that the specified accounting methods are configured
// for different accounting types (system, exec, commands, dot1x) using the default list.
// Proper accounting method configuration ensures audit trail and compliance.
//
// The test performs the following checks:
//   1. Retrieves the AAA accounting method configuration from the device.
//   2. Verifies that the specified methods are configured for each accounting type.
//   3. Validates that the method order matches the expected configuration.
//
// Expected Results:
//   - Success: The test will pass if all specified accounting methods match the configuration.
//   - Failure: The test will fail if any method configuration doesn't match the expected setup.
//   - Error: The test will report an error if AAA configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyAcctDefaultMethods complete setup
//     VerifyAcctDefaultMethods:
//       system:
//         - "group tacacs+"
//         - "local"
//       exec:
//         - "group tacacs+"
//         - "local"
//       commands:
//         - "group tacacs+"
//         - "local"
//       dot1x:
//         - "group radius"
//
//   - name: VerifyAcctDefaultMethods tacacs only
//     VerifyAcctDefaultMethods:
//       system:
//         - "group tacacs+"
//       exec:
//         - "group tacacs+"
type VerifyAcctDefaultMethods struct {
	test.BaseTest
	SystemMethods   []string `yaml:"system,omitempty" json:"system,omitempty"`
	ExecMethods     []string `yaml:"exec,omitempty" json:"exec,omitempty"`
	CommandMethods  []string `yaml:"commands,omitempty" json:"commands,omitempty"`
	Dot1xMethods    []string `yaml:"dot1x,omitempty" json:"dot1x,omitempty"`
}

func NewVerifyAcctDefaultMethods(inputs map[string]any) (test.Test, error) {
	t := &VerifyAcctDefaultMethods{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAcctDefaultMethods",
			TestDescription: "Verify AAA accounting default methods",
			TestCategories:  []string{"security", "aaa"},
		},
	}

	if inputs != nil {
		if system, ok := inputs["system"].([]any); ok {
			for _, method := range system {
				if methodStr, ok := method.(string); ok {
					t.SystemMethods = append(t.SystemMethods, methodStr)
				}
			}
		}
		if exec, ok := inputs["exec"].([]any); ok {
			for _, method := range exec {
				if methodStr, ok := method.(string); ok {
					t.ExecMethods = append(t.ExecMethods, methodStr)
				}
			}
		}
		if commands, ok := inputs["commands"].([]any); ok {
			for _, method := range commands {
				if methodStr, ok := method.(string); ok {
					t.CommandMethods = append(t.CommandMethods, methodStr)
				}
			}
		}
		if dot1x, ok := inputs["dot1x"].([]any); ok {
			for _, method := range dot1x {
				if methodStr, ok := method.(string); ok {
					t.Dot1xMethods = append(t.Dot1xMethods, methodStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyAcctDefaultMethods) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show aaa methods accounting",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get AAA accounting methods: %v", err)
		return result, nil
	}

	failures := []string{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Check system accounting methods
		if len(t.SystemMethods) > 0 {
			if !t.verifyAcctMethodList(data, "system", "default", t.SystemMethods) {
				failures = append(failures, fmt.Sprintf("System accounting methods mismatch: expected %v", t.SystemMethods))
			}
		}

		// Check exec accounting methods
		if len(t.ExecMethods) > 0 {
			if !t.verifyAcctMethodList(data, "exec", "default", t.ExecMethods) {
				failures = append(failures, fmt.Sprintf("Exec accounting methods mismatch: expected %v", t.ExecMethods))
			}
		}

		// Check commands accounting methods
		if len(t.CommandMethods) > 0 {
			if !t.verifyAcctMethodList(data, "commands", "default", t.CommandMethods) {
				failures = append(failures, fmt.Sprintf("Commands accounting methods mismatch: expected %v", t.CommandMethods))
			}
		}

		// Check dot1x accounting methods
		if len(t.Dot1xMethods) > 0 {
			if !t.verifyAcctMethodList(data, "dot1x", "default", t.Dot1xMethods) {
				failures = append(failures, fmt.Sprintf("Dot1x accounting methods mismatch: expected %v", t.Dot1xMethods))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Accounting method failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyAcctDefaultMethods) verifyAcctMethodList(data map[string]any, acctType, listName string, expected []string) bool {
	if acctData, ok := data[acctType].(map[string]any); ok {
		if methods, ok := acctData[listName].(map[string]any); ok {
			if methodList, ok := methods["methods"].([]any); ok {
				if len(methodList) != len(expected) {
					return false
				}
				for i, method := range methodList {
					if methodStr, ok := method.(string); ok {
						if !strings.EqualFold(methodStr, expected[i]) {
							return false
						}
					} else {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func (t *VerifyAcctDefaultMethods) ValidateInput(input any) error {
	if len(t.SystemMethods) == 0 && len(t.ExecMethods) == 0 && len(t.CommandMethods) == 0 && len(t.Dot1xMethods) == 0 {
		return fmt.Errorf("at least one accounting type must be specified")
	}
	return nil
}

// VerifyAcctConsoleMethods verifies AAA accounting console method configurations.
//
// This test validates that the specified accounting methods are configured
// for different accounting types (system, exec, commands, dot1x) using the console list.
// Console accounting configuration is important for tracking local console access.
//
// The test performs the following checks:
//   1. Retrieves the AAA accounting method configuration from the device.
//   2. Verifies that the specified methods are configured for each accounting type on console.
//   3. Validates that the method order matches the expected configuration.
//
// Expected Results:
//   - Success: The test will pass if all specified accounting methods match the configuration.
//   - Failure: The test will fail if any method configuration doesn't match the expected setup.
//   - Error: The test will report an error if AAA configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyAcctConsoleMethods complete setup
//     VerifyAcctConsoleMethods:
//       system:
//         - "group tacacs+"
//         - "local"
//       exec:
//         - "group tacacs+"
//         - "local"
//       commands:
//         - "group tacacs+"
//         - "local"
//
//   - name: VerifyAcctConsoleMethods local only
//     VerifyAcctConsoleMethods:
//       system:
//         - "local"
//       exec:
//         - "local"
type VerifyAcctConsoleMethods struct {
	test.BaseTest
	SystemMethods   []string `yaml:"system,omitempty" json:"system,omitempty"`
	ExecMethods     []string `yaml:"exec,omitempty" json:"exec,omitempty"`
	CommandMethods  []string `yaml:"commands,omitempty" json:"commands,omitempty"`
	Dot1xMethods    []string `yaml:"dot1x,omitempty" json:"dot1x,omitempty"`
}

func NewVerifyAcctConsoleMethods(inputs map[string]any) (test.Test, error) {
	t := &VerifyAcctConsoleMethods{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAcctConsoleMethods",
			TestDescription: "Verify AAA accounting console methods",
			TestCategories:  []string{"security", "aaa"},
		},
	}

	if inputs != nil {
		if system, ok := inputs["system"].([]any); ok {
			for _, method := range system {
				if methodStr, ok := method.(string); ok {
					t.SystemMethods = append(t.SystemMethods, methodStr)
				}
			}
		}
		if exec, ok := inputs["exec"].([]any); ok {
			for _, method := range exec {
				if methodStr, ok := method.(string); ok {
					t.ExecMethods = append(t.ExecMethods, methodStr)
				}
			}
		}
		if commands, ok := inputs["commands"].([]any); ok {
			for _, method := range commands {
				if methodStr, ok := method.(string); ok {
					t.CommandMethods = append(t.CommandMethods, methodStr)
				}
			}
		}
		if dot1x, ok := inputs["dot1x"].([]any); ok {
			for _, method := range dot1x {
				if methodStr, ok := method.(string); ok {
					t.Dot1xMethods = append(t.Dot1xMethods, methodStr)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyAcctConsoleMethods) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show aaa methods accounting",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get AAA accounting methods: %v", err)
		return result, nil
	}

	failures := []string{}
	if data, ok := cmdResult.Output.(map[string]any); ok {
		// Check system accounting methods
		if len(t.SystemMethods) > 0 {
			if !t.verifyAcctMethodList(data, "system", "console", t.SystemMethods) {
				failures = append(failures, fmt.Sprintf("System console accounting methods mismatch: expected %v", t.SystemMethods))
			}
		}

		// Check exec accounting methods
		if len(t.ExecMethods) > 0 {
			if !t.verifyAcctMethodList(data, "exec", "console", t.ExecMethods) {
				failures = append(failures, fmt.Sprintf("Exec console accounting methods mismatch: expected %v", t.ExecMethods))
			}
		}

		// Check commands accounting methods
		if len(t.CommandMethods) > 0 {
			if !t.verifyAcctMethodList(data, "commands", "console", t.CommandMethods) {
				failures = append(failures, fmt.Sprintf("Commands console accounting methods mismatch: expected %v", t.CommandMethods))
			}
		}

		// Check dot1x accounting methods
		if len(t.Dot1xMethods) > 0 {
			if !t.verifyAcctMethodList(data, "dot1x", "console", t.Dot1xMethods) {
				failures = append(failures, fmt.Sprintf("Dot1x console accounting methods mismatch: expected %v", t.Dot1xMethods))
			}
		}
	}

	if len(failures) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Console accounting method failures: %v", failures)
	}

	return result, nil
}

func (t *VerifyAcctConsoleMethods) verifyAcctMethodList(data map[string]any, acctType, listName string, expected []string) bool {
	if acctData, ok := data[acctType].(map[string]any); ok {
		if methods, ok := acctData[listName].(map[string]any); ok {
			if methodList, ok := methods["methods"].([]any); ok {
				if len(methodList) != len(expected) {
					return false
				}
				for i, method := range methodList {
					if methodStr, ok := method.(string); ok {
						if !strings.EqualFold(methodStr, expected[i]) {
							return false
						}
					} else {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func (t *VerifyAcctConsoleMethods) ValidateInput(input any) error {
	if len(t.SystemMethods) == 0 && len(t.ExecMethods) == 0 && len(t.CommandMethods) == 0 && len(t.Dot1xMethods) == 0 {
		return fmt.Errorf("at least one accounting type must be specified")
	}
	return nil
}