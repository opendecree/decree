package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/sdk/configclient"
)

// normalizeStreamErr maps a streaming RPC error to nil when the error signals a
// clean stream end: context cancellation (Ctrl-C / SIGTERM) or io.EOF (graceful
// server shutdown). All other errors are returned unchanged.
func normalizeStreamErr(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
		return nil
	}
	return err
}

func typedValueDisplay(tv *configclient.TypedValue) string {
	if tv == nil {
		return "<null>"
	}
	return tv.String()
}

var watchCmd = &cobra.Command{
	Use:   "watch <tenant-id> [field-paths...]",
	Short: "Stream live config changes (like tail -f)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tenantID := args[0]
		fieldPaths := args[1:]

		conn, err := dialServer()
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		transport, err := newConfigTransport(conn)
		if err != nil {
			return err
		}

		sub, err := transport.Subscribe(cmd.Context(), &configclient.SubscribeRequest{
			TenantID:   tenantID,
			FieldPaths: fieldPaths,
		})
		if err != nil {
			return err
		}

		for {
			c, err := sub.Recv()
			if err != nil {
				return normalizeStreamErr(err)
			}
			ts := time.Now().Format("15:04:05")
			fmt.Printf("[%s] v%d %s: %q → %q (by %s)\n", ts, c.Version, c.FieldPath,
				typedValueDisplay(c.OldValue), typedValueDisplay(c.NewValue), c.ChangedBy)
		}
	},
}
