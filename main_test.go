package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseTimeParam(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "empty string uses default",
			input:       "",
			expectError: false,
		},
		{
			name:        "valid RFC3339",
			input:       "2026-01-20T10:00:00Z",
			expectError: false,
		},
		{
			name:        "valid duration - hours",
			input:       "1h",
			expectError: false,
		},
		{
			name:        "valid duration - minutes",
			input:       "30m",
			expectError: false,
		},
		{
			name:        "valid duration - hours and minutes",
			input:       "1h30m",
			expectError: false,
		},
		{
			name:        "invalid format",
			input:       "not-a-time",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultTime := time.Now()
			_, err := parseTimeParam(tt.input, defaultTime)
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseTimeParamValues(t *testing.T) {
	// Test that durations work correctly
	now := time.Now()
	result, err := parseTimeParam("1h", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be approximately 1 hour before now
	expected := now.Add(-1 * time.Hour)
	diff := expected.Sub(result)
	if diff > time.Second || diff < -time.Second {
		t.Errorf("expected time around %v, got %v (diff: %v)", expected, result, diff)
	}
}

func TestMCPServerListTools(t *testing.T) {
	// Create a server without API keys (we're just testing tool listing)
	server := &MCPServer{}

	tools := server.ListTools()

	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	// Check that query_logs tool exists
	var queryLogsTool *Tool
	for i := range tools {
		if tools[i].Name == "query_logs" {
			queryLogsTool = &tools[i]
			break
		}
	}

	if queryLogsTool == nil {
		t.Fatal("query_logs tool not found")
		return
	}

	if queryLogsTool.Description == "" {
		t.Error("query_logs tool should have a description")
	}

	if queryLogsTool.InputSchema.Type == "" {
		t.Error("query_logs tool should have an input schema")
	}
}

func TestHandleInitializeRequest(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "initialize",
	}

	resp := server.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}

	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}

	// Unmarshal and check the result
	var result InitializeResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result.ProtocolVersion == "" {
		t.Error("expected protocolVersion in result")
	}

	if result.ServerInfo.Name == "" {
		t.Error("expected serverInfo.name in result")
	}
}

func TestHandleToolsListRequest(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := server.HandleRequest(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error.Message)
	}

	if resp.Result == nil {
		t.Fatal("expected result, got nil")
	}

	// Unmarshal and check the result
	var result ToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if len(result.Tools) == 0 {
		t.Error("expected at least one tool")
	}
}

func TestHandleUnknownMethod(t *testing.T) {
	server := &MCPServer{}

	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      3,
		Method:  "unknown/method",
	}

	resp := server.HandleRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestHandleToolsCallWithoutName(t *testing.T) {
	server := &MCPServer{}

	params, _ := json.Marshal(map[string]string{
		// Missing "name" parameter
		"arguments": "{}",
	})

	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  params,
	}

	resp := server.HandleRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error when tool name is missing")
	}

	if resp.Error.Code != -32602 {
		t.Errorf("expected error code -32602, got %d", resp.Error.Code)
	}
}

func TestHandleToolsCallUnknownTool(t *testing.T) {
	server := &MCPServer{}

	params, _ := json.Marshal(ToolCallParams{
		Name:      "unknown_tool",
		Arguments: json.RawMessage(`{}`),
	})

	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params:  params,
	}

	resp := server.HandleRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestFormatLogsResult(t *testing.T) {
	input := &QueryLogsResult{
		Logs: []LogEntry{
			{
				ID:      "test-id",
				Message: "test message",
			},
		},
		Count: 1,
		Query: "test query",
		From:  "2026-01-20T00:00:00Z",
		To:    "2026-01-20T01:00:00Z",
	}

	result := formatLogsResult(input)

	if result == "" {
		t.Error("expected non-empty formatted result")
	}

	// Verify it's valid JSON
	var parsed QueryLogsResult
	err := json.Unmarshal([]byte(result), &parsed)
	if err != nil {
		t.Errorf("formatted result should be valid JSON: %v", err)
	}
}

func TestMCPRequestUnmarshal(t *testing.T) {
	jsonStr := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {"test": "value"}
	}`

	var req MCPRequest
	err := json.Unmarshal([]byte(jsonStr), &req)
	if err != nil {
		t.Fatalf("failed to unmarshal MCPRequest: %v", err)
	}

	if req.Jsonrpc != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got '%s'", req.Jsonrpc)
	}

	if req.Method != "initialize" {
		t.Errorf("expected method 'initialize', got '%s'", req.Method)
	}
}

func TestMCPResponseMarshal(t *testing.T) {
	result, _ := json.Marshal(map[string]string{"status": "ok"})

	resp := MCPResponse{
		Jsonrpc: "2.0",
		ID:      1,
		Result:  result,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal MCPResponse: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}

	// Verify it unmarshals correctly
	var parsed MCPResponse
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
	}
}
