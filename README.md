# Simple MCP Client with OAuth support

A simple CLI alternative to [MCP Inspector](https://github.com/modelcontextprotocol/inspector) for testing/development MCP servers with OAuth. Based on [oauth client example](https://github.com/modelcontextprotocol/go-sdk/blob/main/examples/auth/client/main.go) and [this](https://github.com/jhrozek/go-sdk/blob/client_example/examples/client/simple-auth/main.go).

## Pre-registered client
```bash
CLIENT_ID=mcp-test-client \
CLIENT_SECRET=UzErcSLIH04pnEXDUvegmzF4xwKSR6Xc \
go run main.go \
--server-url http://localhost:7777/mcp
Connecting to MCP server...
Please open the following URL in your browser: http://localhost:8090/realms/mcp-realm/protocol/openid-connect/auth?client_id=mcp-test-client&code_challenge=Q-_Cko7VvCgFkxhKuJA792KIW6QGqQ_JghUfcJYlu3I&code_challenge_method=S256&redirect_uri
=http%3A%2F%2Flocalhost%3A3142&resource=http%3A%2F%2Flocalhost%3A7777%2Fmcp&response_type=code&scope=mcp%3Atools%3Aread+mcp%3Atools%3Awrite&state=5XPBPSSO3CJFK7ZNEJGMEXGH47
Connected to MCP server

Interactive MCP Client
Commands:
  list - List available tools
  call <tool_name> [args] - Call a tool
  quit - Exit the client

mcp> list

Available tools:
1. echo
   Description
mcp> call echo {"input": "banana"}

Tool 'echo' result:
banana
mcp> quit
```

## DCR
Dynamic Client Registration will be tried if `CLIENT_ID` is not set.
```bash
go run main.go \
--server-url http://localhost:7777/mcp
...
```
