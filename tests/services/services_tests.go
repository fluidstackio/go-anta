package services

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

// VerifyHostname verifies the hostname of a device.
//
// Expected Results:
//   - Success: The test will pass if the hostname matches the provided input.
//   - Failure: The test will fail if the hostname does not match the provided input.
//   - Error: The test will report an error if the hostname cannot be determined.
//
// Examples:
//   - name: VerifyHostname
//     VerifyHostname:
//       hostname: "s1-spine1"
type VerifyHostname struct {
	test.BaseTest
	Hostname string `yaml:"hostname" json:"hostname"`
}

func NewVerifyHostname(inputs map[string]any) (test.Test, error) {
	t := &VerifyHostname{
		BaseTest: test.BaseTest{
			TestName:        "VerifyHostname",
			TestDescription: "Verify the hostname of a device",
			TestCategories:  []string{"services", "hostname"},
		},
	}

	if inputs != nil {
		if hostname, ok := inputs["hostname"].(string); ok {
			t.Hostname = hostname
		}
	}

	return t, nil
}

func (t *VerifyHostname) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show hostname",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get hostname: %v", err)
		return result, nil
	}

	var currentHostname string
	if hostnameData, ok := cmdResult.Output.(map[string]any); ok {
		if hostname, ok := hostnameData["hostname"].(string); ok {
			currentHostname = hostname
		} else if fqdn, ok := hostnameData["fqdn"].(string); ok {
			// Extract hostname from FQDN
			parts := strings.Split(fqdn, ".")
			if len(parts) > 0 {
				currentHostname = parts[0]
			}
		}
	}

	if currentHostname == "" {
		result.Status = test.TestError
		result.Message = "Could not determine device hostname"
		return result, nil
	}

	if currentHostname != t.Hostname {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Hostname is '%s', expected '%s'", currentHostname, t.Hostname)
	} else {
		result.Details = map[string]any{
			"hostname": currentHostname,
		}
	}

	return result, nil
}

func (t *VerifyHostname) ValidateInput(input any) error {
	if t.Hostname == "" {
		return fmt.Errorf("hostname must be specified")
	}
	return nil
}

// VerifyDNSLookup verifies the DNS (Domain Name Service) name to IP address resolution.
//
// Expected Results:
//   - Success: The test will pass if all domain names resolve to IP addresses.
//   - Failure: The test will fail if any domain name does not resolve to an IP address.
//   - Error: The test will report an error if a domain name is invalid or DNS lookup fails.
//
// Examples:
//   - name: VerifyDNSLookup
//     VerifyDNSLookup:
//       domain_names:
//         - "arista.com"
//         - "www.google.com"
//         - "github.com"
type VerifyDNSLookup struct {
	test.BaseTest
	DomainNames []string `yaml:"domain_names" json:"domain_names"`
}

func NewVerifyDNSLookup(inputs map[string]any) (test.Test, error) {
	t := &VerifyDNSLookup{
		BaseTest: test.BaseTest{
			TestName:        "VerifyDNSLookup",
			TestDescription: "Verify the DNS name to IP address resolution",
			TestCategories:  []string{"services", "dns"},
		},
	}

	if inputs != nil {
		if domainNames, ok := inputs["domain_names"].([]any); ok {
			for _, d := range domainNames {
				if domain, ok := d.(string); ok {
					t.DomainNames = append(t.DomainNames, domain)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyDNSLookup) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	issues := []string{}
	resolvedDomains := make(map[string]string)

	for _, domain := range t.DomainNames {
		cmd := device.Command{
			Template: fmt.Sprintf("bash timeout 10 nslookup %s", domain),
			Format:   "text",
			UseCache: false,
		}

		cmdResult, err := dev.Execute(ctx, cmd)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to lookup domain %s: %v", domain, err))
			continue
		}

		if output, ok := cmdResult.Output.(string); ok {
			// Parse nslookup output
			lines := strings.Split(output, "\n")
			resolved := false
			var resolvedIP string

			for _, line := range lines {
				line = strings.TrimSpace(line)
				// Look for "Address: " followed by IP
				if strings.HasPrefix(line, "Address:") && !strings.Contains(line, "#") {
					parts := strings.Fields(line)
					if len(parts) >= 2 {
						ip := parts[1]
						if net.ParseIP(ip) != nil {
							resolved = true
							resolvedIP = ip
							break
						}
					}
				}
				// Alternative format: "Name: domain.com Address: IP"
				if strings.Contains(line, "Address:") && strings.Contains(line, domain) {
					parts := strings.Split(line, "Address:")
					if len(parts) >= 2 {
						ip := strings.TrimSpace(parts[1])
						if net.ParseIP(ip) != nil {
							resolved = true
							resolvedIP = ip
							break
						}
					}
				}
			}

			if !resolved {
				issues = append(issues, fmt.Sprintf("Could not resolve domain %s", domain))
			} else {
				resolvedDomains[domain] = resolvedIP
			}
		} else {
			issues = append(issues, fmt.Sprintf("Invalid nslookup output format for domain %s", domain))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("DNS lookup issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"resolved_domains": resolvedDomains,
		}
	}

	return result, nil
}

func (t *VerifyDNSLookup) ValidateInput(input any) error {
	if len(t.DomainNames) == 0 {
		return fmt.Errorf("at least one domain name must be specified")
	}
	return nil
}

// VerifyDNSServers verifies if the DNS servers are correctly configured.
//
// Expected Results:
//   - Success: The test will pass if all specified DNS servers are configured with correct VRF and priority.
//   - Failure: The test will fail if any DNS server is missing or has incorrect configuration.
//   - Error: The test will report an error if DNS server configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyDNSServers with VRF and priority
//     VerifyDNSServers:
//       dns_servers:
//         - server: "8.8.8.8"
//           vrf: "default"
//           priority: 1
//         - server: "8.8.4.4"
//           vrf: "default"
//           priority: 2
type VerifyDNSServers struct {
	test.BaseTest
	DNSServers []DNSServer `yaml:"dns_servers" json:"dns_servers"`
}

type DNSServer struct {
	Server   string `yaml:"server" json:"server"`
	VRF      string `yaml:"vrf,omitempty" json:"vrf,omitempty"`
	Priority int    `yaml:"priority,omitempty" json:"priority,omitempty"`
}

func NewVerifyDNSServers(inputs map[string]any) (test.Test, error) {
	t := &VerifyDNSServers{
		BaseTest: test.BaseTest{
			TestName:        "VerifyDNSServers",
			TestDescription: "Verify if the DNS servers are correctly configured",
			TestCategories:  []string{"services", "dns"},
		},
	}

	if inputs != nil {
		if dnsServers, ok := inputs["dns_servers"].([]any); ok {
			for _, s := range dnsServers {
				if serverMap, ok := s.(map[string]any); ok {
					server := DNSServer{
						VRF: "default", // Default VRF
					}

					if addr, ok := serverMap["server"].(string); ok {
						server.Server = addr
					}
					if vrf, ok := serverMap["vrf"].(string); ok {
						server.VRF = vrf
					}
					if priority, ok := serverMap["priority"].(float64); ok {
						server.Priority = int(priority)
					} else if priority, ok := serverMap["priority"].(int); ok {
						server.Priority = priority
					}

					if server.Server != "" {
						t.DNSServers = append(t.DNSServers, server)
					}
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyDNSServers) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show ip name-server",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get DNS servers: %v", err)
		return result, nil
	}

	issues := []string{}
	configuredServers := make(map[string]DNSServer)

	if dnsData, ok := cmdResult.Output.(map[string]any); ok {
		if servers, ok := dnsData["nameServerConfigs"].([]any); ok {
			for _, serverData := range servers {
				if server, ok := serverData.(map[string]any); ok {
					var configuredServer DNSServer

					if addr, ok := server["ipAddr"].(string); ok {
						configuredServer.Server = addr
					}
					if vrf, ok := server["vrf"].(string); ok {
						configuredServer.VRF = vrf
					} else {
						configuredServer.VRF = "default"
					}
					if priority, ok := server["priority"].(float64); ok {
						configuredServer.Priority = int(priority)
					}

					if configuredServer.Server != "" {
						key := fmt.Sprintf("%s:%s", configuredServer.Server, configuredServer.VRF)
						configuredServers[key] = configuredServer
					}
				}
			}
		}
	}

	// Check each expected DNS server
	for _, expectedServer := range t.DNSServers {
		key := fmt.Sprintf("%s:%s", expectedServer.Server, expectedServer.VRF)
		configuredServer, found := configuredServers[key]

		if !found {
			issues = append(issues, fmt.Sprintf("DNS server %s not configured in VRF %s", expectedServer.Server, expectedServer.VRF))
			continue
		}

		// Check priority if specified
		if expectedServer.Priority > 0 && configuredServer.Priority != expectedServer.Priority {
			issues = append(issues, fmt.Sprintf("DNS server %s in VRF %s has priority %d, expected %d",
				expectedServer.Server, expectedServer.VRF, configuredServer.Priority, expectedServer.Priority))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("DNS server configuration issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"expected_servers":   t.DNSServers,
			"configured_servers": configuredServers,
		}
	}

	return result, nil
}

func (t *VerifyDNSServers) ValidateInput(input any) error {
	if len(t.DNSServers) == 0 {
		return fmt.Errorf("at least one DNS server must be specified")
	}

	for i, server := range t.DNSServers {
		if server.Server == "" {
			return fmt.Errorf("DNS server at index %d has empty server address", i)
		}
		if net.ParseIP(server.Server) == nil {
			return fmt.Errorf("DNS server at index %d has invalid IP address: %s", i, server.Server)
		}
	}

	return nil
}

// VerifyErrdisableRecovery verifies the error disable recovery functionality.
//
// Expected Results:
//   - Success: The test will pass if all specified errdisable recovery reasons are configured
//     with the expected status and interval.
//   - Failure: The test will fail if any errdisable recovery reason is missing or has incorrect configuration.
//   - Error: The test will report an error if errdisable recovery configuration cannot be retrieved.
//
// Examples:
//   - name: VerifyErrdisableRecovery with specific reasons
//     VerifyErrdisableRecovery:
//       reasons:
//         - reason: "bpduguard"
//           status: "enabled"
//           interval: 300
//         - reason: "link-flap"
//           status: "enabled"
type VerifyErrdisableRecovery struct {
	test.BaseTest
	Reasons []ErrdisableRecovery `yaml:"reasons" json:"reasons"`
}

type ErrdisableRecovery struct {
	Reason   string `yaml:"reason" json:"reason"`
	Interval int    `yaml:"interval,omitempty" json:"interval,omitempty"`
	Status   string `yaml:"status,omitempty" json:"status,omitempty"`
}

func NewVerifyErrdisableRecovery(inputs map[string]any) (test.Test, error) {
	t := &VerifyErrdisableRecovery{
		BaseTest: test.BaseTest{
			TestName:        "VerifyErrdisableRecovery",
			TestDescription: "Verify the error disable recovery functionality",
			TestCategories:  []string{"services", "errdisable"},
		},
	}

	if inputs != nil {
		if reasons, ok := inputs["reasons"].([]any); ok {
			for _, r := range reasons {
				if reasonMap, ok := r.(map[string]any); ok {
					reason := ErrdisableRecovery{
						Status: "enabled", // Default status
					}

					if name, ok := reasonMap["reason"].(string); ok {
						reason.Reason = name
					}
					if interval, ok := reasonMap["interval"].(float64); ok {
						reason.Interval = int(interval)
					} else if interval, ok := reasonMap["interval"].(int); ok {
						reason.Interval = interval
					}
					if status, ok := reasonMap["status"].(string); ok {
						reason.Status = status
					}

					if reason.Reason != "" {
						t.Reasons = append(t.Reasons, reason)
					}
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyErrdisableRecovery) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show errdisable recovery",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get errdisable recovery configuration: %v", err)
		return result, nil
	}

	issues := []string{}
	configuredReasons := make(map[string]ErrdisableRecovery)

	if errdisableData, ok := cmdResult.Output.(map[string]any); ok {
		if reasons, ok := errdisableData["errdisableRecoveryReasons"].(map[string]any); ok {
			for reasonName, reasonData := range reasons {
				if reason, ok := reasonData.(map[string]any); ok {
					var configuredReason ErrdisableRecovery
					configuredReason.Reason = reasonName

					if enabled, ok := reason["enabled"].(bool); ok {
						if enabled {
							configuredReason.Status = "enabled"
						} else {
							configuredReason.Status = "disabled"
						}
					}

					if interval, ok := reason["interval"].(float64); ok {
						configuredReason.Interval = int(interval)
					}

					configuredReasons[reasonName] = configuredReason
				}
			}
		}
	}

	// Check each expected reason
	for _, expectedReason := range t.Reasons {
		configuredReason, found := configuredReasons[expectedReason.Reason]

		if !found {
			issues = append(issues, fmt.Sprintf("Errdisable recovery reason '%s' not found", expectedReason.Reason))
			continue
		}

		// Check status
		if expectedReason.Status != "" && configuredReason.Status != expectedReason.Status {
			issues = append(issues, fmt.Sprintf("Errdisable recovery reason '%s' status is '%s', expected '%s'",
				expectedReason.Reason, configuredReason.Status, expectedReason.Status))
		}

		// Check interval if specified
		if expectedReason.Interval > 0 && configuredReason.Interval != expectedReason.Interval {
			issues = append(issues, fmt.Sprintf("Errdisable recovery reason '%s' interval is %d, expected %d",
				expectedReason.Reason, configuredReason.Interval, expectedReason.Interval))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("Errdisable recovery issues: %v", issues)
	} else {
		result.Details = map[string]any{
			"expected_reasons":   t.Reasons,
			"configured_reasons": configuredReasons,
		}
	}

	return result, nil
}

func (t *VerifyErrdisableRecovery) ValidateInput(input any) error {
	if len(t.Reasons) == 0 {
		return fmt.Errorf("at least one errdisable recovery reason must be specified")
	}

	for i, reason := range t.Reasons {
		if reason.Reason == "" {
			return fmt.Errorf("errdisable recovery reason at index %d has empty reason name", i)
		}
		if reason.Status != "" && reason.Status != "enabled" && reason.Status != "disabled" {
			return fmt.Errorf("errdisable recovery reason at index %d has invalid status '%s' (must be 'enabled' or 'disabled')", i, reason.Status)
		}
		if reason.Interval < 0 {
			return fmt.Errorf("errdisable recovery reason at index %d has negative interval", i)
		}
	}

	return nil
}