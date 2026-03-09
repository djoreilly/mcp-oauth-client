# Simple MCP Client with OAuth support

A simple CLI alternative to [MCP Inspector](https://github.com/modelcontextprotocol/inspector) for testing/development. Based on [oauth client example](https://github.com/modelcontextprotocol/go-sdk/blob/main/examples/auth/client/main.go).

## Pre-registered client
```
CLIENT_ID=mcp-test-client \
CLIENT_SECRET=UzErcSLIH04pnEXDUvegmzF4xwKSR6Xc \
GOFLAGS="-tags=mcp_go_client_oauth" go run main.go \
--server-url http://localhost:7777/mcp
```

## DCR
```
GOFLAGS="-tags=mcp_go_client_oauth" go run main.go \
--server-url http://localhost:7777/mcp
```
