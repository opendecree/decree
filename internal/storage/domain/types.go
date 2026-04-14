// Package domain defines database-agnostic entity types for the storage layer.
// Store interfaces use these types instead of database-specific types (pgtype, sqlc).
// This enables alternative storage backends (DynamoDB, MySQL, etc.) to implement
// the same interfaces without importing PostgreSQL-specific packages.
package domain

import (
	"errors"
	"time"
)

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// FieldType represents a schema field's value type.
type FieldType string

const (
	FieldTypeInteger  FieldType = "integer"
	FieldTypeNumber   FieldType = "number"
	FieldTypeString   FieldType = "string"
	FieldTypeBool     FieldType = "bool"
	FieldTypeTime     FieldType = "time"
	FieldTypeDuration FieldType = "duration"
	FieldTypeURL      FieldType = "url"
	FieldTypeJSON     FieldType = "json"
)

// Schema represents a configuration schema.
type Schema struct {
	ID          string
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// SchemaVersion represents a specific version of a schema.
type SchemaVersion struct {
	ID            string
	SchemaID      string
	Version       int32
	ParentVersion *int32
	Description   *string
	Checksum      string
	Published     bool
	CreatedAt     time.Time
}

// SchemaField represents a field definition within a schema version.
type SchemaField struct {
	ID              string
	SchemaVersionID string
	Path            string
	FieldType       FieldType
	Constraints     []byte
	Nullable        bool
	Deprecated      bool
	RedirectTo      *string
	DefaultValue    *string
	Description     *string
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

// TenantFieldLock represents a locked field for a tenant.
type TenantFieldLock struct {
	TenantID     string
	FieldPath    string
	LockedValues []byte
}

// SchemaVersionKey identifies a specific schema version by schema ID and version number.
// Used as a shared parameter type across service packages.
type SchemaVersionKey struct {
	SchemaID string
	Version  int32
}

// ConfigVersion represents a config version snapshot.
type ConfigVersion struct {
	ID          string
	TenantID    string
	Version     int32
	Description *string
	CreatedBy   string
	CreatedAt   time.Time
}

// ConfigValue represents a single config field's value at a version.
type ConfigValue struct {
	ConfigVersionID string
	FieldPath       string
	Value           *string
	Checksum        *string
	Description     *string
}

// AuditWriteLog represents a config change event in the audit log.
type AuditWriteLog struct {
	ID            string
	TenantID      string
	Actor         string
	Action        string
	FieldPath     *string
	OldValue      *string
	NewValue      *string
	ConfigVersion *int32
	Metadata      []byte
	CreatedAt     time.Time
}

// UsageStat represents read usage statistics for a field.
type UsageStat struct {
	TenantID    string
	FieldPath   string
	PeriodStart time.Time
	ReadCount   int64
	LastReadBy  *string
	LastReadAt  *time.Time
}

// TenantUsageRow represents aggregated usage for a tenant's field.
type TenantUsageRow struct {
	FieldPath  string
	ReadCount  int64
	LastReadAt *time.Time
}
