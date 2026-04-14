//go:build example

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/configwatcher"
)

func TestExample(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(serverAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close()

	tenantID := os.Getenv("TENANT_ID")
	if tenantID == "" {
		data, err := os.ReadFile("../.tenant-id")
		if err != nil {
			t.Skip("no tenant ID available")
		}
		tenantID = strings.TrimSpace(string(data))
	}

	w := configwatcher.New(conn, tenantID,
		configwatcher.WithSubject("live-config-test"),
	)
	rateLimit := w.Int("server.rate_limit", 100)
	timeout := w.Duration("server.timeout", 30*time.Second)
	maxConns := w.Int("server.max_connections", 50)
	debug := w.Bool("app.debug", false)

	if err := w.Start(ctx); err != nil {
		t.Fatalf("start watcher: %v", err)
	}
	defer w.Close()

	// Start HTTP server on a random port.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /config", func(wr http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"rate_limit":      rateLimit.Get(),
			"timeout":         timeout.Get().String(),
			"max_connections": maxConns.Get(),
			"debug":           debug.Get(),
		}
		wr.Header().Set("Content-Type", "application/json")
		json.NewEncoder(wr).Encode(resp)
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := &http.Server{Handler: mux}
	go srv.Serve(ln)
	defer srv.Shutdown(ctx)

	// Hit the endpoint and verify JSON response.
	resp, err := http.Get(fmt.Sprintf("http://%s/config", ln.Addr()))
	if err != nil {
		t.Fatalf("GET /config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	if body["rate_limit"] == nil {
		t.Error("expected rate_limit in response")
	}
	if body["timeout"] == nil {
		t.Error("expected timeout in response")
	}

	fmt.Printf("live-config: GET /config returned %v\n", body)
}
