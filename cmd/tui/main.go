package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	// Define CLI flags
	gatewayURL := flag.String("gateway-url", env("WOLINK_URL", "http://localhost:8080"), "Gateway URL")
	adminToken := flag.String("admin-token", env("WOLINK_ADMIN_KEY", ""), "Admin API token")
	gatewayBinary := flag.String("gateway-binary", env("WOLINK_GATEWAY_PATH", ""), "Path to gateway binary")
	pollInterval := flag.Duration("poll-interval", 3*time.Second, "Health poll interval")
	flag.Parse()

	// Build config
	cfg := TUIConfig{
		GatewayURL:    *gatewayURL,
		AdminToken:    *adminToken,
		GatewayBinary: *gatewayBinary,
		PollInterval:  *pollInterval,
		ModelsDir:     "./configs/models",
		PluginsDir:    "./configs/plugins",
	}

	// Resolve working directory
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine working directory: %v\n", err)
	} else {
		cfg.WorkingDir = wd
	}

	// Diagnostic banner
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("  Wolink Gateway Manager — TUI")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  Gateway URL:       %s\n", cfg.GatewayURL)
	if cfg.AdminToken != "" {
		fmt.Printf("  Admin Token:       %s (%d chars)\n", strings.Repeat("*", 8), len(cfg.AdminToken))
	} else {
		fmt.Println("  Admin Token:       (not set — some features will be unavailable)")
	}
	if cfg.GatewayBinary != "" {
		fmt.Printf("  Gateway Binary:    %s\n", cfg.GatewayBinary)
	} else {
		fmt.Println("  Gateway Binary:    (not set — will build from source)")
	}
	fmt.Printf("  Poll Interval:     %s\n", cfg.PollInterval)
	fmt.Printf("  Working Directory: %s\n", cfg.WorkingDir)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println()
	fmt.Println("  TUI configuration loaded successfully.")
	fmt.Println("  The interactive TUI launches once Plan 02 is implemented.")
	fmt.Println()
	fmt.Println("  Usage: go run ./cmd/tui/ --gateway-url http://localhost:8080 --admin-token YOUR_TOKEN")
	fmt.Println()
	os.Exit(0)
}

func init() {
	http.DefaultClient.Timeout = 10 * time.Second
}
