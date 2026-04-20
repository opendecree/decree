// Package seed bootstraps an OpenDecree environment from a single YAML file.
// A seed file can contain any combination of schema, tenant, config, and locks
// sections. The [Run] function dispatches based on which sections are present:
//
//   - schema only                  → ImportSchema
//   - tenant only                  → CreateTenant (reuse if exists)
//   - schema + tenant              → ImportSchema + CreateTenant
//   - tenant + config (+ locks)    → CreateTenant + ImportConfig + LockField
//   - schema + tenant + config     → all three (combined envelope, legacy form)
//
// In config-only mode, [TenantDef.Schema] references an already-imported schema
// by name; if [TenantDef.SchemaVersion] is nil, the latest published version is
// used.
//
// The operation is idempotent: importing a schema with identical fields, or a
// config whose values match the latest version, is a no-op and does not create
// a new version. Existing tenants are reused.
//
// The [File] type also serves as the shared YAML format for the dump package.
package seed

import (
	"context"
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/opendecree/decree/sdk/adminclient"
)

// --- Seed/dump YAML format ---

// File is the top-level YAML document for seed and dump operations.
// Each of schema, tenant, and config is independently optional; see the package
// docs for the valid combinations.
type File struct {
	SpecVersion string    `yaml:"spec_version"`
	Schema      SchemaDef `yaml:"schema,omitempty"`
	Tenant      TenantDef `yaml:"tenant,omitempty"`
	Config      ConfigDef `yaml:"config,omitempty"`
	Locks       []LockDef `yaml:"locks,omitempty"`
}

// SchemaDef defines a schema within a seed file.
type SchemaDef struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description,omitempty"`
	Info        *SchemaInfoDef      `yaml:"info,omitempty"`
	Fields      map[string]FieldDef `yaml:"fields"`
}

// SchemaInfoDef contains optional schema-level metadata.
type SchemaInfoDef struct {
	Title   string            `yaml:"title,omitempty"`
	Author  string            `yaml:"author,omitempty"`
	Contact *SchemaContactDef `yaml:"contact,omitempty"`
	Labels  map[string]string `yaml:"labels,omitempty"`
}

// SchemaContactDef contains contact information for a schema owner.
type SchemaContactDef struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

// FieldDef describes a single schema field.
type FieldDef struct {
	Type         string                `yaml:"type"`
	Description  string                `yaml:"description,omitempty"`
	Default      string                `yaml:"default,omitempty"`
	Nullable     bool                  `yaml:"nullable,omitempty"`
	Deprecated   bool                  `yaml:"deprecated,omitempty"`
	RedirectTo   string                `yaml:"redirect_to,omitempty"`
	Constraints  *ConstraintsDef       `yaml:"constraints,omitempty"`
	Title        string                `yaml:"title,omitempty"`
	Example      string                `yaml:"example,omitempty"`
	Examples     map[string]ExampleDef `yaml:"examples,omitempty"`
	ExternalDocs *ExternalDocsDef      `yaml:"externalDocs,omitempty"`
	Tags         []string              `yaml:"tags,omitempty"`
	Format       string                `yaml:"format,omitempty"`
	ReadOnly     bool                  `yaml:"readOnly,omitempty"`
	WriteOnce    bool                  `yaml:"writeOnce,omitempty"`
	Sensitive    bool                  `yaml:"sensitive,omitempty"`
}

// ExampleDef represents a named example value.
type ExampleDef struct {
	Value   string `yaml:"value"`
	Summary string `yaml:"summary,omitempty"`
}

// ExternalDocsDef links to external documentation.
type ExternalDocsDef struct {
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url"`
}

// ConstraintsDef uses OAS-style naming for field constraints.
type ConstraintsDef struct {
	Minimum          *float64 `yaml:"minimum,omitempty"`
	Maximum          *float64 `yaml:"maximum,omitempty"`
	ExclusiveMinimum *float64 `yaml:"exclusiveMinimum,omitempty"`
	ExclusiveMaximum *float64 `yaml:"exclusiveMaximum,omitempty"`
	MinLength        *int32   `yaml:"minLength,omitempty"`
	MaxLength        *int32   `yaml:"maxLength,omitempty"`
	Pattern          string   `yaml:"pattern,omitempty"`
	Enum             []string `yaml:"enum,omitempty"`
	JSONSchema       string   `yaml:"json_schema,omitempty"`
}

// TenantDef defines the tenant to create or reuse.
//
// In combined mode (file also contains a schema section), Schema and
// SchemaVersion are ignored — the tenant binds to the co-located schema.
//
// In config-only mode (no schema section), Schema names an already-imported
// schema. If SchemaVersion is nil, the latest published version is used.
type TenantDef struct {
	Name          string `yaml:"name"`
	Schema        string `yaml:"schema,omitempty"`
	SchemaVersion *int32 `yaml:"schema_version,omitempty"`
}

// ConfigDef defines initial configuration values.
type ConfigDef struct {
	Description string                    `yaml:"description,omitempty"`
	Values      map[string]ConfigValueDef `yaml:"values,omitempty"`
}

// ConfigValueDef represents a single config value.
type ConfigValueDef struct {
	Value       any    `yaml:"value"`
	Description string `yaml:"description,omitempty"`
}

// LockDef defines a field lock.
type LockDef struct {
	FieldPath    string   `yaml:"field_path"`
	LockedValues []string `yaml:"locked_values,omitempty"`
}

// --- Client interface ---

// Client defines the adminclient methods used by seed operations.
// The [adminclient.Client] type satisfies this interface.
type Client interface {
	ImportSchema(ctx context.Context, yamlContent []byte, autoPublish ...bool) (*adminclient.Schema, error)
	ListSchemas(ctx context.Context) ([]*adminclient.Schema, error)
	GetLatestPublishedSchemaVersion(ctx context.Context, name string) (string, int32, error)
	ListTenants(ctx context.Context, schemaID string) ([]*adminclient.Tenant, error)
	CreateTenant(ctx context.Context, name, schemaID string, schemaVersion int32) (*adminclient.Tenant, error)
	ImportConfig(ctx context.Context, tenantID string, yamlContent []byte, description string, mode ...adminclient.ImportMode) (*adminclient.Version, error)
	ListConfigVersions(ctx context.Context, tenantID string) ([]*adminclient.Version, error)
	LockField(ctx context.Context, tenantID, fieldPath string, lockedValues ...string) error
}

// --- Parse ---

// ParseFile parses and validates a seed YAML file. Validation depends on which
// top-level sections are present; see the package docs for the valid shapes.
func ParseFile(data []byte) (*File, error) {
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if f.SpecVersion != "v1" {
		return nil, fmt.Errorf("unsupported spec_version: %q (expected \"v1\")", f.SpecVersion)
	}

	hasSchema := f.Schema.Name != "" || len(f.Schema.Fields) > 0
	hasTenant := f.Tenant.Name != ""
	hasConfig := len(f.Config.Values) > 0
	hasLocks := len(f.Locks) > 0

	if !hasSchema && !hasTenant && !hasConfig && !hasLocks {
		return nil, fmt.Errorf("seed file is empty — need at least one of schema, tenant, config, or locks")
	}

	if hasSchema {
		if f.Schema.Name == "" {
			return nil, fmt.Errorf("schema.name is required when schema section is present")
		}
		if len(f.Schema.Fields) == 0 {
			return nil, fmt.Errorf("schema.fields must have at least one field")
		}
	}

	if (hasConfig || hasLocks) && !hasTenant {
		return nil, fmt.Errorf("config and locks require a tenant section")
	}

	if hasTenant {
		if f.Tenant.Name == "" {
			return nil, fmt.Errorf("tenant.name is required when tenant section is present")
		}
		// Config-only mode: tenant must name its schema.
		if !hasSchema && f.Tenant.Schema == "" {
			return nil, fmt.Errorf("tenant.schema is required when file has no schema section (config-only mode)")
		}
	}

	return &f, nil
}

// Marshal serializes a seed/dump file to YAML.
func Marshal(f *File) ([]byte, error) {
	return yaml.Marshal(f)
}

// --- Seed execution ---

// Result reports what happened during seeding.
type Result struct {
	SchemaID       string
	SchemaVersion  int32
	SchemaCreated  bool // false if skipped (already existed with same fields)
	TenantID       string
	TenantCreated  bool // false if reused existing tenant
	ConfigVersion  int32
	ConfigImported bool
	LocksApplied   int
}

// Option configures seed behavior.
type Option func(*options)

type options struct {
	autoPublish bool
}

// AutoPublish publishes the schema version after creation.
func AutoPublish() Option {
	return func(o *options) { o.autoPublish = true }
}

// Run executes the seed operation against a live server.
// It is idempotent: importing a schema with identical fields, or a config
// whose values match the latest version, is a no-op and does not create
// a new version. Existing tenants are reused.
func Run(ctx context.Context, client Client, file *File, opts ...Option) (*Result, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}

	result := &Result{}

	hasSchema := file.Schema.Name != ""
	hasTenant := file.Tenant.Name != ""
	hasConfig := len(file.Config.Values) > 0

	// 1. Resolve schema — either import it (schema section present) or look up
	//    an existing one by name (config-only mode).
	if hasSchema {
		if err := importSchema(ctx, client, file, o, result); err != nil {
			return nil, err
		}
	} else if hasTenant {
		if err := resolveSchemaRef(ctx, client, file, result); err != nil {
			return nil, err
		}
	}

	// 2. Find or create tenant.
	if hasTenant {
		if err := resolveTenant(ctx, client, file, result); err != nil {
			return nil, err
		}
	}

	// 3. Import config.
	if hasConfig {
		if err := importConfig(ctx, client, file, result); err != nil {
			return nil, err
		}
	}

	// 4. Apply locks.
	for _, lock := range file.Locks {
		if err := client.LockField(ctx, result.TenantID, lock.FieldPath, lock.LockedValues...); err != nil {
			return nil, fmt.Errorf("locking field %s: %w", lock.FieldPath, err)
		}
		result.LocksApplied++
	}

	return result, nil
}

func importSchema(ctx context.Context, client Client, file *File, o options, result *Result) error {
	schemaYAML, err := marshalSchemaYAML(file)
	if err != nil {
		return fmt.Errorf("marshaling schema: %w", err)
	}

	schema, err := client.ImportSchema(ctx, schemaYAML, o.autoPublish)
	if err == nil {
		result.SchemaID = schema.ID
		result.SchemaVersion = schema.Version
		result.SchemaCreated = true
		return nil
	}
	if !errors.Is(err, adminclient.ErrAlreadyExists) {
		return fmt.Errorf("importing schema: %w", err)
	}
	// Fields identical to latest version — look up the existing schema.
	schemas, listErr := client.ListSchemas(ctx)
	if listErr != nil {
		return fmt.Errorf("listing schemas: %w", listErr)
	}
	for _, s := range schemas {
		if s.Name == file.Schema.Name {
			result.SchemaID = s.ID
			result.SchemaVersion = s.Version
			return nil
		}
	}
	return fmt.Errorf("schema %q reported as existing but not found", file.Schema.Name)
}

// resolveSchemaRef looks up an existing schema by the reference in tenant.schema.
// Uses tenant.schema_version if set, otherwise resolves the latest published version.
func resolveSchemaRef(ctx context.Context, client Client, file *File, result *Result) error {
	if file.Tenant.SchemaVersion != nil {
		schemas, err := client.ListSchemas(ctx)
		if err != nil {
			return fmt.Errorf("listing schemas: %w", err)
		}
		for _, s := range schemas {
			if s.Name == file.Tenant.Schema {
				result.SchemaID = s.ID
				result.SchemaVersion = *file.Tenant.SchemaVersion
				return nil
			}
		}
		return fmt.Errorf("schema %q not found", file.Tenant.Schema)
	}
	id, version, err := client.GetLatestPublishedSchemaVersion(ctx, file.Tenant.Schema)
	if err != nil {
		if errors.Is(err, adminclient.ErrNotFound) {
			return fmt.Errorf("no published version of schema %q found", file.Tenant.Schema)
		}
		return fmt.Errorf("resolving schema %q: %w", file.Tenant.Schema, err)
	}
	result.SchemaID = id
	result.SchemaVersion = version
	return nil
}

func resolveTenant(ctx context.Context, client Client, file *File, result *Result) error {
	tenants, err := client.ListTenants(ctx, result.SchemaID)
	if err != nil {
		return fmt.Errorf("listing tenants: %w", err)
	}
	for _, t := range tenants {
		if t.Name == file.Tenant.Name {
			result.TenantID = t.ID
			return nil
		}
	}
	// Tenant not found under this schema. In config-only mode, check if the
	// tenant exists under a different schema and fail cleanly rather than
	// silently creating a duplicate.
	if file.Schema.Name == "" {
		allSchemas, err := client.ListSchemas(ctx)
		if err == nil {
			for _, s := range allSchemas {
				if s.ID == result.SchemaID {
					continue
				}
				others, _ := client.ListTenants(ctx, s.ID)
				for _, t := range others {
					if t.Name == file.Tenant.Name {
						return fmt.Errorf("tenant %q is bound to schema %q, not %q", t.Name, s.Name, file.Tenant.Schema)
					}
				}
			}
		}
	}
	tenant, err := client.CreateTenant(ctx, file.Tenant.Name, result.SchemaID, result.SchemaVersion)
	if err != nil {
		return fmt.Errorf("creating tenant: %w", err)
	}
	result.TenantID = tenant.ID
	result.TenantCreated = true
	return nil
}

func importConfig(ctx context.Context, client Client, file *File, result *Result) error {
	configYAML, err := marshalConfigYAML(file)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	ver, err := client.ImportConfig(ctx, result.TenantID, configYAML, file.Config.Description, adminclient.ImportModeMerge)
	if err == nil {
		result.ConfigVersion = ver.Version
		result.ConfigImported = true
		return nil
	}
	if !errors.Is(err, adminclient.ErrAlreadyExists) {
		return fmt.Errorf("importing config: %w", err)
	}
	// Values match the latest version — report the existing version, no import.
	versions, listErr := client.ListConfigVersions(ctx, result.TenantID)
	if listErr != nil {
		return fmt.Errorf("listing config versions: %w", listErr)
	}
	var latest int32
	for _, v := range versions {
		if v.Version > latest {
			latest = v.Version
		}
	}
	result.ConfigVersion = latest
	return nil
}

// --- Internal helpers to marshal sub-documents ---

// marshalSchemaYAML extracts the schema section into the standard schema YAML format.
func marshalSchemaYAML(f *File) ([]byte, error) {
	doc := struct {
		SpecVersion string              `yaml:"spec_version"`
		Name        string              `yaml:"name"`
		Description string              `yaml:"description,omitempty"`
		Fields      map[string]FieldDef `yaml:"fields"`
	}{
		SpecVersion: "v1",
		Name:        f.Schema.Name,
		Description: f.Schema.Description,
		Fields:      f.Schema.Fields,
	}
	return yaml.Marshal(doc)
}

// marshalConfigYAML extracts the config section into the standard config YAML format.
func marshalConfigYAML(f *File) ([]byte, error) {
	doc := struct {
		SpecVersion string                    `yaml:"spec_version"`
		Description string                    `yaml:"description,omitempty"`
		Values      map[string]ConfigValueDef `yaml:"values"`
	}{
		SpecVersion: "v1",
		Description: f.Config.Description,
		Values:      f.Config.Values,
	}
	return yaml.Marshal(doc)
}
