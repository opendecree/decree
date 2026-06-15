// Package loader populates the decree-docs documentation model from schema
// sources: a local schema YAML file (via sdk/tools/validate) or a running
// decree server (via sdk/adminclient).
//
// Both loaders produce identical [docmodel.Document] values for equivalent
// schema content. To keep that guarantee independent of source quirks the
// mapping canonicalizes the model: fields are sorted by path regardless of
// source order, and empty collections and empty optional blocks (tags,
// labels, examples, constraints, info, contact, externalDocs) become nil —
// absent and empty mean the same thing. Server-side bookkeeping that schema
// YAML cannot express (ID, parent version, checksum, published state,
// creation time) is excluded from the model; see the docmodel package
// documentation.
package loader

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/opendecree/decree/contrib/decree-docs/docmodel"
	"github.com/opendecree/decree/sdk/adminclient"
	"github.com/opendecree/decree/sdk/tools/validate"
)

// --- File loader ---

// FromFile loads the doc model from a schema YAML file on disk.
func FromFile(path string) (*docmodel.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema file: %w", err)
	}
	doc, err := FromYAML(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

// FromYAML builds the doc model from schema YAML content. The content is
// validated with the same rules the decree CLI applies (spec_version "v1",
// known field types, defaults satisfying constraints).
func FromYAML(data []byte) (*docmodel.Document, error) {
	sf, err := validate.ParseSchema(data)
	if err != nil {
		return nil, fmt.Errorf("invalid schema YAML: %w", err)
	}
	s := docmodel.Schema{
		Name:               sf.Name,
		Description:        sf.Description,
		Version:            sf.Version,
		VersionDescription: sf.VersionDescription,
		Info:               infoFromYAML(sf.Info),
		Fields:             make([]docmodel.Field, 0, len(sf.Fields)),
	}
	for path, fd := range sf.Fields {
		s.Fields = append(s.Fields, fieldFromYAML(path, fd))
	}
	sortFields(s.Fields)
	return docmodel.New(s), nil
}

func infoFromYAML(info *validate.SchemaInfoDef) *docmodel.Info {
	if info == nil {
		return nil
	}
	di := &docmodel.Info{
		Title:  info.Title,
		Author: info.Author,
		Labels: cloneMap(info.Labels),
	}
	if c := info.Contact; c != nil {
		di.Contact = canonicalContact(&docmodel.Contact{Name: c.Name, Email: c.Email, URL: c.URL})
	}
	return canonicalInfo(di)
}

func fieldFromYAML(path string, fd validate.FieldDef) docmodel.Field {
	f := docmodel.Field{
		Path:        path,
		Type:        fd.Type,
		Title:       fd.Title,
		Description: fd.Description,
		Default:     fd.Default,
		Nullable:    fd.Nullable,
		Deprecated:  fd.Deprecated,
		RedirectTo:  fd.RedirectTo,
		Example:     fd.Example,
		Tags:        cloneSlice(fd.Tags),
		Format:      fd.Format,
		ReadOnly:    fd.ReadOnly,
		WriteOnce:   fd.WriteOnce,
		Sensitive:   fd.Sensitive,
	}
	if len(fd.Examples) > 0 {
		f.Examples = make(map[string]docmodel.Example, len(fd.Examples))
		for name, ex := range fd.Examples {
			f.Examples[name] = docmodel.Example{Value: ex.Value, Summary: ex.Summary}
		}
	}
	if d := fd.ExternalDocs; d != nil {
		f.ExternalDocs = canonicalExternalDocs(&docmodel.ExternalDocs{Description: d.Description, URL: d.URL})
	}
	if c := fd.Constraints; c != nil {
		f.Constraints = canonicalConstraints(&docmodel.Constraints{
			Minimum:          c.Minimum,
			Maximum:          c.Maximum,
			ExclusiveMinimum: c.ExclusiveMinimum,
			ExclusiveMaximum: c.ExclusiveMaximum,
			MinLength:        c.MinLength,
			MaxLength:        c.MaxLength,
			Pattern:          c.Pattern,
			Enum:             cloneSlice(c.Enum),
			JSONSchema:       c.JSONSchema,
			AllowedSchemes:   cloneSlice(c.AllowedSchemes),
		})
	}
	return f
}

// --- Server loader ---

// SchemaClient is the subset of the admin client surface the server loader
// consumes. *adminclient.Client implements it; tests substitute a fake or
// wire a fake transport into a real client via adminclient.New.
type SchemaClient interface {
	GetSchema(ctx context.Context, id string) (*adminclient.Schema, error)
	GetSchemaVersion(ctx context.Context, id string, version int32) (*adminclient.Schema, error)
}

// FromServer fetches a schema from a decree server and builds the doc model
// from it. version selects a specific schema version; 0 means latest.
func FromServer(ctx context.Context, client SchemaClient, schemaID string, version int32) (*docmodel.Document, error) {
	var (
		s   *adminclient.Schema
		err error
	)
	if version > 0 {
		s, err = client.GetSchemaVersion(ctx, schemaID, version)
	} else {
		s, err = client.GetSchema(ctx, schemaID)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch schema %s: %w", schemaID, err)
	}
	return FromAdminSchema(s), nil
}

// FromAdminSchema builds the doc model from an already-fetched admin schema.
// It must map every documentation-relevant property the admin client carries
// so that server mode documents a schema identically to file mode;
// TestFromAdminSchema_PropertyDrift guards the mapping against new
// adminclient properties.
func FromAdminSchema(s *adminclient.Schema) *docmodel.Document {
	ds := docmodel.Schema{
		Name:               s.Name,
		Description:        s.Description,
		Version:            s.Version,
		VersionDescription: s.VersionDescription,
		Info:               infoFromAdmin(s.Info),
		Fields:             make([]docmodel.Field, 0, len(s.Fields)),
	}
	for _, f := range s.Fields {
		ds.Fields = append(ds.Fields, fieldFromAdmin(f))
	}
	sortFields(ds.Fields)
	return docmodel.New(ds)
}

func infoFromAdmin(info *adminclient.SchemaInfo) *docmodel.Info {
	if info == nil {
		return nil
	}
	di := &docmodel.Info{
		Title:  info.Title,
		Author: info.Author,
		Labels: cloneMap(info.Labels),
	}
	if c := info.Contact; c != nil {
		di.Contact = canonicalContact(&docmodel.Contact{Name: c.Name, Email: c.Email, URL: c.URL})
	}
	return canonicalInfo(di)
}

func fieldFromAdmin(fd adminclient.Field) docmodel.Field {
	f := docmodel.Field{
		Path:        fd.Path,
		Type:        string(fd.Type),
		Title:       fd.Title,
		Description: fd.Description,
		Default:     fd.Default,
		Nullable:    fd.Nullable,
		Deprecated:  fd.Deprecated,
		RedirectTo:  fd.RedirectTo,
		Example:     fd.Example,
		Tags:        cloneSlice(fd.Tags),
		Format:      fd.Format,
		ReadOnly:    fd.ReadOnly,
		WriteOnce:   fd.WriteOnce,
		Sensitive:   fd.Sensitive,
	}
	if len(fd.Examples) > 0 {
		f.Examples = make(map[string]docmodel.Example, len(fd.Examples))
		for name, ex := range fd.Examples {
			f.Examples[name] = docmodel.Example{Value: ex.Value, Summary: ex.Summary}
		}
	}
	if d := fd.ExternalDocs; d != nil {
		f.ExternalDocs = canonicalExternalDocs(&docmodel.ExternalDocs{Description: d.Description, URL: d.URL})
	}
	if c := fd.Constraints; c != nil {
		f.Constraints = canonicalConstraints(&docmodel.Constraints{
			Minimum:          c.Min,
			Maximum:          c.Max,
			ExclusiveMinimum: c.ExclusiveMin,
			ExclusiveMaximum: c.ExclusiveMax,
			MinLength:        c.MinLength,
			MaxLength:        c.MaxLength,
			Pattern:          c.Pattern,
			Enum:             cloneSlice(c.Enum),
			JSONSchema:       c.JSONSchema,
			AllowedSchemes:   cloneSlice(c.AllowedSchemes),
		})
	}
	return f
}

// --- Canonicalization helpers ---

func sortFields(fields []docmodel.Field) {
	slices.SortFunc(fields, func(a, b docmodel.Field) int {
		return strings.Compare(a.Path, b.Path)
	})
}

// cloneSlice copies s into the model, mapping empty (or nil) to nil.
func cloneSlice[T any](s []T) []T {
	if len(s) == 0 {
		return nil
	}
	return slices.Clone(s)
}

// cloneMap copies m into the model, mapping empty (or nil) to nil.
func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	if len(m) == 0 {
		return nil
	}
	return maps.Clone(m)
}

// canonicalInfo collapses an info block with no content to nil.
func canonicalInfo(i *docmodel.Info) *docmodel.Info {
	if i.Title == "" && i.Author == "" && i.Contact == nil && len(i.Labels) == 0 {
		return nil
	}
	return i
}

// canonicalContact collapses a contact with no content to nil.
func canonicalContact(c *docmodel.Contact) *docmodel.Contact {
	if c.Name == "" && c.Email == "" && c.URL == "" {
		return nil
	}
	return c
}

// canonicalExternalDocs collapses an externalDocs block with no content to nil.
func canonicalExternalDocs(d *docmodel.ExternalDocs) *docmodel.ExternalDocs {
	if d.Description == "" && d.URL == "" {
		return nil
	}
	return d
}

// canonicalConstraints collapses a constraints block with no rules to nil.
func canonicalConstraints(c *docmodel.Constraints) *docmodel.Constraints {
	if c.Minimum == nil && c.Maximum == nil &&
		c.ExclusiveMinimum == nil && c.ExclusiveMaximum == nil &&
		c.MinLength == nil && c.MaxLength == nil &&
		c.Pattern == "" && len(c.Enum) == 0 &&
		c.JSONSchema == "" && len(c.AllowedSchemes) == 0 {
		return nil
	}
	return c
}
