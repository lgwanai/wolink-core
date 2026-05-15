package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "chat":
		cmdChat()
	case "models":
		cmdModels()
	case "status":
		cmdStatus()
	case "log":
		cmdLog()
	case "tokens":
		cmdTokens()
	case "logs":
		cmdLogsSearch()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`wolink-cli - Gateway CLI tool

Usage:
  wolink-cli chat <model> <message>         Send chat message
  wolink-cli models                          List available models
  wolink-cli status                          Gateway node status
  wolink-cli log <track_id>                  View communication log by track_id
  wolink-cli tokens <track_id>               View token stats by track_id
  wolink-cli logs search <api_key_id>        Search token records by API key

Environment:
  WOLINK_URL       Gateway URL (default: http://localhost:8080)
  WOLINK_API_KEY   API key for model requests
  WOLINK_ADMIN_KEY Admin token for admin endpoints
  WOLINK_LOGS      Log directory (default: ./logs)`)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func gwURL() string     { return env("WOLINK_URL", "http://localhost:8080") }
func apiKeyVal() string  { return env("WOLINK_API_KEY", "ak-dev-default") }
func adminToken() string { return env("WOLINK_ADMIN_KEY", "your-admin-token-change-me-in-production") }
func logsPath() string   { return env("WOLINK_LOGS", "./logs") }

func cmdChat() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: wolink-cli chat <model> <message>")
		return
	}
	model := os.Args[2]
	message := strings.Join(os.Args[3:], " ")

	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": message},
		},
		"stream": false,
	}
	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", gwURL()+"/v1/chat/completions", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKeyVal())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	respData, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		fmt.Printf("Error %d: %s\n", resp.StatusCode, string(respData))
		return
	}

	var result map[string]interface{}
	json.Unmarshal(respData, &result)

	if choices, ok := result["choices"].([]interface{}); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]interface{}); ok {
			if msg, ok := choice["message"].(map[string]interface{}); ok {
				fmt.Println(msg["content"])
			}
		}
	}
	if usage, ok := result["usage"]; ok {
		u, _ := json.MarshalIndent(usage, "", "  ")
		fmt.Println("\n---\nUsage:", string(u))
	}
}

func cmdModels() {
	req, _ := http.NewRequest("GET", gwURL()+"/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKeyVal())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if data, ok := result["data"].([]interface{}); ok {
		fmt.Printf("Models (%d):\n", len(data))
		for _, m := range data {
			if model, ok := m.(map[string]interface{}); ok {
				fmt.Printf("  - %s\n", model["id"])
			}
		}
	}
}

func cmdStatus() {
	req, _ := http.NewRequest("GET", gwURL()+"/admin/node/status", nil)
	req.Header.Set("X-Admin-Token", adminToken())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if data, ok := result["data"]; ok {
		d, _ := json.MarshalIndent(data, "", "  ")
		fmt.Println(string(d))
	} else {
		d, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(d))
	}
}

func cmdLog() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: wolink-cli log <track_id>")
		return
	}
	dir := filepath.Join(logsPath(), "communications")
	findInJSONL(dir, "track_id", os.Args[2])
}

func cmdTokens() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: wolink-cli tokens <track_id>")
		return
	}
	dir := filepath.Join(logsPath(), "tokens")
	findInJSONL(dir, "track_id", os.Args[2])
}

func cmdLogsSearch() {
	if len(os.Args) < 4 || os.Args[2] != "search" {
		fmt.Println("Usage: wolink-cli logs search <api_key_id>")
		return
	}
	dir := filepath.Join(logsPath(), "tokens")
	findInJSONL(dir, "api_key_id", os.Args[3])
}

func findInJSONL(dir, key, value string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	found := false
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		f, err := os.Open(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var record map[string]interface{}
			if json.Unmarshal([]byte(line), &record) != nil {
				continue
			}
			if v, ok := record[key]; ok && fmt.Sprint(v) == value {
				found = true
				d, _ := json.MarshalIndent(record, "", "  ")
				fmt.Println(string(d))
				fmt.Println("---")
			}
		}
	}
	if !found {
		fmt.Printf("No records found for %s=%s\n", key, value)
	}
}

func init() {
	http.DefaultClient.Timeout = 30 * time.Second
}
