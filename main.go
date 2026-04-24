// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

var (
	// URL of the MCP server.
	serverURL = flag.String("server-url", "", "URL of the MCP server.")
	// Port for the local HTTP server that will receive the authorization code.
	callbackPort = flag.Int("callback-port", 3142, "Port for the local HTTP server that will receive the authorization code.")
	caCertFile   = flag.String("cacert", "", "cacert to verify the TLS server")
	debugLogging = flag.Bool("debug", false, "enable debug logging jsonrpc for calls")
)

type debugLogger struct{}

func (*debugLogger) Write(p []byte) (int, error) {
	slog.Debug(string(p))
	return len(p), nil
}

type codeReceiver struct {
	authChan chan *auth.AuthorizationResult
	errChan  chan error
	listener net.Listener
	server   *http.Server
}

func (r *codeReceiver) serveRedirectHandler(listener net.Listener) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.authChan <- &auth.AuthorizationResult{
			Code:  req.URL.Query().Get("code"),
			State: req.URL.Query().Get("state"),
		}
		fmt.Fprint(w, "Authentication successful. You can close this window.")
	})

	r.server = &http.Server{
		Addr:    fmt.Sprintf("localhost:%d", *callbackPort),
		Handler: mux,
	}
	if err := r.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		r.errChan <- err
	}
}

func (r *codeReceiver) getAuthorizationCode(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	fmt.Printf("Please open the following URL in your browser: %s\n", args.URL)
	select {
	case authRes := <-r.authChan:
		return authRes, nil
	case err := <-r.errChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *codeReceiver) close() {
	if r.server != nil {
		r.server.Close()
	}
}

// AuthClient is a simple MCP client.
type AuthClient struct {
	transport mcp.Transport
	session   *mcp.ClientSession
}

// NewAuthClient creates a new client with the given transport.
func NewAuthClient(transport mcp.Transport) *AuthClient {
	return &AuthClient{
		transport: transport,
	}
}

// Connect connects to the MCP server.
func (c *AuthClient) Connect(ctx context.Context) error {
	fmt.Println("Connecting to MCP server...")

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "simple-auth-client",
		Version: "v1.0.0",
	}, nil)

	// Connect to server
	session, err := client.Connect(ctx, c.transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.session = session
	fmt.Println("Connected to MCP server")

	return nil
}

// ListTools lists available tools from the server.
func (c *AuthClient) ListTools(ctx context.Context) error {
	if c.session == nil {
		return fmt.Errorf("not connected to server")
	}

	fmt.Println("\nAvailable tools:")
	count := 0
	for tool, err := range c.session.Tools(ctx, nil) {
		if err != nil {
			return fmt.Errorf("failed to list tools: %w", err)
		}
		count++
		fmt.Printf("%d. %s", count, tool.Name)
		if tool.Description != "" {
			fmt.Printf("\n   Description: %s", tool.Description)
		}
		fmt.Println()
	}

	if count == 0 {
		fmt.Println("No tools available")
	}

	return nil
}

// CallTool calls a specific tool.
func (c *AuthClient) CallTool(ctx context.Context, toolName string, arguments map[string]any) error {
	if c.session == nil {
		return fmt.Errorf("not connected to server")
	}

	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return fmt.Errorf("failed to call tool '%s': %w", toolName, err)
	}

	fmt.Printf("\nTool '%s' result:\n", toolName)
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			fmt.Println(textContent.Text)
		} else {
			fmt.Printf("%+v\n", content)
		}
	}

	return nil
}

// InteractiveLoop runs the interactive command loop.
func (c *AuthClient) InteractiveLoop(ctx context.Context) error {
	fmt.Println("\nInteractive MCP Client")
	fmt.Println("Commands:")
	fmt.Println("  list - List available tools")
	fmt.Println("  call <tool_name> [args] - Call a tool")
	fmt.Println("  quit - Exit the client")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("mcp> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if line == "quit" {
			fmt.Println("\nGoodbye!")
			break
		}

		if line == "list" {
			if err := c.ListTools(ctx); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			continue
		}

		if strings.HasPrefix(line, "call ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) < 2 {
				fmt.Println("Please specify a tool name")
				continue
			}

			toolName := parts[1]
			var arguments map[string]any

			if len(parts) > 2 {
				if err := json.Unmarshal([]byte(parts[2]), &arguments); err != nil {
					fmt.Printf("Invalid arguments format (expected JSON): %v\n", err)
					continue
				}
			}

			if err := c.CallTool(ctx, toolName, arguments); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			continue
		}

		fmt.Println("Unknown command. Try 'list', 'call <tool_name>', or 'quit'")
	}

	return nil
}

func main() {
	flag.Parse()
	if *debugLogging {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	receiver := &codeReceiver{
		authChan: make(chan *auth.AuthorizationResult),
		errChan:  make(chan error),
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *callbackPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	go receiver.serveRedirectHandler(listener)
	defer receiver.close()

	authCodeHandlerCfg := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              fmt.Sprintf("http://localhost:%d", *callbackPort),
		AuthorizationCodeFetcher: receiver.getAuthorizationCode,
	}
	clientID := os.Getenv("CLIENT_ID")
	clientSecret := os.Getenv("CLIENT_SECRET")
	if clientID != "" {
		if clientSecret == "" {
			// the go-sdk requires CLIENT_SECRET even if not using client authentication
			log.Print("warning: CLIENT_SECRET needs to be set with CLIENT_ID.")
			log.Print("If client authentication is disabled, set it CLIENT_SECRET to anything.")
		}
		authCodeHandlerCfg.PreregisteredClient = &oauthex.ClientCredentials{
			ClientID: clientID,
			ClientSecretAuth: &oauthex.ClientSecretAuth{
				ClientSecret: clientSecret,
			},
		}
	} else {
		authCodeHandlerCfg.DynamicClientRegistrationConfig = &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:   "Dynamically registered MCP client",
				RedirectURIs: []string{fmt.Sprintf("http://localhost:%d", *callbackPort)},
			},
		}
	}
	authHandler, err := auth.NewAuthorizationCodeHandler(authCodeHandlerCfg)
	if err != nil {
		log.Fatalf("failed to create auth handler: %v", err)
	}

	httpClient := http.DefaultClient
	if *caCertFile != "" {
		caCert, err := os.ReadFile(*caCertFile)
		if err != nil {
			log.Fatalf("could not read cacert file: %s: %v", *caCertFile, err)
		}
		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caCert) {
			log.Fatal("append certs to rootCAs failed")
		}
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: rootCAs,
				},
			},
		}
	}

	transport := &mcp.StreamableClientTransport{
		HTTPClient:   httpClient,
		Endpoint:     *serverURL,
		OAuthHandler: authHandler,
	}

	loggingTransport := &mcp.LoggingTransport{
		Transport: transport,
		Writer:    &debugLogger{},
	}

	// Create and connect client
	ctx := context.Background()
	client := NewAuthClient(loggingTransport)
	if err := client.Connect(ctx); err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer client.session.Close()

	// Run interactive loop
	if err := client.InteractiveLoop(ctx); err != nil {
		log.Fatalf("Interactive loop failed: %v", err)
	}
}
