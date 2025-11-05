package security

import (
	"context"
	"fmt"

	"github.com/gavmckee/go-anta/pkg/device"
	"github.com/gavmckee/go-anta/pkg/test"
)

// VerifySSHStatus verifies if the SSHD agent is disabled in the default VRF.
//
// Expected Results:
//   - Success: The test will pass if the SSHD agent is disabled in the default VRF.
//   - Failure: The test will fail if the SSHD agent is NOT disabled in the default VRF.
//   - Error: The test will report an error if SSH status cannot be determined.
//
// Examples:
//   - name: VerifySSHStatus
//     VerifySSHStatus:
type VerifySSHStatus struct {
	test.BaseTest
}

func NewVerifySSHStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifySSHStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifySSHStatus",
			TestDescription: "Verify if SSHD agent is disabled in the default VRF",
			TestCategories:  []string{"security", "ssh"},
		},
	}

	return t, nil
}

func (t *VerifySSHStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management ssh",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get SSH management status: %v", err)
		return result, nil
	}

	if sshData, ok := cmdResult.Output.(map[string]any); ok {
		if serverData, ok := sshData["sshServer"].(map[string]any); ok {
			if vrfs, ok := serverData["vrfs"].(map[string]any); ok {
				if defaultVrf, ok := vrfs["default"].(map[string]any); ok {
					if enabled, ok := defaultVrf["enabled"].(bool); ok {
						if enabled {
							result.Status = test.TestFailure
							result.Message = "SSH is enabled in the default VRF - should be disabled for security"
						} else {
							result.Details = map[string]any{
								"ssh_status": "disabled",
								"vrf":        "default",
							}
						}
						return result, nil
					}
				}
			}
		}
	}

	result.Status = test.TestError
	result.Message = "Could not determine SSH status from device response"
	return result, nil
}

func (t *VerifySSHStatus) ValidateInput(input any) error {
	return nil
}

// VerifySSHIPv4Acl verifies if the SSHD agent has the correct number of IPv4 ACLs configured.
//
// Expected Results:
//   - Success: The test will pass if the SSHD agent has the expected number of IPv4 ACLs in the specified VRF.
//   - Failure: The test will fail if the SSHD agent does not have the expected number of IPv4 ACLs.
//   - Error: The test will report an error if SSH IPv4 ACL information cannot be retrieved.
//
// Examples:
//   - name: VerifySSHIPv4Acl with specific count
//     VerifySSHIPv4Acl:
//       number: 3
//       vrf: "default"
//
//   - name: VerifySSHIPv4Acl with default VRF
//     VerifySSHIPv4Acl:
//       number: 2
type VerifySSHIPv4Acl struct {
	test.BaseTest
	Number int    `yaml:"number" json:"number"`
	VRF    string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifySSHIPv4Acl(inputs map[string]any) (test.Test, error) {
	t := &VerifySSHIPv4Acl{
		BaseTest: test.BaseTest{
			TestName:        "VerifySSHIPv4Acl",
			TestDescription: "Verify SSH IPv4 Access Control Lists",
			TestCategories:  []string{"security", "ssh", "acl"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if number, ok := inputs["number"].(float64); ok {
			t.Number = int(number)
		} else if number, ok := inputs["number"].(int); ok {
			t.Number = number
		}

		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifySSHIPv4Acl) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management ssh ip access-list summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get SSH IPv4 ACL summary: %v", err)
		return result, nil
	}

	aclCount := 0
	if aclData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := aclData["vrfs"].(map[string]any); ok {
			if vrfData, ok := vrfs[t.VRF].(map[string]any); ok {
				if ipAccessLists, ok := vrfData["ipAccessLists"].(map[string]any); ok {
					aclCount = len(ipAccessLists)
				}
			}
		}
	}

	if aclCount != t.Number {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Expected %d SSH IPv4 ACLs in VRF %s, found %d", t.Number, t.VRF, aclCount)
	} else {
		result.Details = map[string]any{
			"expected_acl_count": t.Number,
			"actual_acl_count":   aclCount,
			"vrf":                t.VRF,
		}
	}

	return result, nil
}

func (t *VerifySSHIPv4Acl) ValidateInput(input any) error {
	if t.Number < 0 {
		return fmt.Errorf("number of ACLs must be non-negative")
	}
	if t.VRF == "" {
		return fmt.Errorf("VRF cannot be empty")
	}
	return nil
}

// VerifySSHIPv6Acl verifies if the SSHD agent has the correct number of IPv6 ACLs configured.
//
// Expected Results:
//   - Success: The test will pass if the SSHD agent has the expected number of IPv6 ACLs in the specified VRF.
//   - Failure: The test will fail if the SSHD agent does not have the expected number of IPv6 ACLs.
//   - Error: The test will report an error if SSH IPv6 ACL information cannot be retrieved.
//
// Examples:
//   - name: VerifySSHIPv6Acl with specific count
//     VerifySSHIPv6Acl:
//       number: 2
//       vrf: "default"
type VerifySSHIPv6Acl struct {
	test.BaseTest
	Number int    `yaml:"number" json:"number"`
	VRF    string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifySSHIPv6Acl(inputs map[string]any) (test.Test, error) {
	t := &VerifySSHIPv6Acl{
		BaseTest: test.BaseTest{
			TestName:        "VerifySSHIPv6Acl",
			TestDescription: "Verify SSH IPv6 Access Control Lists",
			TestCategories:  []string{"security", "ssh", "acl", "ipv6"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if number, ok := inputs["number"].(float64); ok {
			t.Number = int(number)
		} else if number, ok := inputs["number"].(int); ok {
			t.Number = number
		}

		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifySSHIPv6Acl) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management ssh ipv6 access-list summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get SSH IPv6 ACL summary: %v", err)
		return result, nil
	}

	aclCount := 0
	if aclData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := aclData["vrfs"].(map[string]any); ok {
			if vrfData, ok := vrfs[t.VRF].(map[string]any); ok {
				if ipv6AccessLists, ok := vrfData["ipv6AccessLists"].(map[string]any); ok {
					aclCount = len(ipv6AccessLists)
				}
			}
		}
	}

	if aclCount != t.Number {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Expected %d SSH IPv6 ACLs in VRF %s, found %d", t.Number, t.VRF, aclCount)
	} else {
		result.Details = map[string]any{
			"expected_acl_count": t.Number,
			"actual_acl_count":   aclCount,
			"vrf":                t.VRF,
		}
	}

	return result, nil
}

func (t *VerifySSHIPv6Acl) ValidateInput(input any) error {
	if t.Number < 0 {
		return fmt.Errorf("number of ACLs must be non-negative")
	}
	if t.VRF == "" {
		return fmt.Errorf("VRF cannot be empty")
	}
	return nil
}

// VerifyTelnetStatus verifies if Telnet is disabled in the default VRF.
//
// Expected Results:
//   - Success: The test will pass if Telnet is disabled in the default VRF.
//   - Failure: The test will fail if Telnet is NOT disabled in the default VRF.
//   - Error: The test will report an error if Telnet status cannot be determined.
//
// Examples:
//   - name: VerifyTelnetStatus
//     VerifyTelnetStatus:
type VerifyTelnetStatus struct {
	test.BaseTest
}

func NewVerifyTelnetStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifyTelnetStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifyTelnetStatus",
			TestDescription: "Verify if Telnet is disabled in the default VRF",
			TestCategories:  []string{"security", "telnet"},
		},
	}

	return t, nil
}

func (t *VerifyTelnetStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management telnet",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get Telnet management status: %v", err)
		return result, nil
	}

	if telnetData, ok := cmdResult.Output.(map[string]any); ok {
		if serverData, ok := telnetData["telnetServer"].(map[string]any); ok {
			if vrfs, ok := serverData["vrfs"].(map[string]any); ok {
				if defaultVrf, ok := vrfs["default"].(map[string]any); ok {
					if enabled, ok := defaultVrf["enabled"].(bool); ok {
						if enabled {
							result.Status = test.TestFailure
							result.Message = "Telnet is enabled in the default VRF - should be disabled for security"
						} else {
							result.Details = map[string]any{
								"telnet_status": "disabled",
								"vrf":           "default",
							}
						}
						return result, nil
					}
				}
			}
		}
	}

	result.Status = test.TestError
	result.Message = "Could not determine Telnet status from device response"
	return result, nil
}

func (t *VerifyTelnetStatus) ValidateInput(input any) error {
	return nil
}

// VerifyAPIHttpStatus verifies if the eAPI HTTP server is disabled globally.
//
// Expected Results:
//   - Success: The test will pass if the eAPI HTTP server is disabled.
//   - Failure: The test will fail if the eAPI HTTP server is running.
//   - Error: The test will report an error if eAPI HTTP status cannot be determined.
//
// Examples:
//   - name: VerifyAPIHttpStatus
//     VerifyAPIHttpStatus:
type VerifyAPIHttpStatus struct {
	test.BaseTest
}

func NewVerifyAPIHttpStatus(inputs map[string]any) (test.Test, error) {
	t := &VerifyAPIHttpStatus{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAPIHttpStatus",
			TestDescription: "Verify if eAPI HTTP server is disabled globally",
			TestCategories:  []string{"security", "api", "http"},
		},
	}

	return t, nil
}

func (t *VerifyAPIHttpStatus) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management api http-commands",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get API HTTP commands status: %v", err)
		return result, nil
	}

	if apiData, ok := cmdResult.Output.(map[string]any); ok {
		if httpServer, ok := apiData["httpServer"].(map[string]any); ok {
			if running, ok := httpServer["running"].(bool); ok {
				if running {
					result.Status = test.TestFailure
					result.Message = "eAPI HTTP server is running - should be disabled for security"
				} else {
					result.Details = map[string]any{
						"http_server_status": "disabled",
					}
				}
				return result, nil
			}
		}
	}

	result.Status = test.TestError
	result.Message = "Could not determine eAPI HTTP server status from device response"
	return result, nil
}

func (t *VerifyAPIHttpStatus) ValidateInput(input any) error {
	return nil
}

// VerifyAPIHttpsSSL verifies the eAPI HTTPS server SSL profile configuration.
//
// Expected Results:
//   - Success: The test will pass if the eAPI HTTPS server uses the specified SSL profile.
//   - Failure: The test will fail if the eAPI HTTPS server uses a different SSL profile.
//   - Error: The test will report an error if eAPI HTTPS SSL profile cannot be determined.
//
// Examples:
//   - name: VerifyAPIHttpsSSL with custom profile
//     VerifyAPIHttpsSSL:
//       profile: "mySSLProfile"
type VerifyAPIHttpsSSL struct {
	test.BaseTest
	Profile string `yaml:"profile" json:"profile"`
}

func NewVerifyAPIHttpsSSL(inputs map[string]any) (test.Test, error) {
	t := &VerifyAPIHttpsSSL{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAPIHttpsSSL",
			TestDescription: "Verify eAPI HTTPS server SSL profile",
			TestCategories:  []string{"security", "api", "https", "ssl"},
		},
	}

	if inputs != nil {
		if profile, ok := inputs["profile"].(string); ok {
			t.Profile = profile
		}
	}

	return t, nil
}

func (t *VerifyAPIHttpsSSL) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management api http-commands",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get API HTTP commands status: %v", err)
		return result, nil
	}

	if apiData, ok := cmdResult.Output.(map[string]any); ok {
		if httpsServer, ok := apiData["httpsServer"].(map[string]any); ok {
			if sslProfile, ok := httpsServer["sslProfile"].(map[string]any); ok {
				if profileName, ok := sslProfile["name"].(string); ok {
					if profileName != t.Profile {
						result.Status = test.TestFailure
						result.Message = fmt.Sprintf("eAPI HTTPS SSL profile is '%s', expected '%s'", profileName, t.Profile)
					} else {
						result.Details = map[string]any{
							"ssl_profile": profileName,
						}
					}
					return result, nil
				}
			}
		}
	}

	result.Status = test.TestError
	result.Message = "Could not determine eAPI HTTPS SSL profile from device response"
	return result, nil
}

func (t *VerifyAPIHttpsSSL) ValidateInput(input any) error {
	if t.Profile == "" {
		return fmt.Errorf("SSL profile must be specified")
	}
	return nil
}

// VerifyAPIIPv4Acl verifies if the eAPI has the correct number of IPv4 ACLs configured.
//
// Expected Results:
//   - Success: The test will pass if the eAPI has the expected number of IPv4 ACLs in the specified VRF.
//   - Failure: The test will fail if the eAPI does not have the expected number of IPv4 ACLs.
//   - Error: The test will report an error if eAPI IPv4 ACL information cannot be retrieved.
//
// Examples:
//   - name: VerifyAPIIPv4Acl with specific count
//     VerifyAPIIPv4Acl:
//       number: 2
//       vrf: "default"
type VerifyAPIIPv4Acl struct {
	test.BaseTest
	Number int    `yaml:"number" json:"number"`
	VRF    string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyAPIIPv4Acl(inputs map[string]any) (test.Test, error) {
	t := &VerifyAPIIPv4Acl{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAPIIPv4Acl",
			TestDescription: "Verify eAPI IPv4 Access Control Lists",
			TestCategories:  []string{"security", "api", "acl"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if number, ok := inputs["number"].(float64); ok {
			t.Number = int(number)
		} else if number, ok := inputs["number"].(int); ok {
			t.Number = number
		}

		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifyAPIIPv4Acl) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management api http-commands ip access-list summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get API IPv4 ACL summary: %v", err)
		return result, nil
	}

	aclCount := 0
	if aclData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := aclData["vrfs"].(map[string]any); ok {
			if vrfData, ok := vrfs[t.VRF].(map[string]any); ok {
				if ipAccessLists, ok := vrfData["ipAccessLists"].(map[string]any); ok {
					aclCount = len(ipAccessLists)
				}
			}
		}
	}

	if aclCount != t.Number {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Expected %d API IPv4 ACLs in VRF %s, found %d", t.Number, t.VRF, aclCount)
	} else {
		result.Details = map[string]any{
			"expected_acl_count": t.Number,
			"actual_acl_count":   aclCount,
			"vrf":                t.VRF,
		}
	}

	return result, nil
}

func (t *VerifyAPIIPv4Acl) ValidateInput(input any) error {
	if t.Number < 0 {
		return fmt.Errorf("number of ACLs must be non-negative")
	}
	if t.VRF == "" {
		return fmt.Errorf("VRF cannot be empty")
	}
	return nil
}

// VerifyAPIIPv6Acl verifies if the eAPI has the correct number of IPv6 ACLs configured.
//
// Expected Results:
//   - Success: The test will pass if the eAPI has the expected number of IPv6 ACLs in the specified VRF.
//   - Failure: The test will fail if the eAPI does not have the expected number of IPv6 ACLs.
//   - Error: The test will report an error if eAPI IPv6 ACL information cannot be retrieved.
//
// Examples:
//   - name: VerifyAPIIPv6Acl with specific count
//     VerifyAPIIPv6Acl:
//       number: 1
//       vrf: "default"
type VerifyAPIIPv6Acl struct {
	test.BaseTest
	Number int    `yaml:"number" json:"number"`
	VRF    string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
}

func NewVerifyAPIIPv6Acl(inputs map[string]any) (test.Test, error) {
	t := &VerifyAPIIPv6Acl{
		BaseTest: test.BaseTest{
			TestName:        "VerifyAPIIPv6Acl",
			TestDescription: "Verify eAPI IPv6 Access Control Lists",
			TestCategories:  []string{"security", "api", "acl", "ipv6"},
		},
		VRF: "default", // Default VRF
	}

	if inputs != nil {
		if number, ok := inputs["number"].(float64); ok {
			t.Number = int(number)
		} else if number, ok := inputs["number"].(int); ok {
			t.Number = number
		}

		if vrf, ok := inputs["vrf"].(string); ok {
			t.VRF = vrf
		}
	}

	return t, nil
}

func (t *VerifyAPIIPv6Acl) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show management api http-commands ipv6 access-list summary",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get API IPv6 ACL summary: %v", err)
		return result, nil
	}

	aclCount := 0
	if aclData, ok := cmdResult.Output.(map[string]any); ok {
		if vrfs, ok := aclData["vrfs"].(map[string]any); ok {
			if vrfData, ok := vrfs[t.VRF].(map[string]any); ok {
				if ipv6AccessLists, ok := vrfData["ipv6AccessLists"].(map[string]any); ok {
					aclCount = len(ipv6AccessLists)
				}
			}
		}
	}

	if aclCount != t.Number {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Expected %d API IPv6 ACLs in VRF %s, found %d", t.Number, t.VRF, aclCount)
	} else {
		result.Details = map[string]any{
			"expected_acl_count": t.Number,
			"actual_acl_count":   aclCount,
			"vrf":                t.VRF,
		}
	}

	return result, nil
}

func (t *VerifyAPIIPv6Acl) ValidateInput(input any) error {
	if t.Number < 0 {
		return fmt.Errorf("number of ACLs must be non-negative")
	}
	if t.VRF == "" {
		return fmt.Errorf("VRF cannot be empty")
	}
	return nil
}