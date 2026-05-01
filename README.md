# New Relic MCP Server (Go version)

A [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) server for interacting with New Relic observability data, rewritten in Go.

This server provides tools to Claude (and other MCP-compatible clients) to run NRQL queries, assess entity health, search logs, read alerts, and more.

## Prerequisites

- Go 1.25.5 or later
- A [New Relic User API key](https://one.newrelic.com/api-keys)

## Installation

Install the server using `go install`:

```bash
go install github.com/ikozinov/new-relic-mcp@latest
```

The binary will be installed to your `$GOPATH/bin` directory (typically `~/go/bin/new-relic-mcp`).

2. Add the server to your client configuration:

### Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "newrelic-mcp": {
      "command": "/Users/YOUR_USER/go/bin/new-relic-mcp",
      "env": {
        "NEW_RELIC_API_KEY": "NRAK-your-key-here",
        "NEW_RELIC_ACCOUNT_ID": "12345",
        "NEW_RELIC_REGION": "US"
      }
    }
  }
}
```

### Gemini (mcp.json)

For Gemini or other compatible clients that use `mcp.json` or standard config files:

```json
{
  "mcpServers": {
    "newrelic": {
      "command": "/Users/YOUR_USER/go/bin/new-relic-mcp",
      "args": [],
      "env": {
        "NEW_RELIC_API_KEY": "NRAK-your-key-here",
        "NEW_RELIC_ACCOUNT_ID": "12345",
        "NEW_RELIC_REGION": "US"
      }
    }
  }
}
```

## Running the Server Standalone

You can run the server directly if you want to test it. Note that it communicates using standard I/O (stdio) to interact with MCP clients.

```bash
NEW_RELIC_API_KEY=NRAK-... NEW_RELIC_ACCOUNT_ID=12345 ./newrelic-mcp
```

Alternatively, you can provide these variables in a `.env` file in the root of the project:

```
NEW_RELIC_API_KEY=NRAK-...        # New Relic User API key
NEW_RELIC_ACCOUNT_ID=12345        # Your account ID
NEW_RELIC_REGION=US               # US or EU
```

## Features

This Go version exposes basic text-based core MCP tools without requiring a complex web environment:

* **nrql-query**: Execute a custom NRQL query.
* **entity-health**: Get health statuses for APM and BROWSER entities.
* **alert-incidents**: Get currently active alert issues.
* **log-search**: Query logs across services.
* **error-inbox**: View grouped error fingerprints.
* **describe-event**: Look up the schema for any New Relic event type.
* **dashboard**: List New Relic dashboards or inspect their configuration structure.

### Fuzzy Resolution

For tools requiring `appName` arguments, this MCP server includes fuzzy entity resolution to guess the right application name even if slightly misspelled.
