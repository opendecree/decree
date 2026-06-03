package config

import (
	"context"
	"errors"
	"time"

	"github.com/opendecree/decree/internal/storage/domain"
)

// ErrVersionConflict is returned by CreateConfigVersion when a concurrent
// writer already committed the same version number (UNIQUE constraint violation).
// Callers should surface this as codes.Aborted and advise a retry.
var ErrVersionConflict = errors.New("version conflict")

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

// GetConfigValuesSinceParams identifies config value deltas at or after a version.
type GetConfigValuesSinceParams struct {
	TenantID     string
	StartVersion int32
}

// ConfigValueSince is a single row from GetConfigValuesSince.
type ConfigValueSince struct {
	FieldPath string
	Value     *string
	Version   int32
	CreatedBy string
	ChangedAt time.Time
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
	// The Store passed to fn is bound to that transaction; all reads and writes
	// inside fn observe the same isolated view. If fn returns a non-nil error,
	// the transaction is rolled back and that error is returned unchanged. If fn
	// returns nil, the transaction is committed.
	//
	// Implementations must guarantee atomicity: either all writes from fn are
	// visible after a successful return, or none are (on error).
	RunInTx(ctx context.Context, fn func(Store) error) error

	// Config versions.
	CreateConfigVersion(ctx context.Context, arg CreateConfigVersionParams) (domain.ConfigVersion, error)
	GetConfigVersion(ctx context.Context, arg GetConfigVersionParams) (domain.ConfigVersion, error)
	GetLatestConfigVersion(ctx context.Context, tenantID string) (domain.ConfigVersion, error)
	ListConfigVersions(ctx context.Context, arg ListConfigVersionsParams) ([]domain.ConfigVersion, error)

	// Config values.
	SetConfigValue(ctx context.Context, arg SetConfigValueParams) error
	BulkSetConfigValues(ctx context.Context, args []SetConfigValueParams) error
	GetConfigValues(ctx context.Context, configVersionID string) ([]domain.ConfigValue, error)
	GetConfigValueAtVersion(ctx context.Context, arg GetConfigValueAtVersionParams) (GetConfigValueAtVersionRow, error)
	GetFullConfigAtVersion(ctx context.Context, arg GetFullConfigAtVersionParams) ([]GetFullConfigAtVersionRow, error)
	GetConfigValuesSince(ctx context.Context, arg GetConfigValuesSinceParams) ([]ConfigValueSince, error)

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
	BulkInsertAuditWriteLog(ctx context.Context, args []InsertAuditWriteLogParams) error
}
