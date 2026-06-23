package adminclient

import (
	"context"

	sdkretry "github.com/opendecree/decree/sdk/retry"
)

// RetryConfig configures automatic retry with exponential backoff.
// It is an alias for the shared [sdkretry.Config] type.
type RetryConfig = sdkretry.Config

// retry executes fn with retries if retry is enabled. Otherwise calls fn once.
// Only idempotent operations (reads: List*, Get*) should be wrapped with retry.
func retry[T any](ctx context.Context, c *Client, fn func(ctx context.Context) (T, error)) (T, error) {
	return sdkretry.Run(ctx, c.opts.retryEnabled, c.opts.retry, fn)
}
