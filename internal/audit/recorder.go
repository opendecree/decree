package audit

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.opentelemetry.io/otel/metric"
)

// Option configures a UsageRecorder.
type Option func(*recorderOptions)

type recorderOptions struct {
	flushInterval       time.Duration
	shutdownTimeout     time.Duration
	logger              *slog.Logger
	dbErrCounter        metric.Int64Counter
	flushTimeoutCounter metric.Int64Counter
}

// WithFlushInterval sets how often pending stats are flushed to the store.
// Zero or negative falls back to 30s.
func WithFlushInterval(d time.Duration) Option {
	return func(o *recorderOptions) { o.flushInterval = d }
}

// WithShutdownTimeout sets the deadline for the final flush performed when
// the recorder's context is cancelled. Zero or negative falls back to 5s.
func WithShutdownTimeout(d time.Duration) Option {
	return func(o *recorderOptions) { o.shutdownTimeout = d }
}

// WithLogger sets the recorder logger. Defaults to slog.Default() when unset.
func WithLogger(l *slog.Logger) Option {
	return func(o *recorderOptions) { o.logger = l }
}

// WithDBErrorCounter sets the OTel counter incremented for each DB write error
// during a usage-stats flush. Nil disables the metric.
func WithDBErrorCounter(c metric.Int64Counter) Option {
	return func(o *recorderOptions) { o.dbErrCounter = c }
}

// WithFlushTimeoutCounter sets the OTel counter incremented when the final
// shutdown flush exceeds its deadline. Nil disables the metric.
func WithFlushTimeoutCounter(c metric.Int64Counter) Option {
	return func(o *recorderOptions) { o.flushTimeoutCounter = c }
}

// usageKey identifies a unique (tenant, field) pair for batching.
type usageKey struct {
	TenantID  string
	FieldPath string
}

// usageBucket accumulates read counts for a single (tenant, field) pair.
type usageBucket struct {
	Count       int64
	LastReadBy  *string
	LastReadAt  time.Time
	PeriodStart time.Time
}

// UsageRecorder accumulates config read events in memory and flushes them
// to the audit store periodically as batched usage statistics.
// A nil *UsageRecorder is safe to call — all methods are no-ops.
type UsageRecorder struct {
	store               Store
	logger              *slog.Logger
	interval            time.Duration
	shutdownTimeout     time.Duration
	dbErrCounter        metric.Int64Counter // nil when metrics are disabled
	flushTimeoutCounter metric.Int64Counter // nil when metrics are disabled

	mu      sync.Mutex
	pending map[usageKey]*usageBucket

	done chan struct{}
}

// NewUsageRecorder creates a new recorder. Call Start to begin the background flush goroutine.
func NewUsageRecorder(store Store, opts ...Option) *UsageRecorder {
	o := recorderOptions{
		flushInterval:   30 * time.Second,
		shutdownTimeout: 5 * time.Second,
		logger:          slog.Default(),
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.flushInterval <= 0 {
		o.flushInterval = 30 * time.Second
	}
	if o.shutdownTimeout <= 0 {
		o.shutdownTimeout = 5 * time.Second
	}
	if o.logger == nil {
		o.logger = slog.Default()
	}
	return &UsageRecorder{
		store:               store,
		logger:              o.logger,
		interval:            o.flushInterval,
		shutdownTimeout:     o.shutdownTimeout,
		dbErrCounter:        o.dbErrCounter,
		flushTimeoutCounter: o.flushTimeoutCounter,
		pending:             make(map[usageKey]*usageBucket),
		done:                make(chan struct{}),
	}
}

// RecordRead records a single config field read. Non-blocking.
func (r *UsageRecorder) RecordRead(tenantID, fieldPath string, actor *string) {
	if r == nil {
		return
	}
	now := time.Now().UTC()
	key := usageKey{TenantID: tenantID, FieldPath: fieldPath}

	r.mu.Lock()
	b, ok := r.pending[key]
	if !ok {
		b = &usageBucket{PeriodStart: now.Truncate(time.Hour)}
		r.pending[key] = b
	}
	b.Count++
	b.LastReadBy = actor
	b.LastReadAt = now
	r.mu.Unlock()
}

// RecordReads records reads for multiple fields in one call. Non-blocking.
func (r *UsageRecorder) RecordReads(tenantID string, fieldPaths []string, actor *string) {
	if r == nil {
		return
	}
	now := time.Now().UTC()
	periodStart := now.Truncate(time.Hour)

	r.mu.Lock()
	for _, fp := range fieldPaths {
		key := usageKey{TenantID: tenantID, FieldPath: fp}
		b, ok := r.pending[key]
		if !ok {
			b = &usageBucket{PeriodStart: periodStart}
			r.pending[key] = b
		}
		b.Count++
		b.LastReadBy = actor
		b.LastReadAt = now
	}
	r.mu.Unlock()
}

// Start runs the background flush loop. Blocks until ctx is cancelled, then
// performs a final flush bounded by the shutdown timeout (default 5s) before
// returning. If the flush exceeds the deadline, Start logs a warning, increments
// the flush-timeout counter (if configured), and returns without blocking further.
// GracefulStop callers must account for up to shutdownTimeout of additional delay.
func (r *UsageRecorder) Start(ctx context.Context) {
	if r == nil {
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := r.Flush(ctx); err != nil {
				r.logger.WarnContext(ctx, "usage stats flush failed", "error", err)
			}
		case <-ctx.Done():
			flushCtx, cancel := context.WithTimeout(context.Background(), r.shutdownTimeout)
			defer cancel()
			if err := r.Flush(flushCtx); err != nil {
				if flushCtx.Err() != nil {
					r.logger.Warn("usage stats final flush timed out", "timeout", r.shutdownTimeout)
					if r.flushTimeoutCounter != nil {
						r.flushTimeoutCounter.Add(context.Background(), 1)
					}
				} else {
					r.logger.Warn("usage stats final flush failed", "error", err)
				}
			}
			close(r.done)
			return
		}
	}
}

// Stop waits for the background goroutine to finish its final flush.
// The caller must cancel the context passed to Start before calling Stop.
func (r *UsageRecorder) Stop() {
	if r == nil {
		return
	}
	<-r.done
}

// Flush swaps the pending buffer and writes all accumulated stats to the store.
// Exported for testing.
func (r *UsageRecorder) Flush(ctx context.Context) error {
	if r == nil {
		return nil
	}

	// Swap buffer under lock — reads are never blocked by slow DB writes.
	r.mu.Lock()
	snapshot := r.pending
	r.pending = make(map[usageKey]*usageBucket, len(snapshot))
	r.mu.Unlock()

	if len(snapshot) == 0 {
		return nil
	}

	var firstErr error
	for key, bucket := range snapshot {
		err := r.store.UpsertUsageStats(ctx, UpsertUsageStatsParams{
			TenantID:    key.TenantID,
			FieldPath:   key.FieldPath,
			PeriodStart: bucket.PeriodStart,
			ReadCount:   bucket.Count,
			LastReadBy:  bucket.LastReadBy,
			LastReadAt:  bucket.LastReadAt,
		})
		if err != nil {
			r.logger.WarnContext(ctx, "failed to upsert usage stats",
				"tenant_id", key.TenantID,
				"field_path", key.FieldPath,
				"error", err,
			)
			if r.dbErrCounter != nil {
				r.dbErrCounter.Add(ctx, 1)
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
