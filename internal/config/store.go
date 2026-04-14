package config

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
)

// Store defines the data access interface for config operations.
type Store interface {
	// RunInTx executes fn within a database transaction.
	// The Store passed to fn is bound to the transaction.
	// If fn returns nil the transaction is committed; otherwise it is rolled back.
	RunInTx(ctx context.Context, fn func(Store) error) error

	// Config versions.
	CreateConfigVersion(ctx context.Context, arg dbstore.CreateConfigVersionParams) (dbstore.ConfigVersion, error)
	GetConfigVersion(ctx context.Context, arg dbstore.GetConfigVersionParams) (dbstore.ConfigVersion, error)
	GetLatestConfigVersion(ctx context.Context, tenantID pgtype.UUID) (dbstore.ConfigVersion, error)
	ListConfigVersions(ctx context.Context, arg dbstore.ListConfigVersionsParams) ([]dbstore.ConfigVersion, error)

	// Config values.
	SetConfigValue(ctx context.Context, arg dbstore.SetConfigValueParams) error
	GetConfigValues(ctx context.Context, configVersionID pgtype.UUID) ([]dbstore.ConfigValue, error)
	GetConfigValueAtVersion(ctx context.Context, arg dbstore.GetConfigValueAtVersionParams) (dbstore.GetConfigValueAtVersionRow, error)
	GetFullConfigAtVersion(ctx context.Context, arg dbstore.GetFullConfigAtVersionParams) ([]dbstore.GetFullConfigAtVersionRow, error)

	// Tenant lookup (needed for validation).
	GetTenantByID(ctx context.Context, id pgtype.UUID) (dbstore.Tenant, error)

	// Schema field lookup (needed for validation).
	GetSchemaFields(ctx context.Context, schemaVersionID pgtype.UUID) ([]dbstore.SchemaField, error)
	GetSchemaVersion(ctx context.Context, arg dbstore.GetSchemaVersionParams) (dbstore.SchemaVersion, error)

	// Field locks (needed for write validation).
	GetFieldLocks(ctx context.Context, tenantID pgtype.UUID) ([]dbstore.TenantFieldLock, error)

	// Audit.
	InsertAuditWriteLog(ctx context.Context, arg dbstore.InsertAuditWriteLogParams) error
}
