package audit

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Option configures a UsageRecorder.
type Option func(*recorderOptions)

type recorderOptions struct {
	flushInterval time.Duration
	logger        *slog.Logger
}

// WithFlushInterval sets how often pending stats are flushed to the store.
// Zero or negative falls back to 30s.
func WithFlushInterval(d time.Duration) Option {
	return func(o *recorderOptions) { o.flushInterval = d }
}

// WithLogger sets the recorder logger. Defaults to slog.Default() when unset.
func WithLogger(l *slog.Logger) Option {
	return func(o *recorderOptions) { o.logger = l }
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
	store    Store
	logger   *slog.Logger
	interval time.Duration

	mu      sync.Mutex
	pending map[usageKey]*usageBucket

	done chan struct{}
}

// NewUsageRecorder creates a new recorder. Call Start to begin the background flush goroutine.
func NewUsageRecorder(store Store, opts ...Option) *UsageRecorder {
	o := recorderOptions{
		flushInterval: 30 * time.Second,
		logger:        slog.Default(),
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.flushInterval <= 0 {
		o.flushInterval = 30 * time.Second
	}
	if o.logger == nil {
		o.logger = slog.Default()
	}
	return &UsageRecorder{
		store:    store,
		logger:   o.logger,
		interval: o.flushInterval,
		pending:  make(map[usageKey]*usageBucket),
		done:     make(chan struct{}),
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

// Start runs the background flush loop. Blocks until ctx is cancelled,
// then performs a final flush before returning.
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
			// Final flush with a fresh context so it isn't cancelled.
			if err := r.Flush(context.Background()); err != nil {
				r.logger.Warn("usage stats final flush failed", "error", err)
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
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
