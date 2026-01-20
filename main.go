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
	Params  json.RawMessage `json:"params,omitempty"`
	ID      int             `json:"id"`
	Jsonrpc string          `json:"jsonrpc"`
	Method  string          `json:"method"`
}

type MCPResponse struct {
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	Jsonrpc string          `json:"jsonrpc"`
}

type MCPError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

type SchemaProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Items       *SchemaProperty `json:"items,omitempty"`
}

type InputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]SchemaProperty `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type Tool struct {
	InputSchema InputSchema `json:"inputSchema"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type QueryLogsParams struct {
	Query string `json:"query"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
	Limit int32  `json:"limit,omitempty"`
}

type LogEntry struct {
	ID        string     `json:"id"`
	Timestamp *time.Time `json:"timestamp"`
	Message   string     `json:"message"`
	Status    string     `json:"status"`
	Service   string     `json:"service"`
	Tags      []string   `json:"tags"`
}

type QueryLogsResult struct {
	Logs  []LogEntry `json:"logs"`
	Count int        `json:"count"`
	Query string     `json:"query"`
	From  string     `json:"from"`
	To    string     `json:"to"`
}

type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	ServerInfo      ServerInfo       `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerCapabilities struct {
	Tools ToolsCapability `json:"tools"`
}

type ToolsCapability struct{}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult struct {
	Content []TextContent `json:"content"`
}

func NewMCPServer() (*MCPServer, error) {
	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")
	site := os.Getenv("DD_SITE") // Optional: datadoghq.com (default), datadoghq.eu, us3.datadoghq.com, etc.

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

	// Configure site/region if specified
	if site != "" {
		ctx = context.WithValue(ctx, datadog.ContextServerVariables, map[string]string{
			"site": site,
		})
		log.Printf("Using Datadog site: %s", site)
	}

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
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]SchemaProperty{
					"query": {
						Type:        "string",
						Description: "Search query using Datadog query syntax (e.g., 'service:web status:error')",
					},
					"from": {
						Type:        "string",
						Description: "Start time in RFC3339 format or relative time (e.g., '1h', '30m'). Defaults to 1 hour ago.",
					},
					"to": {
						Type:        "string",
						Description: "End time in RFC3339 format or relative time. Defaults to now.",
					},
					"limit": {
						Type:        "integer",
						Description: "Maximum number of logs to return (max 1000). Defaults to 50.",
					},
				},
				Required: []string{"query"},
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

func (s *MCPServer) QueryLogs(params QueryLogsParams) (*QueryLogsResult, error) {
	if params.Query == "" {
		return nil, fmt.Errorf("query parameter is required")
	}

	// Default time range: last 1 hour
	defaultFrom := time.Now().Add(-1 * time.Hour)
	defaultTo := time.Now()

	from, err := parseTimeParam(params.From, defaultFrom)
	if err != nil {
		return nil, err
	}

	to, err := parseTimeParam(params.To, defaultTo)
	if err != nil {
		return nil, err
	}

	limit := int32(50)
	if params.Limit > 0 {
		limit = params.Limit
		if limit > 1000 {
			limit = 1000
		}
	}

	// Build the logs search request
	body := datadogV2.LogsListRequest{
		Filter: &datadogV2.LogsQueryFilter{
			From:  datadog.PtrString(from.Format(time.RFC3339)),
			To:    datadog.PtrString(to.Format(time.RFC3339)),
			Query: datadog.PtrString(params.Query),
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
	logs := make([]LogEntry, 0)
	if resp.Data != nil {
		for _, log := range resp.Data {
			entry := LogEntry{
				ID:        log.GetId(),
				Timestamp: log.Attributes.Timestamp,
				Message:   log.Attributes.GetMessage(),
				Status:    log.Attributes.GetStatus(),
				Service:   log.Attributes.GetService(),
				Tags:      log.Attributes.GetTags(),
			}
			logs = append(logs, entry)
		}
	}

	return &QueryLogsResult{
		Logs:  logs,
		Count: len(logs),
		Query: params.Query,
		From:  from.Format(time.RFC3339),
		To:    to.Format(time.RFC3339),
	}, nil
}

func (s *MCPServer) HandleRequest(req MCPRequest) MCPResponse {
	resp := MCPResponse{
		Jsonrpc: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		result := InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    "datadog-mcp-server",
				Version: "0.1.0",
			},
			Capabilities: ServerCapabilities{
				Tools: ToolsCapability{},
			},
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			resp.Error = &MCPError{Code: -32603, Message: fmt.Sprintf("failed to marshal result: %v", err)}
			return resp
		}
		resp.Result = resultJSON

	case "tools/list":
		result := ToolsListResult{
			Tools: s.ListTools(),
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			resp.Error = &MCPError{Code: -32603, Message: fmt.Sprintf("failed to marshal result: %v", err)}
			return resp
		}
		resp.Result = resultJSON

	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &MCPError{Code: -32602, Message: fmt.Sprintf("invalid params: %v", err)}
			return resp
		}

		if params.Name == "" {
			resp.Error = &MCPError{Code: -32602, Message: "tool name is required"}
			return resp
		}

		switch params.Name {
		case "query_logs":
			var queryParams QueryLogsParams
			if err := json.Unmarshal(params.Arguments, &queryParams); err != nil {
				resp.Error = &MCPError{Code: -32602, Message: fmt.Sprintf("invalid arguments: %v", err)}
				return resp
			}

			result, err := s.QueryLogs(queryParams)
			if err != nil {
				resp.Error = &MCPError{Code: -32000, Message: err.Error()}
				return resp
			}

			toolResult := ToolCallResult{
				Content: []TextContent{
					{
						Type: "text",
						Text: formatLogsResult(result),
					},
				},
			}
			resultJSON, err := json.Marshal(toolResult)
			if err != nil {
				resp.Error = &MCPError{Code: -32603, Message: fmt.Sprintf("failed to marshal result: %v", err)}
				return resp
			}
			resp.Result = resultJSON

		default:
			resp.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("unknown tool: %s", params.Name)}
		}

	default:
		resp.Error = &MCPError{Code: -32601, Message: fmt.Sprintf("unknown method: %s", req.Method)}
	}

	return resp
}

func formatLogsResult(result *QueryLogsResult) string {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf(`{"error": "failed to format result: %v"}`, err)
	}
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
