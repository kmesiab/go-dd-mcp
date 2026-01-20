# Datadog MCP Server

A Model Context Protocol (MCP) server that provides access to Datadog's API, starting with logs querying functionality.

## Features

- Query Datadog logs using natural language
- Support for time ranges and filters
- Built on official Datadog Go API client
- Simple MCP protocol integration

## Prerequisites

- Go 1.21 or higher
- Datadog API key and Application key
- Access to a Datadog account with logs

## Installation

```bash
# Clone the repository
git clone https://github.com/kmesiab/go-dd-mcp.git
cd go-dd-mcp

# Build the server
go build -o datadog-mcp-server
```

## Configuration

### Environment Variables

Set your Datadog credentials as environment variables:

```bash
export DD_API_KEY="your-api-key"
export DD_APP_KEY="your-application-key"
export DD_SITE="datadoghq.com"  # Optional: defaults to datadoghq.com if not set
```

**Obtaining Credentials:**
You can obtain these keys from your Datadog account:
- API Key: Organization Settings > API Keys
- Application Key: Organization Settings > Application Keys

**Regional Sites:**
If your organization uses a different Datadog region, set `DD_SITE` to the appropriate value:

| Region | DD_SITE Value | Example URL |
|--------|---------------|-------------|
| US1 (default) | `datadoghq.com` | `https://app.datadoghq.com` |
| US3 | `us3.datadoghq.com` | `https://us3.datadoghq.com` |
| US5 | `us5.datadoghq.com` | `https://us5.datadoghq.com` |
| EU | `datadoghq.eu` | `https://app.datadoghq.eu` |
| AP1 | `ap1.datadoghq.com` | `https://ap1.datadoghq.com` |
| Government | `ddog-gov.com` | `https://app.ddog-gov.com` |

To identify your site, check the URL you use to access Datadog in your browser.

**Note for SSO Users:**
If your company uses SSO, you still use the same API and Application keys. SSO only affects UI login, not API authentication.

## Usage

### Running the Server

```bash
./datadog-mcp-server
```

The server communicates via JSON-RPC 2.0 over stdin/stdout.

### MCP Configuration

Add this to your MCP client configuration (e.g., Claude Desktop config):

```json
{
  "mcpServers": {
    "datadog": {
      "command": "/path/to/datadog-mcp-server",
      "env": {
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-application-key",
        "DD_SITE": "datadoghq.com"
      }
    }
  }
}
```

For EU region example:
```json
{
  "mcpServers": {
    "datadog": {
      "command": "/path/to/datadog-mcp-server",
      "env": {
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-application-key",
        "DD_SITE": "datadoghq.eu"
      }
    }
  }
}
```

For Claude Desktop on macOS, the config file is located at:
`~/Library/Application Support/Claude/claude_desktop_config.json`

### VSCode Configuration

#### Using with Cline Extension

If you're using the [Cline extension](https://github.com/cline/cline) for VSCode:

1. Install the Cline extension from the VSCode marketplace
2. Open VSCode settings (Cmd/Ctrl + ,)
3. Search for "Cline: MCP Settings"
4. Click "Edit in settings.json"
5. Add the MCP server configuration:

```json
{
  "cline.mcpServers": {
    "datadog": {
      "command": "/absolute/path/to/datadog-mcp-server",
      "env": {
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-application-key",
        "DD_SITE": "datadoghq.com"
      }
    }
  }
}
```

#### Using with Continue Extension

If you're using the [Continue extension](https://continue.dev/):

1. Install the Continue extension
2. Open the Continue configuration file (`~/.continue/config.json`)
3. Add the MCP server under `mcpServers`:

```json
{
  "mcpServers": [
    {
      "name": "datadog",
      "command": "/absolute/path/to/datadog-mcp-server",
      "env": {
        "DD_API_KEY": "your-api-key",
        "DD_APP_KEY": "your-application-key",
        "DD_SITE": "datadoghq.com"
      }
    }
  ]
}
```

#### Alternative: Use .env file

Instead of hardcoding credentials in the config, you can create a `.env` file in the project directory:

```bash
# .env
DD_API_KEY=your-api-key
DD_APP_KEY=your-application-key
DD_SITE=datadoghq.com
```

Then source it before launching VSCode:

```bash
# Load environment variables
export $(cat .env | xargs)

# Launch VSCode
code .
```

**Important:** Make sure `/absolute/path/to/datadog-mcp-server` points to the actual binary location, for example:
- `/Users/yourusername/go/github.com/kmesiab/go-dd-mcp/datadog-mcp-server` (macOS/Linux)
- `C:\path\to\datadog-mcp-server.exe` (Windows)

## Available Tools

### query_logs

Search and query Datadog logs with filters and time ranges.

**Parameters:**

- `query` (required): Search query using Datadog query syntax
  - Examples: `service:web status:error`, `env:production @user.id:12345`
- `from` (optional): Start time in RFC3339 format or relative time (e.g., `1h`, `30m`)
  - Default: 1 hour ago
- `to` (optional): End time in RFC3339 format or relative time
  - Default: now
- `limit` (optional): Maximum number of logs to return (max 1000)
  - Default: 50

**Example queries:**

```
Query all error logs in the last hour:
  query: "status:error"

Query logs from specific service in last 30 minutes:
  query: "service:api-gateway"
  from: "30m"

Query logs with custom time range:
  query: "env:production @http.status_code:500"
  from: "2026-01-20T10:00:00Z"
  to: "2026-01-20T12:00:00Z"
  limit: 100
```

## Datadog Query Syntax

The `query` parameter supports full Datadog log search syntax:

- **Status**: `status:error`, `status:warn`, `status:info`
- **Service**: `service:web-api`, `service:database`
- **Environment**: `env:production`, `env:staging`
- **Tags**: `version:1.2.3`, `region:us-east-1`
- **Attributes**: `@user.id:123`, `@http.status_code:404`
- **Text search**: `"error message"` (quoted for exact match)
- **Wildcards**: `service:web-*`, `@user.email:*@example.com`
- **Boolean operators**: `service:api AND status:error`, `status:error OR status:warn`
- **Exclusion**: `-status:info`, `NOT service:test`

## Troubleshooting

### Custom Enterprise Subdomains

If your company uses a custom Datadog subdomain (e.g., `yourcompany.datadoghq.com`), you may encounter validation errors. The Go API client has a [known limitation](https://github.com/DataDog/datadog-api-client-go/issues/2456) with custom enterprise domains.

**Workaround:**
If you need to use a custom subdomain, the code would need to be modified to use context variables instead of the `DD_SITE` parameter. Contact your Datadog administrator to determine if your organization uses a standard regional site (like `datadoghq.eu` or `us3.datadoghq.com`) instead of a custom subdomain.

Most enterprise SSO setups use standard regional sites, so this limitation typically doesn't affect API access.

### Connection Issues

If you're getting connection errors:

1. **Verify your site/region**: Check the URL you use to access Datadog:
   - `app.datadoghq.com` → `DD_SITE=datadoghq.com`
   - `app.datadoghq.eu` → `DD_SITE=datadoghq.eu`
   - `us3.datadoghq.com` → `DD_SITE=us3.datadoghq.com`

2. **Verify API keys**: Ensure your API and App keys are valid and have the necessary permissions:
   - Go to Organization Settings > API Keys
   - Go to Organization Settings > Application Keys

3. **Test manually**: Try the test commands in the "Testing the Server" section below.

## Development

### Project Structure

```
.
├── main.go           # MCP server implementation
├── go.mod            # Go module dependencies
├── go.sum            # Dependency checksums
└── README.md         # This file
```

### Testing the Server

You can test the server manually using stdin/stdout:

```bash
# Initialize
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./datadog-mcp-server

# List available tools
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./datadog-mcp-server

# Query logs
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"query_logs","arguments":{"query":"status:error","limit":10}}}' | ./datadog-mcp-server
```

## Architecture

This server:
1. Uses the official [Datadog Go API client](https://github.com/DataDog/datadog-api-client-go)
2. Implements the MCP protocol for tool-based interactions
3. Provides a simple bridge between LLMs and Datadog's Logs API

The implementation is straightforward:
- `main.go`: ~250 lines of code
- MCP protocol handling
- Single tool implementation (logs query)
- Authentication via environment variables

## Future Extensions

Potential additions (PRs welcome):
- Metrics querying
- Event search
- Monitor management
- Dashboard access
- Incident tracking
- APM traces
- Additional log analysis tools

## License

MIT

## Contributing

Contributions are welcome. Please ensure code is well-tested and documented.

## Resources

- [Datadog API Documentation](https://docs.datadoghq.com/api/latest/)
- [Datadog Go Client](https://github.com/DataDog/datadog-api-client-go)
- [MCP Protocol Specification](https://modelcontextprotocol.io/)
- [Datadog Log Search Syntax](https://docs.datadoghq.com/logs/explorer/search_syntax/)
