package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
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

	// Start the Bubble Tea TUI
	startTUI(cfg)
}

func init() {
	http.DefaultClient.Timeout = 10 * time.Second
}
