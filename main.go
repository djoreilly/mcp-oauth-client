// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//go:build mcp_go_client_oauth

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

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
)

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

func main() {
	flag.Parse()
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
		authCodeHandlerCfg.PreregisteredClientConfig = &auth.PreregisteredClientConfig{
			ClientSecretAuthConfig: &auth.ClientSecretAuthConfig{
				ClientID:     clientID,
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

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, nil)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("client.Connect(): %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("session.ListTools(): %v", err)
	}
	log.Println("Tools:")
	for _, tool := range tools.Tools {
		log.Printf("- %q", tool.Name)
	}
}
