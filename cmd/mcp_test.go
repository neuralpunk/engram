package cmd

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"engram/internal/config"
	"engram/internal/db"
)

// mcpExchange sends a JSON-RPC line and returns the parsed response.
// Returns nil response if no output was produced (for notifications).
func mcpExchange(t *testing.T, database *db.DB, lines ...string) []json.RawMessage {
	t.Helper()

	input := strings.Join(lines, "\n") + "\n"
	var out bytes.Buffer

	err := mcpServeWithDB(strings.NewReader(input), &out, database)
	if err != nil {
		t.Fatalf("mcpServe error: %v", err)
	}

	var responses []json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		responses = append(responses, json.RawMessage(line))
	}
	return responses
}

// mcpServeWithDB runs the MCP server loop with an already-open database.
func mcpServeWithDB(r *strings.Reader, w *bytes.Buffer, database *db.DB) error {
	cfg := config.DefaultConfig()
	return mcpServeInternal(r, w, database, &cfg)
}

func mustParseResponse(t *testing.T, raw json.RawMessage) map[string]interface{} {
	t.Helper()
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, string(raw))
	}
	return m
}

func TestMCPInitialize(t *testing.T) {
	database := mustOpenTestDB(t)
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	result := resp["result"].(map[string]interface{})

	if result["protocolVersion"] != mcpProtocolVersion {
		t.Errorf("expected protocolVersion %q, got %v", mcpProtocolVersion, result["protocolVersion"])
	}

	serverInfo := result["serverInfo"].(map[string]interface{})
	if serverInfo["name"] != "engram" {
		t.Errorf("expected server name 'engram', got %v", serverInfo["name"])
	}
	if serverInfo["version"] != mcpVersion {
		t.Errorf("expected server version %q, got %v", mcpVersion, serverInfo["version"])
	}
}

func TestMCPInitializedNotification(t *testing.T) {
	database := mustOpenTestDB(t)
	// initialized has no id — it's a notification
	req := `{"jsonrpc":"2.0","method":"initialized"}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 0 {
		t.Errorf("expected no response for initialized notification, got %d responses", len(responses))
	}
}

func TestMCPToolsList(t *testing.T) {
	database := mustOpenTestDB(t)
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	result := resp["result"].(map[string]interface{})
	tools := result["tools"].([]interface{})

	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		tm := tool.(map[string]interface{})
		names[tm["name"].(string)] = true
		if tm["description"] == nil || tm["description"] == "" {
			t.Errorf("tool %q has empty description", tm["name"])
		}
		if tm["inputSchema"] == nil {
			t.Errorf("tool %q has no inputSchema", tm["name"])
		}
	}

	for _, name := range []string{"store", "search", "get"} {
		if !names[name] {
			t.Errorf("missing tool %q in tools/list", name)
		}
	}
}

func TestMCPToolsCallStore(t *testing.T) {
	database := mustOpenTestDB(t)
	req := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"store","arguments":{"fact":"Test uses Go 1.22","scope":"global","tags":"go,version"}}}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	if !strings.Contains(text, "#") {
		t.Errorf("expected store result to contain '#<id>', got: %s", text)
	}
	if result["isError"] != false {
		t.Errorf("expected isError=false, got %v", result["isError"])
	}
}

func TestMCPToolsCallSearch(t *testing.T) {
	database := mustOpenTestDB(t)

	// Store a correction first
	database.Store(&db.Correction{
		Fact:       "Project uses BurntSushi/toml",
		Scope:      "global",
		Tags:       sql.NullString{String: "toml,config,parsing", Valid: true},
		Source:     sql.NullString{String: "user", Valid: true},
		Type:       "fact",
		Confidence: 1.0,
	})

	req := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search","arguments":{"query":"toml config"}}}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	if !strings.Contains(text, "BurntSushi/toml") {
		t.Errorf("expected search to find toml correction, got: %s", text)
	}
}

func TestMCPToolsCallGet(t *testing.T) {
	database := mustOpenTestDB(t)

	database.Store(&db.Correction{
		Fact:       "Always use tabs for indentation",
		Scope:      "global",
		Source:     sql.NullString{String: "user", Valid: true},
		Type:       "preference",
		Confidence: 1.0,
	})

	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"get","arguments":{"prompt":"indentation style"}}}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	result := resp["result"].(map[string]interface{})
	content := result["content"].([]interface{})
	text := content[0].(map[string]interface{})["text"].(string)

	if !strings.Contains(text, "<engram-memory>") {
		t.Errorf("expected get result to contain <engram-memory>, got: %s", text)
	}
}

func TestMCPToolsCallUnknownTool(t *testing.T) {
	database := mustOpenTestDB(t)
	req := `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	result := resp["result"].(map[string]interface{})

	if result["isError"] != true {
		t.Errorf("expected isError=true for unknown tool, got %v", result["isError"])
	}
}

func TestMCPUnknownMethodRequest(t *testing.T) {
	database := mustOpenTestDB(t)
	req := `{"jsonrpc":"2.0","id":7,"method":"unknown/method"}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := mustParseResponse(t, responses[0])
	if resp["error"] == nil {
		t.Fatalf("expected error in response")
	}
	errObj := resp["error"].(map[string]interface{})
	if errObj["code"].(float64) != -32601 {
		t.Errorf("expected error code -32601, got %v", errObj["code"])
	}
}

func TestMCPUnknownMethodNotification(t *testing.T) {
	database := mustOpenTestDB(t)
	// No id = notification
	req := `{"jsonrpc":"2.0","method":"unknown/notification"}`
	responses := mcpExchange(t, database, req)

	if len(responses) != 0 {
		t.Errorf("expected no response for unknown notification, got %d", len(responses))
	}
}

func mustOpenTestDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}
