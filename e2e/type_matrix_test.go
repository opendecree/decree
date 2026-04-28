//go:build e2e

package e2e

// Matrix 3 — Field type × nullable × round-trip operation.
//
// Iterates the 8 supported field types × {nullable, non-nullable} × 5
// operations, asserting per-cell behavior:
//
//   - set_value:   writing a typed sample value succeeds and round-trips
//   - set_null:    SetNull succeeds for nullable, fails for non-nullable
//   - get:         after seeding, the typed getter returns the seeded value
//   - export_yaml: ExportConfig output contains the field path and value
//   - import_yaml: ImportConfig (merge) of just this field succeeds
//
// One schema is created with all 16 fields. Cells operate on their own
// (type, nullable) field path so they don't clobber each other.

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/configclient"
	"github.com/opendecree/decree/sdk/grpctransport"
)

// sampleSeq advances each time a varying sample is generated. Importing
// the same value the field already holds is rejected with AlreadyExists
// ("no changes to apply"), so deterministic samples (e.g. always 42) make
// import_yaml a no-op. Counter-based variance keeps every cell producing
// a distinct value on each invocation.
var sampleSeq int64

// typeCase describes one field type for matrix 3.
type typeCase struct {
	name      string                                       // "string", "integer", ...
	fieldType string                                       // "FIELD_TYPE_STRING"
	sample    func() *configclient.TypedValue              // a fresh sample value
	yamlValue func(sample *configclient.TypedValue) string // YAML literal for ImportConfig
	verifyEq  func(t *testing.T, want, got *configclient.TypedValue)
}

func typeCases() []typeCase {
	return []typeCase{
		{
			name: "string", fieldType: "FIELD_TYPE_STRING",
			sample:    func() *configclient.TypedValue { return configclient.StringVal("hello-" + randSuffix()) },
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%q", tv.StringValue()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.Equal(t, want.StringValue(), got.StringValue())
			},
		},
		{
			name: "integer", fieldType: "FIELD_TYPE_INT",
			sample: func() *configclient.TypedValue {
				return configclient.IntVal(atomic.AddInt64(&sampleSeq, 1))
			},
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%d", tv.IntValue()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.Equal(t, want.IntValue(), got.IntValue())
			},
		},
		{
			name: "number", fieldType: "FIELD_TYPE_NUMBER",
			sample: func() *configclient.TypedValue {
				return configclient.FloatVal(float64(atomic.AddInt64(&sampleSeq, 1)) / 100)
			},
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%g", tv.FloatValue()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.InEpsilon(t, want.FloatValue(), got.FloatValue(), 1e-9)
			},
		},
		{
			name: "bool", fieldType: "FIELD_TYPE_BOOL",
			sample: func() *configclient.TypedValue {
				return configclient.BoolVal(atomic.AddInt64(&sampleSeq, 1)%2 == 0)
			},
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%t", tv.BoolValue()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.Equal(t, want.BoolValue(), got.BoolValue())
			},
		},
		{
			name: "time", fieldType: "FIELD_TYPE_TIME",
			sample: func() *configclient.TypedValue {
				return configclient.TimeVal(time.Unix(atomic.AddInt64(&sampleSeq, 1), 0).UTC())
			},
			yamlValue: func(tv *configclient.TypedValue) string {
				return fmt.Sprintf("%q", tv.TimeValue().Format(time.RFC3339Nano))
			},
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.True(t, want.TimeValue().Equal(got.TimeValue()),
					"time mismatch: want=%s got=%s", want.TimeValue(), got.TimeValue())
			},
		},
		{
			name: "duration", fieldType: "FIELD_TYPE_DURATION",
			sample: func() *configclient.TypedValue {
				return configclient.DurationVal(time.Duration(atomic.AddInt64(&sampleSeq, 1)) * time.Second)
			},
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%q", tv.DurationValue().String()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.Equal(t, want.DurationValue(), got.DurationValue())
			},
		},
		{
			name: "url", fieldType: "FIELD_TYPE_URL",
			sample:    func() *configclient.TypedValue { return configclient.URLVal("https://example.com/" + randSuffix()) },
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%q", tv.URLValue()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.Equal(t, want.URLValue(), got.URLValue())
			},
		},
		{
			name: "json", fieldType: "FIELD_TYPE_JSON",
			sample: func() *configclient.TypedValue {
				return configclient.JSONVal(fmt.Sprintf(`{"k":"v-%d"}`, atomic.AddInt64(&sampleSeq, 1)))
			},
			yamlValue: func(tv *configclient.TypedValue) string { return fmt.Sprintf("%q", tv.JSONValue()) },
			verifyEq: func(t *testing.T, want, got *configclient.TypedValue) {
				assert.JSONEq(t, want.JSONValue(), got.JSONValue())
			},
		},
	}
}

// fieldPath builds the deterministic path for a (type, nullable) cell.
func fieldPath(tc typeCase, nullable bool) string {
	suffix := "required"
	if nullable {
		suffix = "nullable"
	}
	return fmt.Sprintf("f.%s_%s", tc.name, suffix)
}

// bootstrapTypeMatrix creates a schema with all 16 (type, nullable) fields,
// publishes it, creates a tenant, and seeds every field with a sample so
// `get`, `export_yaml`, and (the read step inside) `import_yaml` have
// values to read on the first pass.
func bootstrapTypeMatrix(t *testing.T) (*matrixFixture, map[string]*configclient.TypedValue) {
	t.Helper()
	conn := dial(t)
	admin := newAdminClient(conn)
	cfg := newConfigClient(conn)
	ctx := context.Background()

	cases := typeCases()
	fields := make([]adminclient.Field, 0, len(cases)*2)
	for _, tc := range cases {
		for _, nullable := range []bool{false, true} {
			fields = append(fields, adminclient.Field{
				Path:     fieldPath(tc, nullable),
				Type:     tc.fieldType,
				Nullable: nullable,
			})
		}
	}

	schemaName := "type-matrix-" + randSuffix()
	s, err := admin.CreateSchema(ctx, schemaName, fields, "")
	require.NoError(t, err)
	_, err = admin.PublishSchema(ctx, s.ID, 1)
	require.NoError(t, err)

	tenantName := "type-matrix-tenant-" + randSuffix()
	tenant, err := admin.CreateTenant(ctx, tenantName, s.ID, 1)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = admin.DeleteTenant(ctx, tenant.ID)
		_ = admin.DeleteSchema(ctx, s.ID)
	})

	// Seed each field with a sample. Keep the samples so cells can compare
	// round-tripped values without re-deriving them.
	seeds := make(map[string]*configclient.TypedValue, len(cases)*2)
	for _, tc := range cases {
		for _, nullable := range []bool{false, true} {
			path := fieldPath(tc, nullable)
			sample := tc.sample()
			require.NoError(t, cfg.SetTyped(ctx, tenant.ID, path, sample),
				"seeding %s", path)
			seeds[path] = sample
		}
	}

	return &matrixFixture{schemaID: s.ID, tenantID: tenant.ID}, seeds
}

func TestTypeMatrix(t *testing.T) {
	conn := dial(t)
	cfg := newConfigClient(conn)
	admin := newAdminClient(conn)
	cfgTransport := newConfigTransportSuperadmin(conn)
	fx, seeds := bootstrapTypeMatrix(t)

	for _, tc := range typeCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			for _, nullable := range []bool{false, true} {
				nullable := nullable
				label := "required"
				if nullable {
					label = "nullable"
				}
				t.Run(label, func(t *testing.T) {
					path := fieldPath(tc, nullable)

					t.Run("set_value", func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						sample := tc.sample()
						require.NoError(t, cfg.SetTyped(ctx, fx.tenantID, path, sample))
						seeds[path] = sample // refresh seed for downstream ops
					})

					t.Run("set_null", func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						err := cfg.SetNull(ctx, fx.tenantID, path)
						if nullable {
							assert.NoError(t, err, "nullable field must accept null")
						} else {
							assert.Error(t, err, "non-nullable field must reject null")
						}
						// Restore a non-null seed so subsequent ops have content.
						sample := tc.sample()
						require.NoError(t, cfg.SetTyped(ctx, fx.tenantID, path, sample))
						seeds[path] = sample
					})

					t.Run("get", func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						resp, err := cfgTransport.GetField(ctx, &configclient.GetFieldRequest{
							TenantID:  fx.tenantID,
							FieldPath: path,
						})
						require.NoError(t, err)
						require.NotNil(t, resp.Value, "field %s should not be null at get-time", path)
						tc.verifyEq(t, seeds[path], resp.Value)
					})

					t.Run("export_yaml", func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						out, err := admin.ExportConfig(ctx, fx.tenantID, nil)
						require.NoError(t, err)
						assert.Contains(t, string(out), path,
							"export must include field path %s", path)
					})

					t.Run("import_yaml", func(t *testing.T) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						sample := tc.sample()
						yaml := buildSingleFieldYAML(path, tc.yamlValue(sample))
						_, err := admin.ImportConfig(ctx, fx.tenantID, []byte(yaml),
							"matrix3 import "+path, adminclient.ImportModeMerge)
						require.NoError(t, err, "import yaml: %s", yaml)

						// Round-trip read.
						resp, err := cfgTransport.GetField(ctx, &configclient.GetFieldRequest{
							TenantID:  fx.tenantID,
							FieldPath: path,
						})
						require.NoError(t, err)
						require.NotNil(t, resp.Value)
						tc.verifyEq(t, sample, resp.Value)
						seeds[path] = sample
					})
				})
			}
		})
	}
}

// buildSingleFieldYAML constructs a minimal merge-import yaml document with
// a single field. The literal is interpolated as-is — yamlValue() returns
// a YAML-safe representation.
func buildSingleFieldYAML(fieldPath, yamlLiteral string) string {
	var b strings.Builder
	b.WriteString("spec_version: \"v1\"\nvalues:\n  ")
	b.WriteString(fieldPath)
	b.WriteString(":\n    value: ")
	b.WriteString(yamlLiteral)
	b.WriteString("\n")
	return b.String()
}

// newConfigTransportSuperadmin returns a default-superadmin
// ConfigTransport. The matrix calls Transport.GetField directly because
// configclient.Client only exposes type-specific getters but the matrix
// needs to compare TypedValue payloads regardless of kind.
func newConfigTransportSuperadmin(conn *grpc.ClientConn) *grpctransport.ConfigTransport {
	return grpctransport.NewConfigTransport(conn,
		grpctransport.WithSubject("e2e-type-matrix"))
}
