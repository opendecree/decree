package adminclient

import "time"

// FieldType is the data type of a schema field.
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
	Info               *SchemaInfo
}

// SchemaInfo contains optional schema-level metadata.
type SchemaInfo struct {
	Title   string
	Author  string
	Contact *SchemaContact
	Labels  map[string]string
}

// SchemaContact contains contact information for a schema owner.
type SchemaContact struct {
	Name  string
	Email string
	URL   string
}

// Field represents a single field definition within a schema.
type Field struct {
	Path         string
	Type         FieldType
	Nullable     bool
	Deprecated   bool
	RedirectTo   string
	Default      string
	Description  string
	Constraints  *FieldConstraints
	Title        string
	Example      string
	Examples     map[string]FieldExample
	ExternalDocs *ExternalDocs
	Tags         []string
	Format       string
	ReadOnly     bool
	WriteOnce    bool
	Sensitive    bool
}

// FieldExample represents a named example value.
type FieldExample struct {
	Value   string
	Summary string
}

// ExternalDocs links to external documentation.
type ExternalDocs struct {
	Description string
	URL         string
}

// FieldConstraints defines validation rules for a field.
type FieldConstraints struct {
	Min          *float64
	Max          *float64
	ExclusiveMin *float64
	ExclusiveMax *float64
	MinLength    *int32
	MaxLength    *int32
	Pattern      string
	Enum         []string
	JSONSchema   string
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
	ObjectKind    string
	FieldPath     string
	OldValue      string
	NewValue      string
	ConfigVersion *int32
	PreviousHash  string
	EntryHash     string
	CreatedAt     time.Time
	// ChainEpoch is the hash scheme epoch. 0 = legacy (structural fields only).
	// 1+ = full payload included in the hash.
	ChainEpoch uint64
	// Metadata contains arbitrary key-value pairs attached to this entry by the server.
	Metadata map[string]string
}

// VerifyChainResult is the outcome of a local audit chain verification.
type VerifyChainResult struct {
	TenantID string
	Total    int
	OK       bool
	Breaks   []VerifyChainBreak
}

// VerifyChainBreak describes a single tampered or missing link in the audit chain.
type VerifyChainBreak struct {
	EntryID  string
	Position int
	Got      string // stored value (entry_hash or previous_hash)
	Want     string // expected value
	Reason   string // human-readable description of the break (e.g. "hash mismatch" or "chain is truncated: ...")
}

// UsageStats represents aggregated read usage statistics for a field.
type UsageStats struct {
	TenantID   string
	FieldPath  string
	ReadCount  int64
	LastReadBy string
	LastReadAt *time.Time
}

// ServerInfo contains the server's version and enabled features.
type ServerInfo struct {
	Version  string
	Commit   string
	Features map[string]bool
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

// ChangeType categorizes how a field changed between two config versions.
type ChangeType int32

const (
	// ChangeTypeUnspecified is the zero value; never returned by the server.
	ChangeTypeUnspecified ChangeType = 0
	// ChangeTypeAdded means the field is present in the target version but
	// absent in the base version.
	ChangeTypeAdded ChangeType = 1
	// ChangeTypeRemoved means the field is present in the base version but
	// absent in the target version.
	ChangeTypeRemoved ChangeType = 2
	// ChangeTypeModified means the field is present in both versions with a
	// different value.
	ChangeTypeModified ChangeType = 3
)

// String returns a human-readable name for the change type.
func (c ChangeType) String() string {
	switch c {
	case ChangeTypeAdded:
		return "added"
	case ChangeTypeRemoved:
		return "removed"
	case ChangeTypeModified:
		return "modified"
	default:
		return "unspecified"
	}
}

// FieldDiff describes a single field that differs between two config versions.
type FieldDiff struct {
	// FieldPath is the dot-separated field path (e.g. "payments.fee").
	FieldPath string
	// ChangeType is how the field changed.
	ChangeType ChangeType
	// OldValue is the value at the base version. Empty when ChangeType is
	// ChangeTypeAdded.
	OldValue string
	// NewValue is the value at the target version. Empty when ChangeType is
	// ChangeTypeRemoved.
	NewValue string
}
