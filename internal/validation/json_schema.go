package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/metric"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// jsonSchemaValidator validates JSON values against a JSON Schema document.
type jsonSchemaValidator struct {
	schema *jsonschema.Schema
}

// newJSONSchemaValidator compiles a JSON Schema document for validation.
// limits.MaxDepth bounds structural nesting before compile;
// limits.CompileTimeout caps the wall-clock duration of the compile call.
// timeoutCounter, if non-nil, is incremented when the deadline fires.
//
// Goroutine leak: jsonschema/v6 has no CompileContext, so a compile that
// exceeds the deadline will continue running until it finishes or the
// process exits. This is acceptable because:
//   - The pre-compile depth check (MaxDepth) rejects deeply-nested bombs
//     before the goroutine is started.
//   - The upstream document-size cap (schema.Limits.MaxDocBytes) bounds
//     the total input the compiler can process.
//
// Together these make the worst-case work finite. The timeout is a
// defence-in-depth backstop against unanticipated compiler pathologies.
// When it fires, the counter "validation.json_schema_compile_timeouts_total"
// is incremented so operators can alert on sustained activity.
func newJSONSchemaValidator(schemaDoc string, limits Limits, timeoutCounter metric.Int64Counter) (*jsonSchemaValidator, error) {
	if limits.MaxDepth > 0 {
		if err := scanJSONDepth(schemaDoc, limits.MaxDepth); err != nil {
			return nil, err
		}
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
			ch <- result{nil, fmt.Errorf("invalid json schema: %w", err)}
			return
		}
		if err := c.AddResource("schema.json", doc); err != nil {
			ch <- result{nil, fmt.Errorf("add json schema resource: %w", err)}
			return
		}
		schema, err := c.Compile("schema.json")
		if err != nil {
			ch <- result{nil, fmt.Errorf("compile json schema: %w", err)}
			return
		}
		ch <- result{&jsonSchemaValidator{schema: schema}, nil}
	}()

	if limits.CompileTimeout <= 0 {
		r := <-ch
		return r.v, r.err
	}
	timer := time.NewTimer(limits.CompileTimeout)
	defer timer.Stop()
	select {
	case r := <-ch:
		return r.v, r.err
	case <-timer.C:
		if timeoutCounter != nil {
			timeoutCounter.Add(context.Background(), 1)
		}
		return nil, fmt.Errorf("compile json schema: timeout after %s", limits.CompileTimeout)
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
