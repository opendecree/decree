package adminclient

import "time"

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
	Type         string
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
	Got      string // stored entry_hash
	Want     string // recomputed entry_hash
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
