package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

type MCPServer struct {
	ddClient *datadog.APIClient
	ctx      context.Context
}

type MCPRequest struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

func NewMCPServer() (*MCPServer, error) {
	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")

	if apiKey == "" || appKey == "" {
		return nil, fmt.Errorf("DD_API_KEY and DD_APP_KEY environment variables must be set")
	}

	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {Key: apiKey},
			"appKeyAuth": {Key: appKey},
		},
	)

	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)

	return &MCPServer{
		ddClient: apiClient,
		ctx:      ctx,
	}, nil
}

func (s *MCPServer) ListTools() []Tool {
	return []Tool{
		{
			Name:        "query_logs",
			Description: "Search and query Datadog logs with filters and time ranges",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Search query using Datadog query syntax (e.g., 'service:web status:error')",
					},
					"from": map[string]interface{}{
						"type":        "string",
						"description": "Start time in RFC3339 format or relative time (e.g., '1h', '30m'). Defaults to 1 hour ago.",
					},
					"to": map[string]interface{}{
						"type":        "string",
						"description": "End time in RFC3339 format or relative time. Defaults to now.",
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of logs to return (max 1000). Defaults to 50.",
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

func parseTimeParam(timeStr string, defaultTime time.Time) (time.Time, error) {
	if timeStr == "" {
		return defaultTime, nil
	}

	// Try parsing as RFC3339
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}

	// Try parsing as relative time (e.g., "1h", "30m")
	if duration, err := time.ParseDuration(timeStr); err == nil {
		return time.Now().Add(-duration), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s (use RFC3339 or duration like '1h')", timeStr)
}

func (s *MCPServer) QueryLogs(params map[string]interface{}) (interface{}, error) {
	query, ok := params["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Default time range: last 1 hour
	defaultFrom := time.Now().Add(-1 * time.Hour)
	defaultTo := time.Now()

	fromStr, _ := params["from"].(string)
	toStr, _ := params["to"].(string)

	from, err := parseTimeParam(fromStr, defaultFrom)
	if err != nil {
		return nil, err
	}

	to, err := parseTimeParam(toStr, defaultTo)
	if err != nil {
		return nil, err
	}

	limit := int32(50)
	if l, ok := params["limit"].(float64); ok {
		limit = int32(l)
		if limit > 1000 {
			limit = 1000
		}
	}

	// Build the logs search request
	body := datadogV2.LogsListRequest{
		Filter: &datadogV2.LogsQueryFilter{
			From:  datadog.PtrString(from.Format(time.RFC3339)),
			To:    datadog.PtrString(to.Format(time.RFC3339)),
			Query: datadog.PtrString(query),
		},
		Page: &datadogV2.LogsListRequestPage{
			Limit: datadog.PtrInt32(limit),
		},
		Sort: datadogV2.LOGSSORT_TIMESTAMP_DESCENDING.Ptr(),
	}

	api := datadogV2.NewLogsApi(s.ddClient)
	resp, _, err := api.ListLogs(s.ctx, *datadogV2.NewListLogsOptionalParameters().WithBody(body))
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	// Format the response
	logs := make([]map[string]interface{}, 0)
	if resp.Data != nil {
		for _, log := range resp.Data {
			logMap := map[string]interface{}{
				"id":        log.GetId(),
				"timestamp": log.Attributes.Timestamp,
				"message":   log.Attributes.GetMessage(),
				"status":    log.Attributes.GetStatus(),
				"service":   log.Attributes.GetService(),
				"tags":      log.Attributes.GetTags(),
			}

			// Include custom attributes if present
			if attrs := log.Attributes.GetAttributes(); attrs != nil {
				logMap["attributes"] = attrs
			}

			logs = append(logs, logMap)
		}
	}

	return map[string]interface{}{
		"logs":  logs,
		"count": len(logs),
		"query": query,
		"from":  from.Format(time.RFC3339),
		"to":    to.Format(time.RFC3339),
	}, nil
}

func (s *MCPServer) HandleRequest(req MCPRequest) MCPResponse {
	resp := MCPResponse{
		Jsonrpc: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "datadog-mcp-server",
				"version": "0.1.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{},
			},
		}

	case "tools/list":
		resp.Result = map[string]interface{}{
			"tools": s.ListTools(),
		}

	case "tools/call":
		toolName, ok := req.Params["name"].(string)
		if !ok {
			resp.Error = &MCPError{Code: -32602, Message: "tool name is required"}
			return resp
		}

		args, ok := req.Params["arguments"].(map[string]interface{})
		if !ok {
			args = make(map[string]interface{})
		}

		switch toolName {
		case "query_logs":
			result, err := s.QueryLogs(args)
			if err != nil {
				resp.Error = &MCPError{Code: -32000, Message: err.Error()}
			} else {
				resp.Result = map[string]interface{}{
					"content": []map[string]interface{}{
						{
							"type": "text",
							"text": formatLogsResult(result),
						},
					},
				}
			}
		default:
			resp.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("unknown tool: %s", toolName)}
		}

	default:
		resp.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("unknown method: %s", req.Method)}
	}

	return resp
}

func formatLogsResult(result interface{}) string {
	data, _ := json.MarshalIndent(result, "", "  ")
	return string(data)
}

func main() {
	server, err := NewMCPServer()
	if err != nil {
		log.Fatalf("Failed to initialize MCP server: %v", err)
	}

	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req MCPRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding request: %v", err)
			continue
		}

		resp := server.HandleRequest(req)
		if err := encoder.Encode(resp); err != nil {
			log.Printf("Error encoding response: %v", err)
			continue
		}
	}
}
