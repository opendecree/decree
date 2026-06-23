package configclient

import (
	"context"

	sdkretry "github.com/opendecree/decree/sdk/retry"
)

// RetryConfig configures automatic retry with exponential backoff.
// It is an alias for the shared [sdkretry.Config] type.
type RetryConfig = sdkretry.Config

// WithRetry enables automatic retry with exponential backoff for transient errors.
// By default, only errors wrapped in [RetryableError] by the transport are retried.
func WithRetry(cfg RetryConfig) Option {
	return func(o *options) {
		o.retry = cfg.WithDefaults()
		o.retryEnabled = true
	}
}

// retry executes fn with retries if retry is enabled. Otherwise calls fn once.
func retry[T any](ctx context.Context, c *Client, fn func(ctx context.Context) (T, error)) (T, error) {
	return sdkretry.Run(ctx, c.opts.retryEnabled, c.opts.retry, fn)
}
