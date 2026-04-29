package validation

import "time"

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
}

// DefaultLimits returns conservative defaults: a 5-second compile
// timeout and a max nesting depth of 64. Tune via env vars at the call
// site (cmd/server).
func DefaultLimits() Limits {
	return Limits{
		CompileTimeout: 5 * time.Second,
		MaxDepth:       64,
	}
}

// Option configures a [ValidatorFactory] or [FieldValidator]. See
// [WithLimits].
type Option func(*options)

type options struct {
	limits Limits
}

// WithLimits sets the JSON-Schema compile limits. Defaults to
// [DefaultLimits] when unset.
func WithLimits(l Limits) Option {
	return func(o *options) { o.limits = l }
}

func resolveOptions(opts []Option) options {
	o := options{limits: DefaultLimits()}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}
