package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/fluidstackio/go-anta/pkg/device"
)

// debug is a thin wrapper around device.New that runs `show version`
// against a single device. Useful for verifying connectivity for either
// transport (eapi or gnmi) outside the full nrfu flow.
//
// Examples:
//
//	debug --host 192.0.2.10 --user admin --pass admin
//	debug --host fc00:800f:f01::8 --user admin --pass admin --transport gnmi
//
// DEVICE_PASSWORD env var is used if --pass is omitted, which keeps the
// secret out of your shell history.
func main() {
	host := flag.String("host", "", "device host (IP or hostname)")
	user := flag.String("user", "admin", "username")
	pass := flag.String("pass", "", "password (or set DEVICE_PASSWORD env var)")
	port := flag.Int("port", 0, "device port (default 443 for eapi, 6030 for gnmi)")
	transport := flag.String("transport", "eapi", "transport: eapi or gnmi")
	insecure := flag.Bool("insecure", true, "skip TLS verification")
	cmdStr := flag.String("cmd", "show version", "CLI command to run")
	flag.Parse()

	if *host == "" {
		fmt.Fprintln(os.Stderr, "--host is required")
		flag.Usage()
		os.Exit(2)
	}
	password := *pass
	if password == "" {
		password = os.Getenv("DEVICE_PASSWORD")
	}

	dev, err := device.New(device.DeviceConfig{
		Name:      "debug",
		Host:      *host,
		Port:      *port,
		Username:  *user,
		Password:  password,
		Transport: *transport,
		Insecure:  *insecure,
		Timeout:   10 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "device.New: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := dev.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Connect: %v\n", err)
		os.Exit(1)
	}
	defer dev.Disconnect()

	result, err := dev.Execute(ctx, device.Command{Template: *cmdStr})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Execute %q: %v\n", *cmdStr, err)
		os.Exit(1)
	}

	// Pretty-print the JSON output; fall back to %+v for non-JSON Output.
	if b, err := json.MarshalIndent(result.Output, "", "  "); err == nil {
		fmt.Println(string(b))
	} else {
		fmt.Printf("%+v\n", result.Output)
	}
}
