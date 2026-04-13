package cmd

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"engram/internal/config"
	"engram/internal/db"
	"engram/internal/format"
	"engram/internal/project"
)

const mcpVersion = "0.5.0"
const mcpProtocolVersion = "2024-11-05"

// jsonrpcMessage is a generic JSON-RPC 2.0 message (request or notification).
type jsonrpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

// jsonrpcResponse is a JSON-RPC 2.0 response.
type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// jsonrpcError is a JSON-RPC 2.0 error object.
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// toolCallParams is the params for tools/call.
type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// toolResult is the result shape for tools/call responses.
type toolResult struct {
	Content []toolContent `json:"content"`
	IsError bool          `json:"isError"`
}

type toolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCP starts the MCP stdio server.
func MCP(args []string, dbPath string) error {
	return mcpServe(os.Stdin, os.Stdout, dbPath)
}

// mcpServe runs the MCP server reading from r and writing to w.
func mcpServe(r io.Reader, w io.Writer, dbPath string) error {
	database, cfg, err := openDB(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()
	return mcpServeInternal(r, w, database, cfg)
}

// mcpServeInternal runs the MCP loop with an already-open database.
func mcpServeInternal(r io.Reader, w io.Writer, database *db.DB, cfg *config.Config) error {

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg jsonrpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		isNotification := msg.ID == nil

		var resp *jsonrpcResponse
		switch msg.Method {
		case "initialize":
			resp = handleInitialize(msg.ID)
		case "initialized":
			continue
		case "tools/list":
			resp = handleToolsList(msg.ID)
		case "tools/call":
			resp = handleToolsCall(msg.ID, msg.Params, database, cfg)
		default:
			if isNotification {
				continue
			}
			resp = &jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      rawToInterface(msg.ID),
				Error:   &jsonrpcError{Code: -32601, Message: "Method not found"},
			}
		}

		if resp == nil {
			continue
		}

		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintf(os.Stderr, "engram mcp: marshal error: %v\n", err)
			continue
		}
		fmt.Fprintf(w, "%s\n", out)
		if f, ok := w.(*os.File); ok {
			f.Sync()
		}
	}

	return scanner.Err()
}

func rawToInterface(raw *json.RawMessage) interface{} {
	if raw == nil {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal(*raw, &v); err != nil {
		return nil
	}
	return v
}

func handleInitialize(id *json.RawMessage) *jsonrpcResponse {
	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      rawToInterface(id),
		Result: map[string]interface{}{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":   map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo":     map[string]interface{}{"name": "engram", "version": mcpVersion},
		},
	}
}

func handleToolsList(id *json.RawMessage) *jsonrpcResponse {
	tools := []interface{}{
		map[string]interface{}{
			"name":        "store",
			"description": "Store a correction, preference, constraint, gotcha, or process fact in engram's persistent memory.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"fact":       map[string]interface{}{"type": "string", "description": "The correction as a single atomic sentence."},
					"scope":      map[string]interface{}{"type": "string", "description": "global | project:<name> | domain:<tag>. Auto-detected from cwd if omitted."},
					"type":       map[string]interface{}{"type": "string", "enum": []string{"fact", "preference", "constraint", "gotcha", "process"}, "description": "Correction type. Default: fact."},
					"trigger":    map[string]interface{}{"type": "string", "description": "When should this correction surface? One sentence."},
					"tags":       map[string]interface{}{"type": "string", "description": "Comma-separated retrieval tags (5-10 recommended)."},
					"wrong":      map[string]interface{}{"type": "string", "description": "What was previously assumed incorrectly."},
					"supersedes": map[string]interface{}{"type": "integer", "description": "ID of the correction this replaces."},
				},
				"required": []string{"fact"},
			},
		},
		map[string]interface{}{
			"name":        "search",
			"description": "Search stored corrections by relevance. Returns scored results.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query."},
					"scope": map[string]interface{}{"type": "string", "description": "Optional scope filter."},
					"limit": map[string]interface{}{"type": "integer", "description": "Max results. Default: 10."},
				},
				"required": []string{"query"},
			},
		},
		map[string]interface{}{
			"name":        "get",
			"description": "Retrieve relevant corrections for the current context and format them as an injection block. Use this at the start of a session or when context switches.",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"prompt": map[string]interface{}{"type": "string", "description": "The current prompt or topic. Used as the search query."},
					"scope":  map[string]interface{}{"type": "string", "description": "Optional scope override."},
				},
			},
		},
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      rawToInterface(id),
		Result:  map[string]interface{}{"tools": tools},
	}
}

func handleToolsCall(id *json.RawMessage, params json.RawMessage, database *db.DB, cfg *config.Config) *jsonrpcResponse {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil {
		return mcpToolError(id, "Invalid params: "+err.Error())
	}

	var text string
	var isErr bool

	switch p.Name {
	case "store":
		text, isErr = mcpStore(p.Arguments, database)
	case "search":
		text, isErr = mcpSearch(p.Arguments, database, cfg)
	case "get":
		text, isErr = mcpGet(p.Arguments, database, cfg)
	default:
		text = fmt.Sprintf("Unknown tool: %q", p.Name)
		isErr = true
	}

	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      rawToInterface(id),
		Result: toolResult{
			Content: []toolContent{{Type: "text", Text: text}},
			IsError: isErr,
		},
	}
}

func mcpToolError(id *json.RawMessage, msg string) *jsonrpcResponse {
	return &jsonrpcResponse{
		JSONRPC: "2.0",
		ID:      rawToInterface(id),
		Result: toolResult{
			Content: []toolContent{{Type: "text", Text: "Error: " + msg}},
			IsError: true,
		},
	}
}

func mcpStore(args json.RawMessage, database *db.DB) (string, bool) {
	var p struct {
		Fact       string `json:"fact"`
		Scope      string `json:"scope"`
		Type       string `json:"type"`
		Trigger    string `json:"trigger"`
		Tags       string `json:"tags"`
		Wrong      string `json:"wrong"`
		Supersedes int64  `json:"supersedes"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "Error: " + err.Error(), true
	}
	if p.Fact == "" {
		return "Error: fact is required", true
	}

	scope := p.Scope
	if scope == "" {
		if projName, found := project.Detect("."); found {
			scope = "project:" + projName
		} else {
			scope = "global"
		}
	}

	corrType := p.Type
	if corrType == "" {
		corrType = "fact"
	}
	if !validTypes[corrType] {
		return fmt.Sprintf("Error: invalid type %q: must be one of fact, preference, constraint, gotcha, process", corrType), true
	}

	c := &db.Correction{
		Fact:         p.Fact,
		Wrong:        sql.NullString{String: p.Wrong, Valid: p.Wrong != ""},
		Scope:        scope,
		Tags:         sql.NullString{String: p.Tags, Valid: p.Tags != ""},
		Source:       sql.NullString{String: "user", Valid: true},
		Type:         corrType,
		TriggerHint:  sql.NullString{String: p.Trigger, Valid: p.Trigger != ""},
		SupersedesID: sql.NullInt64{Int64: p.Supersedes, Valid: p.Supersedes > 0},
		Confidence:   1.0,
	}

	id, err := database.Store(c)
	if err != nil {
		return "Error: " + err.Error(), true
	}

	summary := p.Fact
	if runes := []rune(summary); len(runes) > 80 {
		summary = string(runes[:80])
	}
	return fmt.Sprintf("▣ Stored correction #%d: %s", id, summary), false
}

func mcpSearch(args json.RawMessage, database *db.DB, cfg *config.Config) (string, bool) {
	var p struct {
		Query string `json:"query"`
		Scope string `json:"scope"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "Error: " + err.Error(), true
	}
	if p.Query == "" {
		return "Error: query is required", true
	}
	if p.Limit <= 0 {
		p.Limit = 10
	}

	var scopes []string
	if p.Scope != "" {
		scopes = []string{p.Scope}
	}

	results, err := database.Search(p.Query, scopes, p.Limit, cfg.Injection.MinScore)
	if err != nil {
		return "Error: " + err.Error(), true
	}
	if len(results) == 0 {
		return fmt.Sprintf("No corrections found for %q.", p.Query), false
	}

	var b strings.Builder
	for _, r := range results {
		fmt.Fprintf(&b, "#%d [%s] %s (score: %.2f)\n", r.ID, r.Scope, r.Fact, -r.Score)
	}
	return strings.TrimRight(b.String(), "\n"), false
}

func mcpGet(args json.RawMessage, database *db.DB, cfg *config.Config) (string, bool) {
	var p struct {
		Prompt string `json:"prompt"`
		Scope  string `json:"scope"`
	}
	if len(args) > 0 {
		json.Unmarshal(args, &p) // best-effort; all fields optional
	}

	detectedProject := ""
	if projName, found := project.Detect("."); found {
		detectedProject = projName
	}

	var scopes []string
	if p.Scope != "" {
		scopes = []string{p.Scope}
	} else if detectedProject != "" {
		scopes = []string{"global", "project:" + detectedProject}
	}

	var scored []db.ScoredCorrection

	if p.Prompt != "" {
		truncated := truncateRunes(p.Prompt, 2000)
		results, err := database.Search(truncated, scopes, cfg.Injection.MaxCorrections*2, cfg.Injection.MinScore)
		if err == nil && len(results) > 0 {
			scored = results
		}
	}

	if len(scored) == 0 {
		var corrections []db.Correction
		var err error
		if len(scopes) > 0 {
			corrections, err = database.ListByScopes(scopes, "", 0)
		} else {
			corrections, err = database.List("", "", 0)
		}
		if err != nil {
			return "Error: " + err.Error(), true
		}
		scored = make([]db.ScoredCorrection, len(corrections))
		for i, c := range corrections {
			scored[i] = db.ScoredCorrection{Correction: c, Score: 0}
		}
	}

	selected := format.SelectCorrections(scored, cfg.Injection.MaxCorrections, cfg.Injection.MaxTokens, detectedProject)

	if len(selected) > 0 {
		ids := make([]int64, len(selected))
		for i, c := range selected {
			ids[i] = c.ID
		}
		database.IncrementHitCounts(ids)
	}

	return format.FormatMemoryBlock(selected), false
}
