package adminclient

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
)

// Schema represents a configuration schema with its fields.
type Schema struct {
	ID                 string
	Name               string
	Description        string
	Version            int32
	ParentVersion      *int32
	VersionDescription string
	Checksum           string
	Published          bool
	Fields             []Field
	CreatedAt          time.Time
}

// Field represents a single field definition within a schema.
// Field represents a single field definition within a schema.
type Field struct {
	Path        string
	Type        string
	Nullable    bool
	Deprecated  bool
	RedirectTo  string
	Default     string
	Description string
	Constraints *FieldConstraints
}

// FieldConstraints defines validation rules for a field.
type FieldConstraints struct {
	Min              *float64
	Max              *float64
	ExclusiveMin     *float64
	ExclusiveMax     *float64
	MinLength        *int32
	MaxLength        *int32
	Pattern          string
	Enum             []string
	JSONSchema       string
}

// Tenant represents a tenant assigned to a schema.
type Tenant struct {
	ID            string
	Name          string
	SchemaID      string
	SchemaVersion int32
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// FieldLock represents a locked field for a tenant.
type FieldLock struct {
	TenantID     string
	FieldPath    string
	LockedValues []string
}

// AuditEntry represents a config change event from the audit log.
type AuditEntry struct {
	ID            string
	TenantID      string
	Actor         string
	Action        string
	FieldPath     string
	OldValue      string
	NewValue      string
	ConfigVersion *int32
	CreatedAt     time.Time
}

// UsageStats represents aggregated read usage statistics for a field.
type UsageStats struct {
	TenantID   string
	FieldPath  string
	ReadCount  int64
	LastReadBy string
	LastReadAt *time.Time
}

// Version represents a config version snapshot.
type Version struct {
	ID          string
	TenantID    string
	Version     int32
	Description string
	CreatedBy   string
	CreatedAt   time.Time
}

// --- Proto conversion helpers ---

func schemaFromProto(s *pb.Schema) *Schema {
	if s == nil {
		return nil
	}
	r := &Schema{
		ID:                 s.Id,
		Name:               s.Name,
		Description:        s.Description,
		Version:            s.Version,
		ParentVersion:      s.ParentVersion,
		VersionDescription: s.VersionDescription,
		Checksum:           s.Checksum,
		Published:          s.Published,
		Fields:             make([]Field, len(s.Fields)),
		CreatedAt:          s.CreatedAt.AsTime(),
	}
	for i, f := range s.Fields {
		r.Fields[i] = fieldFromProto(f)
	}
	return r
}

func fieldFromProto(f *pb.SchemaField) Field {
	r := Field{
		Path:       f.Path,
		Type:       f.Type.String(),
		Nullable:   f.Nullable,
		Deprecated: f.Deprecated,
	}
	if f.RedirectTo != nil {
		r.RedirectTo = *f.RedirectTo
	}
	if f.DefaultValue != nil {
		r.Default = *f.DefaultValue
	}
	if f.Description != nil {
		r.Description = *f.Description
	}
	return r
}

func fieldsToProto(fields []Field) []*pb.SchemaField {
	result := make([]*pb.SchemaField, len(fields))
	for i, f := range fields {
		pf := &pb.SchemaField{
			Path:       f.Path,
			Nullable:   f.Nullable,
			Deprecated: f.Deprecated,
		}
		if f.RedirectTo != "" {
			pf.RedirectTo = &f.RedirectTo
		}
		if f.Default != "" {
			pf.DefaultValue = &f.Default
		}
		if f.Description != "" {
			pf.Description = &f.Description
		}
		// Type is set by name lookup — caller is responsible for valid type names.
		pf.Type = pb.FieldType(pb.FieldType_value["FIELD_TYPE_"+f.Type])
		if f.Constraints != nil {
			pf.Constraints = constraintsToProto(f.Constraints)
		}
		result[i] = pf
	}
	return result
}

func constraintsToProto(c *FieldConstraints) *pb.FieldConstraints {
	if c == nil {
		return nil
	}
	pc := &pb.FieldConstraints{
		Min:          c.Min,
		Max:          c.Max,
		ExclusiveMin: c.ExclusiveMin,
		ExclusiveMax: c.ExclusiveMax,
		MinLength:    c.MinLength,
		MaxLength:    c.MaxLength,
		EnumValues:   c.Enum,
	}
	if c.Pattern != "" {
		pc.Regex = &c.Pattern
	}
	if c.JSONSchema != "" {
		pc.JsonSchema = &c.JSONSchema
	}
	return pc
}

func tenantFromProto(t *pb.Tenant) *Tenant {
	if t == nil {
		return nil
	}
	return &Tenant{
		ID:            t.Id,
		Name:          t.Name,
		SchemaID:      t.SchemaId,
		SchemaVersion: t.SchemaVersion,
		CreatedAt:     t.CreatedAt.AsTime(),
		UpdatedAt:     t.UpdatedAt.AsTime(),
	}
}

func auditEntryFromProto(e *pb.AuditEntry) *AuditEntry {
	if e == nil {
		return nil
	}
	r := &AuditEntry{
		ID:            e.Id,
		TenantID:      e.TenantId,
		Actor:         e.Actor,
		Action:        e.Action,
		ConfigVersion: e.ConfigVersion,
		CreatedAt:     e.CreatedAt.AsTime(),
	}
	if e.FieldPath != nil {
		r.FieldPath = *e.FieldPath
	}
	if e.OldValue != nil {
		r.OldValue = *e.OldValue
	}
	if e.NewValue != nil {
		r.NewValue = *e.NewValue
	}
	return r
}

func usageStatsFromProto(s *pb.UsageStats) *UsageStats {
	if s == nil {
		return nil
	}
	r := &UsageStats{
		TenantID:  s.TenantId,
		FieldPath: s.FieldPath,
		ReadCount: s.ReadCount,
	}
	if s.LastReadBy != nil {
		r.LastReadBy = *s.LastReadBy
	}
	if s.LastReadAt != nil {
		t := s.LastReadAt.AsTime()
		r.LastReadAt = &t
	}
	return r
}

func versionFromProto(v *pb.ConfigVersion) *Version {
	if v == nil {
		return nil
	}
	return &Version{
		ID:          v.Id,
		TenantID:    v.TenantId,
		Version:     v.Version,
		Description: v.Description,
		CreatedBy:   v.CreatedBy,
		CreatedAt:   v.CreatedAt.AsTime(),
	}
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timeToProto(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}
