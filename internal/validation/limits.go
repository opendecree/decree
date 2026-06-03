package validation

import (
	"time"

	"go.opentelemetry.io/otel/metric"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
)

// Limits caps JSON-Schema compilation cost. Zero values mean "no limit"
// for that dimension. Use [DefaultLimits] for safe defaults.
//
// These guard against pathological JSON Schema documents (cyclic $ref,
// exponential allOf/anyOf, extreme nesting) that would otherwise hang or
// OOM the server during the first compile. Tracked in
// opendecree/decree#217 (security review finding 6).
type Limits struct {
	// CompileTimeout caps the wall-clock duration of a single
	// jsonschema.Compile call. Because jsonschema/v6 has no
	// CompileContext, the underlying goroutine may continue running
	// past the deadline; the bound on input depth and document size
	// (enforced upstream) keeps the worst-case work finite.
	CompileTimeout time.Duration

	// MaxDepth caps the structural nesting of the schema document
	// before compilation. A schema deeper than this is rejected
	// without invoking the compiler.
	MaxDepth int

	// MaxConcurrentCompiles caps the number of jsonschema.Compile calls
	// that may run simultaneously across the process. When the limit is
	// reached, new compile requests block until a slot is released or
	// CompileTimeout fires. This bounds goroutine growth when malicious
	// input repeatedly triggers the timeout path. 0 means no limit.
	MaxConcurrentCompiles int

	// RegexMaxLength caps the byte length of a regex pattern at SetField
	// validation time. Patterns longer than this are rejected without
	// compilation. 0 means no limit.
	RegexMaxLength int
}

// DefaultLimits returns conservative defaults: a 5-second compile
// timeout, a max nesting depth of 64, a concurrency cap of 32, and a
// 1 024-byte regex pattern cap. Tune via env vars at the call site (cmd/server).
func DefaultLimits() Limits {
	return Limits{
		CompileTimeout:        5 * time.Second,
		MaxDepth:              64,
		MaxConcurrentCompiles: 32,
		RegexMaxLength:        1024,
	}
}

// Option configures a [ValidatorFactory] or [FieldValidator]. See
// [WithLimits], [WithTimeoutCounter], [WithRegexErrorCounter], and
// [WithInFlightGauge].
type Option func(*options)

type options struct {
	limits            Limits
	timeoutCounter    metric.Int64Counter       // nil when metrics are disabled
	regexErrorCounter metric.Int64Counter       // nil when metrics are disabled
	inFlightGauge     metric.Int64UpDownCounter // nil when metrics are disabled
	celCapCounter     metric.Int64Counter       // nil when metrics are disabled
	celSoftErrCounter metric.Int64Counter       // nil when metrics are disabled
	compileSem        chan struct{}             // nil when MaxConcurrentCompiles == 0
	nullable          bool
	sensitive         bool
	constraints       *pb.FieldConstraints
}

// WithLimits sets the JSON-Schema compile limits. Defaults to
// [DefaultLimits] when unset.
func WithLimits(l Limits) Option {
	return func(o *options) {
		o.limits = l
		if l.MaxConcurrentCompiles > 0 {
			o.compileSem = make(chan struct{}, l.MaxConcurrentCompiles)
		} else {
			o.compileSem = nil
		}
	}
}

// WithTimeoutCounter sets the OTEL counter incremented when a JSON-Schema
// compile goroutine exceeds its deadline. The counter name should be
// "validation.json_schema_compile_timeouts_total". Pass nil to disable
// (no-op — equivalent to omitting the option).
func WithTimeoutCounter(c metric.Int64Counter) Option {
	return func(o *options) { o.timeoutCounter = c }
}

// WithRegexErrorCounter sets the OTEL counter incremented when a regex
// constraint pattern stored in the DB fails to compile. The counter name
// should be "validator_regex_compile_errors_total". Pass nil to disable.
func WithRegexErrorCounter(c metric.Int64Counter) Option {
	return func(o *options) { o.regexErrorCounter = c }
}

// WithInFlightGauge sets the OTEL up-down counter tracking the number of
// JSON-Schema compile goroutines currently in flight (including zombies
// that outlived their timeout). The metric name should be
// "validation.json_schema_compiles_in_flight". Pass nil to disable.
func WithInFlightGauge(g metric.Int64UpDownCounter) Option {
	return func(o *options) { o.inFlightGauge = g }
}

// WithCelCapCounter sets the OTEL counter incremented (with a tenant_id
// attribute) when the aggregate CEL evaluation cost cap is exceeded. The
// metric name should be
// "validation.cel_aggregate_cost_cap_exceeded_total". Pass nil to disable.
func WithCelCapCounter(c metric.Int64Counter) Option {
	return func(o *options) { o.celCapCounter = c }
}

// WithCelSoftErrCounter sets the OTEL counter incremented (with a tenant_id
// attribute) when a CEL runtime error is treated as a soft error in lenient
// mode. The metric name should be
// "validation.cel_soft_error_total". Pass nil to disable.
func WithCelSoftErrCounter(c metric.Int64Counter) Option {
	return func(o *options) { o.celSoftErrCounter = c }
}

// WithNullable marks the field as nullable. When set, a nil or empty
// [pb.TypedValue] is accepted. Defaults to false (not nullable).
func WithNullable() Option {
	return func(o *options) { o.nullable = true }
}

// WithSensitive marks the field as sensitive. When set, error messages omit
// the offending value to avoid leaking secrets in logs or responses.
// Defaults to false.
func WithSensitive() Option {
	return func(o *options) { o.sensitive = true }
}

// WithConstraints attaches field constraints (min/max, regex, enum, JSON
// Schema, etc.) to the validator. Defaults to nil (no constraints).
func WithConstraints(c *pb.FieldConstraints) Option {
	return func(o *options) { o.constraints = c }
}

func resolveOptions(opts []Option) options {
	defaults := DefaultLimits()
	o := options{
		limits:     defaults,
		compileSem: make(chan struct{}, defaults.MaxConcurrentCompiles),
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
