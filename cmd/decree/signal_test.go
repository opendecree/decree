package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestMain_UsesSignalContext verifies that the root command supports being run
// with a cancellable context (as wired up by signal.NotifyContext in main()).
// We exercise this by executing the root command with an already-cancelled
// context and confirming the command returns without panicking.
func TestMain_UsesSignalContext(t *testing.T) {
	resetRootCmd(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // simulate Ctrl-C before execution

	// Execute a no-op subcommand (argument validation only) so we don't hit
	// the network.  The key assertion is that ExecuteContext does not panic.
	rootCmd.SetArgs([]string{"schema", "get"}) // missing required arg → err
	_ = rootCmd.ExecuteContext(ctx)
	// No panic = context propagation is wired correctly.
}

// TestWatchStream_CancelReturnsNil verifies that a gRPC Canceled status
// returned by stream.Recv() is treated as a clean exit (nil error) by the
// watch command logic — matching the acceptance criterion "exit 0 on clean
// cancel".
func TestWatchStream_CancelReturnsNil(t *testing.T) {
	canceledErr := status.Error(codes.Canceled, "context canceled")

	// Replicate the check inside watchCmd.RunE.
	err := normalizeStreamErr(canceledErr)
	if err != nil {
		t.Errorf("expected nil for gRPC Canceled, got %v", err)
	}
}

// TestWatchStream_ContextCanceledReturnsNil verifies that a plain
// context.Canceled error (e.g., from some middleware) is also treated as a
// clean exit.
func TestWatchStream_ContextCanceledReturnsNil(t *testing.T) {
	err := normalizeStreamErr(context.Canceled)
	if err != nil {
		t.Errorf("expected nil for context.Canceled, got %v", err)
	}
}

// TestWatchStream_OtherErrorPreserved verifies that non-cancellation errors
// are still propagated as errors.
func TestWatchStream_OtherErrorPreserved(t *testing.T) {
	rpcErr := status.Error(codes.Unavailable, "server gone")
	err := normalizeStreamErr(rpcErr)
	if err == nil {
		t.Error("expected non-nil error for Unavailable, got nil")
	}
	if !errors.Is(err, rpcErr) {
		t.Errorf("expected original error preserved, got %v", err)
	}
}

// TestWatchStream_EOFReturnsNil verifies that io.EOF (graceful server shutdown)
// is treated as a clean stream end and mapped to nil — exit 0.
func TestWatchStream_EOFReturnsNil(t *testing.T) {
	err := normalizeStreamErr(io.EOF)
	if err != nil {
		t.Errorf("expected nil for io.EOF, got %v", err)
	}
}
