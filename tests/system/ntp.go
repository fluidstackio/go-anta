package system

import (
	"context"
	"fmt"
	"strings"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/test"
)

type VerifyNTP struct {
	test.BaseTest
	Servers []NTPServer `yaml:"servers" json:"servers"`
}

type NTPServer struct {
	Server       string `yaml:"server" json:"server"`
	Synchronized bool   `yaml:"synchronized" json:"synchronized"`
	Stratum      int    `yaml:"stratum,omitempty" json:"stratum,omitempty"`
}

func NewVerifyNTP(inputs map[string]interface{}) (test.Test, error) {
	t := &VerifyNTP{
		BaseTest: test.BaseTest{
			TestName:        "VerifyNTP",
			TestDescription: "Verify NTP synchronization status",
			TestCategories:  []string{"system", "time"},
		},
	}

	if inputs != nil {
		if servers, ok := inputs["servers"].([]interface{}); ok {
			for _, s := range servers {
				if serverMap, ok := s.(map[string]interface{}); ok {
					server := NTPServer{
						Synchronized: true,
					}
					if addr, ok := serverMap["server"].(string); ok {
						server.Server = addr
					}
					if sync, ok := serverMap["synchronized"].(bool); ok {
						server.Synchronized = sync
					}
					if stratum, ok := serverMap["stratum"].(float64); ok {
						server.Stratum = int(stratum)
					} else if stratum, ok := serverMap["stratum"].(int); ok {
						server.Stratum = stratum
					}
					t.Servers = append(t.Servers, server)
				}
			}
		}
	}

	return t, nil
}

func (t *VerifyNTP) Execute(ctx context.Context, dev device.Device) (*test.TestResult, error) {
	result := &test.TestResult{
		TestName:   t.Name(),
		DeviceName: dev.Name(),
		Status:     test.TestSuccess,
		Categories: t.Categories(),
	}

	cmd := device.Command{
		Template: "show ntp associations",
		Format:   "json",
		UseCache: false,
	}

	cmdResult, err := dev.Execute(ctx, cmd)
	if err != nil {
		result.Status = test.TestError
		result.Message = fmt.Sprintf("Failed to get NTP associations: %v", err)
		return result, nil
	}

	issues := []string{}
	ntpServers := make(map[string]NTPAssociation)

	if ntpData, ok := cmdResult.Output.(map[string]interface{}); ok {
		if peers, ok := ntpData["peers"].([]interface{}); ok {
			for _, p := range peers {
				if peer, ok := p.(map[string]interface{}); ok {
					assoc := NTPAssociation{}
					
					if peerAddr, ok := peer["peerAddress"].(string); ok {
						assoc.PeerAddress = peerAddr
					}
					if condition, ok := peer["condition"].(string); ok {
						assoc.Condition = condition
					}
					if stratum, ok := peer["stratum"].(float64); ok {
						assoc.Stratum = int(stratum)
					}
					
					if assoc.PeerAddress != "" {
						ntpServers[assoc.PeerAddress] = assoc
					}
				}
			}
		}
	}

	if len(t.Servers) == 0 && len(ntpServers) == 0 {
		result.Status = test.TestFailure
		result.Message = "No NTP servers configured"
		return result, nil
	}

	for _, expectedServer := range t.Servers {
		found := false
		for addr, assoc := range ntpServers {
			if strings.Contains(addr, expectedServer.Server) || strings.Contains(expectedServer.Server, addr) {
				found = true
				
				isSynchronized := strings.Contains(assoc.Condition, "sys.peer") || 
								strings.Contains(assoc.Condition, "candidate")
				
				if expectedServer.Synchronized && !isSynchronized {
					issues = append(issues, fmt.Sprintf("Server %s is not synchronized (condition: %s)",
						expectedServer.Server, assoc.Condition))
				} else if !expectedServer.Synchronized && isSynchronized {
					issues = append(issues, fmt.Sprintf("Server %s is unexpectedly synchronized",
						expectedServer.Server))
				}
				
				if expectedServer.Stratum > 0 && assoc.Stratum != expectedServer.Stratum {
					issues = append(issues, fmt.Sprintf("Server %s: expected stratum %d, got %d",
						expectedServer.Server, expectedServer.Stratum, assoc.Stratum))
				}
				
				break
			}
		}
		
		if !found {
			issues = append(issues, fmt.Sprintf("NTP server %s not found", expectedServer.Server))
		}
	}

	if len(issues) > 0 {
		result.Status = test.TestFailure
		result.Message = fmt.Sprintf("NTP issues: %v", issues)
	}

	return result, nil
}

func (t *VerifyNTP) ValidateInput(input interface{}) error {
	for i, server := range t.Servers {
		if server.Server == "" {
			return fmt.Errorf("NTP server at index %d has no address", i)
		}
	}
	return nil
}

type NTPAssociation struct {
	PeerAddress string
	Condition   string
	Stratum     int
}