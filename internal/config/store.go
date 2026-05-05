package config

import (
	"context"

	"github.com/opendecree/decree/internal/storage/domain"
)

// --- Local param/result types ---

// CreateConfigVersionParams contains parameters for creating a config version.
type CreateConfigVersionParams struct {
	TenantID    string
	Version     int32
	Description *string
	CreatedBy   string
}

// GetConfigVersionParams identifies a specific config version.
type GetConfigVersionParams struct {
	TenantID string
	Version  int32
}

// ListConfigVersionsParams contains pagination parameters for listing config versions.
type ListConfigVersionsParams struct {
	TenantID string
	Limit    int32
	Offset   int32
}

// SetConfigValueParams contains parameters for setting a config value.
type SetConfigValueParams struct {
	ConfigVersionID string
	FieldPath       string
	Value           *string
	Checksum        *string
	Description     *string
}

// GetConfigValueAtVersionParams identifies a config value at a specific version.
type GetConfigValueAtVersionParams struct {
	TenantID  string
	FieldPath string
	Version   int32
}

// GetConfigValueAtVersionRow is the result of GetConfigValueAtVersion.
type GetConfigValueAtVersionRow struct {
	FieldPath   string
	Value       *string
	Checksum    *string
	Description *string
}

// GetFullConfigAtVersionParams identifies a full config snapshot at a version.
type GetFullConfigAtVersionParams struct {
	TenantID string
	Version  int32
}

// GetFullConfigAtVersionRow is a single row from GetFullConfigAtVersion.
type GetFullConfigAtVersionRow struct {
	FieldPath   string
	Value       *string
	Checksum    *string
	Description *string
}

// InsertAuditWriteLogParams contains parameters for inserting an audit log entry.
type InsertAuditWriteLogParams struct {
	TenantID      string
	Actor         string
	Action        string
	ObjectKind    string // "field", "schema", "tenant", or "lock"; defaults to "field"
	FieldPath     *string
	OldValue      *string
	NewValue      *string
	ConfigVersion *int32
	Metadata      []byte
}

// Store defines the data access interface for config operations.
// Implementations must return [domain.ErrNotFound] when an entity is not found.
type Store interface {
	// RunInTx executes fn within a database transaction.
	// The Store passed to fn is bound to the transaction.
	// If fn returns nil the transaction is committed; otherwise it is rolled back.
	RunInTx(ctx context.Context, fn func(Store) error) error

	// Config versions.
	CreateConfigVersion(ctx context.Context, arg CreateConfigVersionParams) (domain.ConfigVersion, error)
	GetConfigVersion(ctx context.Context, arg GetConfigVersionParams) (domain.ConfigVersion, error)
	GetLatestConfigVersion(ctx context.Context, tenantID string) (domain.ConfigVersion, error)
	ListConfigVersions(ctx context.Context, arg ListConfigVersionsParams) ([]domain.ConfigVersion, error)

	// Config values.
	SetConfigValue(ctx context.Context, arg SetConfigValueParams) error
	GetConfigValues(ctx context.Context, configVersionID string) ([]domain.ConfigValue, error)
	GetConfigValueAtVersion(ctx context.Context, arg GetConfigValueAtVersionParams) (GetConfigValueAtVersionRow, error)
	GetFullConfigAtVersion(ctx context.Context, arg GetFullConfigAtVersionParams) ([]GetFullConfigAtVersionRow, error)

	// Tenant lookup (needed for validation and slug resolution).
	GetTenantByID(ctx context.Context, id string) (domain.Tenant, error)
	GetTenantByName(ctx context.Context, name string) (domain.Tenant, error)

	// Schema field lookup (needed for validation).
	GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error)
	GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error)

	// Field locks (needed for write validation).
	GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error)

	// Audit.
	InsertAuditWriteLog(ctx context.Context, arg InsertAuditWriteLogParams) error
}
