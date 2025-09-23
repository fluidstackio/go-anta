package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	// Create the same JSON payload that GANTA would send
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "runCmds",
		"params": map[string]interface{}{
			"version": 1,
			"cmds": []map[string]interface{}{
				{
					"cmd":     "show version",
					"version": 1,
					"format":  "json",
				},
			},
			"format": "json",
		},
		"id": 1,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		return
	}

	fmt.Println("=== JSON Payload ===")
	fmt.Println(string(jsonData))

	fmt.Println("\n=== Pretty JSON ===")
	var prettyJSON map[string]interface{}
	json.Unmarshal(jsonData, &prettyJSON)
	pretty, _ := json.MarshalIndent(prettyJSON, "", "  ")
	fmt.Println(string(pretty))

	fmt.Println("\n=== Curl Command ===")
	fmt.Printf("curl -k -X POST https://192.168.1.5:443/command-api \\\n")
	fmt.Printf("  -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("  -u \"admin:YOUR_PASSWORD\" \\\n")
	fmt.Printf("  -d '%s'\n", string(jsonData))

	fmt.Println("\n=== Alternative with timeout ===")
	fmt.Printf("curl -k -X POST https://192.168.1.5:443/command-api \\\n")
	fmt.Printf("  -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("  -u \"admin:YOUR_PASSWORD\" \\\n")
	fmt.Printf("  -d '%s' \\\n", string(jsonData))
	fmt.Printf("  --connect-timeout 10 \\\n")
	fmt.Printf("  --max-time 30\n")
}