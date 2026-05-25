package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// jsonSchemaValidator validates JSON values against a JSON Schema document.
type jsonSchemaValidator struct {
	schema *jsonschema.Schema
}

// newJSONSchemaValidator compiles a JSON Schema document for validation.
// opts.limits.MaxDepth bounds structural nesting before compile;
// opts.limits.CompileTimeout caps the wall-clock duration of the compile call.
// opts.limits.MaxConcurrentCompiles caps simultaneous goroutines — new requests
// block until a slot is free or the timeout fires, bounding goroutine growth
// when malicious input repeatedly triggers the timeout path.
//
// Goroutine leak: jsonschema/v6 has no CompileContext, so a compile that
// exceeds the deadline will continue running until it finishes or the
// process exits. MaxConcurrentCompiles bounds the number of such zombies at
// any instant; the depth and upstream document-size checks (schema.Limits.MaxDocBytes)
// bound the work each zombie can perform.
//
// When the deadline fires, opts.timeoutCounter is incremented.
// opts.inFlightGauge tracks goroutines currently executing the compile
// (including zombies), incremented before spawn and decremented on finish.
func newJSONSchemaValidator(schemaDoc string, opts options) (*jsonSchemaValidator, error) {
	if opts.limits.MaxDepth > 0 {
		if err := scanJSONDepth(schemaDoc, opts.limits.MaxDepth); err != nil {
			return nil, err
		}
	}

	var timer *time.Timer
	var timerC <-chan time.Time
	if opts.limits.CompileTimeout > 0 {
		timer = time.NewTimer(opts.limits.CompileTimeout)
		defer timer.Stop()
		timerC = timer.C
	}

	// Acquire semaphore slot before spawning the goroutine.
	// If the semaphore is full (MaxConcurrentCompiles zombies already running),
	// block here until a slot is released or the deadline fires.
	if opts.compileSem != nil {
		select {
		case opts.compileSem <- struct{}{}:
		case <-timerC:
			if opts.timeoutCounter != nil {
				opts.timeoutCounter.Add(context.Background(), 1)
			}
			return nil, fmt.Errorf("compile json schema: timeout after %s", opts.limits.CompileTimeout)
		}
	}

	if opts.inFlightGauge != nil {
		opts.inFlightGauge.Add(context.Background(), 1)
	}

	type result struct {
		v   *jsonSchemaValidator
		err error
	}
	ch := make(chan result, 1)
	go func() {
		c := jsonschema.NewCompiler()
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(schemaDoc))
		if err != nil {
			if opts.inFlightGauge != nil {
				opts.inFlightGauge.Add(context.Background(), -1)
			}
			if opts.compileSem != nil {
				<-opts.compileSem
			}
			ch <- result{nil, fmt.Errorf("invalid json schema: %w", err)}
			return
		}
		if err := c.AddResource("schema.json", doc); err != nil {
			if opts.inFlightGauge != nil {
				opts.inFlightGauge.Add(context.Background(), -1)
			}
			if opts.compileSem != nil {
				<-opts.compileSem
			}
			ch <- result{nil, fmt.Errorf("add json schema resource: %w", err)}
			return
		}
		schema, err := c.Compile("schema.json")
		if opts.inFlightGauge != nil {
			opts.inFlightGauge.Add(context.Background(), -1)
		}
		if opts.compileSem != nil {
			<-opts.compileSem
		}
		if err != nil {
			ch <- result{nil, fmt.Errorf("compile json schema: %w", err)}
			return
		}
		ch <- result{&jsonSchemaValidator{schema: schema}, nil}
	}()

	if timerC == nil {
		r := <-ch
		return r.v, r.err
	}
	select {
	case r := <-ch:
		return r.v, r.err
	case <-timerC:
		if opts.timeoutCounter != nil {
			opts.timeoutCounter.Add(context.Background(), 1)
		}
		return nil, fmt.Errorf("compile json schema: timeout after %s", opts.limits.CompileTimeout)
	}
}

// validate checks a JSON string against the compiled schema.
func (v *jsonSchemaValidator) validate(jsonStr string) error {
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(jsonStr))
	if err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := v.schema.Validate(inst); err != nil {
		return fmt.Errorf("json schema validation failed: %w", err)
	}
	return nil
}

// scanJSONDepth walks the parsed JSON document and returns an error if
// nesting exceeds maxDepth. Non-JSON input is ignored and left for the
// compiler to report — this scan exists to short-circuit obvious bombs,
// not to validate syntax.
func scanJSONDepth(doc string, maxDepth int) error {
	var v any
	if jsonErr := json.Unmarshal([]byte(doc), &v); jsonErr != nil {
		return nil //nolint:nilerr // non-JSON input is intentionally left for the compiler to report
	}
	return checkDepth(v, 0, maxDepth)
}

func checkDepth(v any, depth, max int) error {
	if depth > max {
		return fmt.Errorf("compile json schema: nesting depth exceeds limit of %d", max)
	}
	switch t := v.(type) {
	case map[string]any:
		for _, child := range t {
			if err := checkDepth(child, depth+1, max); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range t {
			if err := checkDepth(child, depth+1, max); err != nil {
				return err
			}
		}
	}
	return nil
}
