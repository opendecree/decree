// Live Config demonstrates an HTTP server whose behavior is driven by
// configwatcher — rate limits, timeouts, and feature toggles update
// in real time without restarting the server.
//
// Run:
//
//	go run .
//
// Then visit http://localhost:8081/config to see live values, or
// change config on the server and refresh to see updates.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/opendecree/decree/sdk/configwatcher"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := grpc.NewClient(serverAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()

	tenantID := mustTenantID()

	// Register fields that drive server behavior.
	w := configwatcher.New(conn, tenantID,
		configwatcher.WithSubject("live-config-example"),
	)
	rateLimit := w.Int("server.rate_limit", 100)
	timeout := w.Duration("server.timeout", 30*time.Second)
	maxConns := w.Int("server.max_connections", 50)
	debug := w.Bool("app.debug", false)

	if err := w.Start(ctx); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}
	defer w.Close()

	// HTTP handler reads live config on every request — always fresh.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /config", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"rate_limit":      rateLimit.Get(),
			"timeout":         timeout.Get().String(),
			"max_connections": maxConns.Get(),
			"debug":           debug.Get(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	srv := &http.Server{Addr: ":8081", Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	fmt.Println("Listening on http://localhost:8081/config")
	fmt.Println("Config updates are live — change values and refresh.")
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func serverAddr() string {
	if v := os.Getenv("DECREE_ADDR"); v != "" {
		return v
	}
	return "localhost:9090"
}

func mustTenantID() string {
	if v := os.Getenv("TENANT_ID"); v != "" {
		return v
	}
	data, err := os.ReadFile("../.tenant-id")
	if err == nil {
		return strings.TrimSpace(string(data))
	}
	log.Fatal("Set TENANT_ID env var or run 'make setup' from the examples directory")
	return ""
}
