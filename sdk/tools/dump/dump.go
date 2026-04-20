// Package dump exports a full tenant backup (schema + config + locks) as a
// seed-compatible YAML file. The output of [Run] can be fed directly into
// [seed.Run] to recreate the tenant elsewhere.
package dump

import (
	"context"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/seed"
)

// Client defines the adminclient methods used by dump operations.
// The [adminclient.Client] type satisfies this interface.
type Client interface {
	GetTenant(ctx context.Context, id string) (*adminclient.Tenant, error)
	GetSchemaVersion(ctx context.Context, id string, version int32) (*adminclient.Schema, error)
	ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error)
	ListFieldLocks(ctx context.Context, tenantID string) ([]adminclient.FieldLock, error)
}

// Option configures dump behavior.
type Option func(*options)

type options struct {
	includeLocks  bool
	configVersion *int32
}

func defaults() options {
	return options{includeLocks: true}
}

// WithoutLocks excludes field locks from the dump.
func WithoutLocks() Option {
	return func(o *options) { o.includeLocks = false }
}

// WithConfigVersion pins the config export to a specific version.
// By default the latest version is exported.
func WithConfigVersion(v int32) Option {
	return func(o *options) { o.configVersion = &v }
}

// Run exports a full tenant backup as a seed-compatible file.
func Run(ctx context.Context, client Client, tenantID string, opts ...Option) (*seed.File, error) {
	o := defaults()
	for _, opt := range opts {
		opt(&o)
	}

	// 1. Get tenant.
	tenant, err := client.GetTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting tenant: %w", err)
	}

	// 2. Get schema at the tenant's version.
	schema, err := client.GetSchemaVersion(ctx, tenant.SchemaID, tenant.SchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("getting schema: %w", err)
	}

	// 3. Export config as YAML, then parse back into structured form.
	configYAML, err := client.ExportConfig(ctx, tenantID, o.configVersion)
	if err != nil {
		return nil, fmt.Errorf("exporting config: %w", err)
	}

	var configDoc struct {
		Values map[string]seed.ConfigValueDef `yaml:"values"`
	}
	if err := yaml.Unmarshal(configYAML, &configDoc); err != nil {
		return nil, fmt.Errorf("parsing exported config: %w", err)
	}

	// 4. Build seed file.
	file := &seed.File{
		SpecVersion: "v1",
		Schema:      buildSchemaDef(schema),
		Tenant: seed.TenantDef{Name: tenant.Name},
		Config: seed.ConfigDef{Values: configDoc.Values},
	}

	// 5. Export locks.
	if o.includeLocks {
		locks, err := client.ListFieldLocks(ctx, tenantID)
		if err != nil {
			return nil, fmt.Errorf("listing locks: %w", err)
		}
		for _, l := range locks {
			file.Locks = append(file.Locks, seed.LockDef{
				FieldPath:    l.FieldPath,
				LockedValues: l.LockedValues,
			})
		}
	}

	return file, nil
}

// Marshal serializes a dump file to YAML bytes.
func Marshal(f *seed.File) ([]byte, error) {
	return seed.Marshal(f)
}

// --- Helpers ---

// protoTypeToShort maps proto enum names to YAML short names.
var protoTypeToShort = map[string]string{
	"FIELD_TYPE_INT":      "integer",
	"FIELD_TYPE_NUMBER":   "number",
	"FIELD_TYPE_STRING":   "string",
	"FIELD_TYPE_BOOL":     "bool",
	"FIELD_TYPE_TIME":     "time",
	"FIELD_TYPE_DURATION": "duration",
	"FIELD_TYPE_URL":      "url",
	"FIELD_TYPE_JSON":     "json",
}

func fieldTypeName(protoName string) string {
	if short, ok := protoTypeToShort[protoName]; ok {
		return short
	}
	return protoName
}

func buildSchemaDef(s *adminclient.Schema) seed.SchemaDef {
	def := seed.SchemaDef{
		Name:        s.Name,
		Description: s.Description,
		Info:        convertSchemaInfo(s.Info),
		Fields:      make(map[string]seed.FieldDef, len(s.Fields)),
	}

	for _, f := range s.Fields {
		fd := seed.FieldDef{
			Type:        fieldTypeName(f.Type),
			Description: f.Description,
			Default:     f.Default,
			Nullable:    f.Nullable,
			Deprecated:  f.Deprecated,
			RedirectTo:  f.RedirectTo,
			Title:       f.Title,
			Example:     f.Example,
			Format:      f.Format,
			ReadOnly:    f.ReadOnly,
			WriteOnce:   f.WriteOnce,
			Sensitive:   f.Sensitive,
			Tags:        f.Tags,
		}
		if len(f.Examples) > 0 {
			fd.Examples = make(map[string]seed.ExampleDef, len(f.Examples))
			for k, v := range f.Examples {
				fd.Examples[k] = seed.ExampleDef{Value: v.Value, Summary: v.Summary}
			}
		}
		if f.ExternalDocs != nil {
			fd.ExternalDocs = &seed.ExternalDocsDef{
				Description: f.ExternalDocs.Description,
				URL:         f.ExternalDocs.URL,
			}
		}
		if f.Constraints != nil {
			fd.Constraints = convertConstraints(f.Constraints)
		}
		def.Fields[f.Path] = fd
	}

	return def
}

func convertSchemaInfo(info *adminclient.SchemaInfo) *seed.SchemaInfoDef {
	if info == nil {
		return nil
	}
	r := &seed.SchemaInfoDef{
		Title:  info.Title,
		Author: info.Author,
		Labels: info.Labels,
	}
	if info.Contact != nil {
		r.Contact = &seed.SchemaContactDef{
			Name:  info.Contact.Name,
			Email: info.Contact.Email,
			URL:   info.Contact.URL,
		}
	}
	return r
}

func convertConstraints(c *adminclient.FieldConstraints) *seed.ConstraintsDef {
	cd := &seed.ConstraintsDef{
		Minimum:          c.Min,
		Maximum:          c.Max,
		ExclusiveMinimum: c.ExclusiveMin,
		ExclusiveMaximum: c.ExclusiveMax,
		MinLength:        c.MinLength,
		MaxLength:        c.MaxLength,
		Pattern:          c.Pattern,
		JSONSchema:       c.JSONSchema,
	}
	if len(c.Enum) > 0 {
		cd.Enum = c.Enum
	}
	return cd
}
