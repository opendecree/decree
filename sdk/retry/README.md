# retry

> **Alpha** — API subject to change.

Shared exponential-backoff retry engine used internally by `configclient` and `adminclient`. Most callers do not use this package directly — it is wired in automatically via the SDK client option `WithRetry`.

[![Go Reference](https://pkg.go.dev/badge/github.com/opendecree/decree/sdk/retry.svg)](https://pkg.go.dev/github.com/opendecree/decree/sdk/retry)

## Overview

The package provides two generic functions:

- `Run` — execute a function that returns `(T, error)` with retry
- `RunDo` — execute a void function `func(ctx) error` with retry

Both respect context cancellation between attempts and use exponential backoff with an optional full-jitter step.

## Config fields and defaults

```go
cfg := retry.Config{
    MaxAttempts:    3,             // default: 3
    InitialBackoff: 100 * time.Millisecond, // default: 100ms
    MaxBackoff:     5 * time.Second,        // default: 5s
    Jitter:         false,         // default: no jitter (opt-in)
    RetryableCheck: retry.IsRetryable, // default: checks for *RetryableError
}
cfg = cfg.WithDefaults() // fills in zero fields
```

`WithDefaults` returns a new `Config` with zero-valued fields replaced by the defaults above. Non-zero fields are left unchanged.

## Marking errors as retryable

Transport implementations signal that an error is transient by wrapping it in `*RetryableError`:

```go
return nil, &retry.RetryableError{Err: err}
```

`retry.IsRetryable` (the default `RetryableCheck`) uses `errors.As` to detect this wrapper.

## Minimal example

```go
import (
    "context"
    "fmt"
    "github.com/opendecree/decree/sdk/retry"
)

cfg := retry.Config{MaxAttempts: 3, Jitter: true}.WithDefaults()

result, err := retry.Run(ctx, true, cfg, func(ctx context.Context) (string, error) {
    return callRemoteService(ctx)
})
if err != nil {
    return fmt.Errorf("service unavailable: %w", err)
}
```

For void operations:

```go
err := retry.RunDo(ctx, true, cfg, func(ctx context.Context) error {
    return sendEvent(ctx)
})
```

## Note

Consumers typically do not import this package directly. Pass `configclient.WithRetry` or the equivalent `adminclient` option when constructing a client — the retry engine is wired in for you.
