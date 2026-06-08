package schema

import (
	"context"
	"crypto/rand"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/storage/domain"
)

// MemoryStore implements Store using in-memory maps.
// Suitable for testing and development.
type MemoryStore struct {
	mu sync.RWMutex

	schemas        map[string]domain.Schema            // id → Schema
	schemaVersions map[string]domain.SchemaVersion     // id → SchemaVersion
	schemaFields   map[string][]domain.SchemaField     // schemaVersionID → []SchemaField
	tenants        map[string]domain.Tenant            // id → Tenant
	fieldLocks     map[string][]domain.TenantFieldLock // tenantID → []TenantFieldLock
	configVersions map[string]domain.ConfigVersion     // configVersionID → ConfigVersion
	configValues   map[string][]domain.ConfigValue     // configVersionID → []ConfigValue
	auditLog       []domain.AuditWriteLog
}

// NewMemoryStore creates a new in-memory schema store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schemas:        make(map[string]domain.Schema),
		schemaVersions: make(map[string]domain.SchemaVersion),
		schemaFields:   make(map[string][]domain.SchemaField),
		tenants:        make(map[string]domain.Tenant),
		fieldLocks:     make(map[string][]domain.TenantFieldLock),
		configVersions: make(map[string]domain.ConfigVersion),
		configValues:   make(map[string][]domain.ConfigValue),
	}
}

func (m *MemoryStore) RunInTx(_ context.Context, fn func(Store) error) error {
	// Clone current state into a temporary store.
	m.mu.Lock()
	tmp := m.clone()
	m.mu.Unlock()

	if err := fn(tmp); err != nil {
		return err // discard tmp — rollback
	}

	// Commit: atomically swap tmp's state into m.
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mergeFrom(tmp)
	return nil
}

// clone returns a shallow copy of m with all map and slice fields deep-copied.
// Caller must hold m.mu before calling.
func (m *MemoryStore) clone() *MemoryStore {
	tmp := &MemoryStore{
		schemas:        make(map[string]domain.Schema, len(m.schemas)),
		schemaVersions: make(map[string]domain.SchemaVersion, len(m.schemaVersions)),
		schemaFields:   make(map[string][]domain.SchemaField, len(m.schemaFields)),
		tenants:        make(map[string]domain.Tenant, len(m.tenants)),
		fieldLocks:     make(map[string][]domain.TenantFieldLock, len(m.fieldLocks)),
		configVersions: make(map[string]domain.ConfigVersion, len(m.configVersions)),
		configValues:   make(map[string][]domain.ConfigValue, len(m.configValues)),
		auditLog:       make([]domain.AuditWriteLog, len(m.auditLog)),
	}
	for k, v := range m.schemas {
		tmp.schemas[k] = v
	}
	for k, v := range m.schemaVersions {
		tmp.schemaVersions[k] = v
	}
	for k, fs := range m.schemaFields {
		cp := make([]domain.SchemaField, len(fs))
		copy(cp, fs)
		tmp.schemaFields[k] = cp
	}
	for k, v := range m.tenants {
		tmp.tenants[k] = v
	}
	for k, ls := range m.fieldLocks {
		cp := make([]domain.TenantFieldLock, len(ls))
		copy(cp, ls)
		tmp.fieldLocks[k] = cp
	}
	for k, v := range m.configVersions {
		tmp.configVersions[k] = v
	}
	for k, vs := range m.configValues {
		cp := make([]domain.ConfigValue, len(vs))
		copy(cp, vs)
		tmp.configValues[k] = cp
	}
	copy(tmp.auditLog, m.auditLog)
	return tmp
}

// mergeFrom copies all state from src into m.
// Caller must hold m.mu before calling.
func (m *MemoryStore) mergeFrom(src *MemoryStore) {
	src.mu.Lock()
	defer src.mu.Unlock()
	m.schemas = src.schemas
	m.schemaVersions = src.schemaVersions
	m.schemaFields = src.schemaFields
	m.tenants = src.tenants
	m.fieldLocks = src.fieldLocks
	m.configVersions = src.configVersions
	m.configValues = src.configValues
	m.auditLog = src.auditLog
}

func (m *MemoryStore) InsertAuditWriteLog(_ context.Context, arg InsertAuditWriteLogParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.auditLog = append(m.auditLog, domain.AuditWriteLog{
		TenantID:   arg.TenantID,
		Actor:      arg.Actor,
		Action:     arg.Action,
		ObjectKind: arg.ObjectKind,
		FieldPath:  arg.FieldPath,
		OldValue:   arg.OldValue,
		NewValue:   arg.NewValue,
		Metadata:   arg.Metadata,
		CreatedAt:  now,
	})
	return nil
}

func (m *MemoryStore) nextID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// --- Schema CRUD ---

func (m *MemoryStore) CreateSchema(_ context.Context, arg CreateSchemaParams) (domain.Schema, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check name uniqueness among non-deleted schemas.
	for _, s := range m.schemas {
		if s.Name == arg.Name && s.DeletedAt == nil {
			return domain.Schema{}, fmt.Errorf("schema with name %q already exists", arg.Name)
		}
	}

	now := time.Now()
	s := domain.Schema{
		ID:          m.nextID(),
		Name:        arg.Name,
		Description: arg.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.schemas[s.ID] = s
	return s, nil
}

func (m *MemoryStore) GetSchemaByID(_ context.Context, id string) (domain.Schema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.schemas[id]
	if !ok || s.DeletedAt != nil {
		return domain.Schema{}, domain.ErrNotFound
	}
	return s, nil
}

func (m *MemoryStore) GetSchemaByName(_ context.Context, name string) (domain.Schema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.schemas {
		if s.Name == name && s.DeletedAt == nil {
			return s, nil
		}
	}
	return domain.Schema{}, domain.ErrNotFound
}

func (m *MemoryStore) ListSchemas(_ context.Context, arg ListSchemasParams) ([]domain.Schema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]domain.Schema, 0, len(m.schemas))
	for _, s := range m.schemas {
		if s.DeletedAt == nil {
			all = append(all, s)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	return paginate(all, int(arg.Offset), int(arg.Limit)), nil
}

func (m *MemoryStore) DeleteSchema(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.schemas[id]
	if !ok || s.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now()
	s.DeletedAt = &now
	m.schemas[id] = s
	return nil
}

// --- Schema Versions ---

func (m *MemoryStore) CreateSchemaVersion(_ context.Context, arg CreateSchemaVersionParams) (domain.SchemaVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schemas[arg.SchemaID]; !ok {
		return domain.SchemaVersion{}, domain.ErrNotFound
	}

	depReq := arg.DependentRequired
	if len(depReq) == 0 {
		depReq = []byte("[]")
	}
	validations := arg.Validations
	if len(validations) == 0 {
		validations = []byte("[]")
	}
	sv := domain.SchemaVersion{
		ID:                m.nextID(),
		SchemaID:          arg.SchemaID,
		Version:           arg.Version,
		ParentVersion:     arg.ParentVersion,
		Description:       arg.Description,
		Checksum:          arg.Checksum,
		Published:         false,
		DependentRequired: depReq,
		Validations:       validations,
		CreatedAt:         time.Now(),
	}
	m.schemaVersions[sv.ID] = sv
	return sv, nil
}

func (m *MemoryStore) GetSchemaVersion(_ context.Context, arg GetSchemaVersionParams) (domain.SchemaVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, sv := range m.schemaVersions {
		if sv.SchemaID == arg.SchemaID && sv.Version == arg.Version {
			return sv, nil
		}
	}
	return domain.SchemaVersion{}, domain.ErrNotFound
}

func (m *MemoryStore) GetLatestSchemaVersion(_ context.Context, schemaID string) (domain.SchemaVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *domain.SchemaVersion
	for _, sv := range m.schemaVersions {
		if sv.SchemaID == schemaID {
			if latest == nil || sv.Version > latest.Version {
				cp := sv
				latest = &cp
			}
		}
	}
	if latest == nil {
		return domain.SchemaVersion{}, domain.ErrNotFound
	}
	return *latest, nil
}

func (m *MemoryStore) PublishSchemaVersion(_ context.Context, arg PublishSchemaVersionParams) (domain.SchemaVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, sv := range m.schemaVersions {
		if sv.SchemaID == arg.SchemaID && sv.Version == arg.Version {
			sv.Published = true
			m.schemaVersions[id] = sv
			return sv, nil
		}
	}
	return domain.SchemaVersion{}, domain.ErrNotFound
}

// --- Schema Fields ---

func (m *MemoryStore) CreateSchemaField(_ context.Context, arg CreateSchemaFieldParams) (domain.SchemaField, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schemaVersions[arg.SchemaVersionID]; !ok {
		return domain.SchemaField{}, domain.ErrNotFound
	}

	f := domain.SchemaField{
		ID:              m.nextID(),
		SchemaVersionID: arg.SchemaVersionID,
		Path:            arg.Path,
		FieldType:       arg.FieldType,
		Constraints:     arg.Constraints,
		Nullable:        arg.Nullable,
		Deprecated:      arg.Deprecated,
		RedirectTo:      arg.RedirectTo,
		DefaultValue:    arg.DefaultValue,
		Description:     arg.Description,
		Title:           arg.Title,
		Example:         arg.Example,
		Examples:        arg.Examples,
		ExternalDocs:    arg.ExternalDocs,
		Tags:            arg.Tags,
		Format:          arg.Format,
		ReadOnly:        arg.ReadOnly,
		WriteOnce:       arg.WriteOnce,
		Sensitive:       arg.Sensitive,
	}
	m.schemaFields[arg.SchemaVersionID] = append(m.schemaFields[arg.SchemaVersionID], f)
	return f, nil
}

func (m *MemoryStore) GetSchemaFields(_ context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fields := m.schemaFields[schemaVersionID]
	result := make([]domain.SchemaField, len(fields))
	copy(result, fields)
	return result, nil
}

func (m *MemoryStore) BulkCreateSchemaFields(ctx context.Context, args []CreateSchemaFieldParams) ([]domain.SchemaField, error) {
	result := make([]domain.SchemaField, 0, len(args))
	for _, a := range args {
		f, err := m.CreateSchemaField(ctx, a)
		if err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}

func (m *MemoryStore) GetSchemaFieldsByVersionIDs(_ context.Context, versionIDs []string) ([]domain.SchemaField, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idSet := make(map[string]struct{}, len(versionIDs))
	for _, id := range versionIDs {
		idSet[id] = struct{}{}
	}
	var result []domain.SchemaField
	for vID, fields := range m.schemaFields {
		if _, ok := idSet[vID]; ok {
			result = append(result, fields...)
		}
	}
	return result, nil
}

func (m *MemoryStore) GetLatestSchemaVersionsBatch(_ context.Context, schemaIDs []string) ([]domain.SchemaVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idSet := make(map[string]struct{}, len(schemaIDs))
	for _, id := range schemaIDs {
		idSet[id] = struct{}{}
	}
	latest := make(map[string]*domain.SchemaVersion)
	for _, sv := range m.schemaVersions {
		if _, ok := idSet[sv.SchemaID]; !ok {
			continue
		}
		cur := latest[sv.SchemaID]
		if cur == nil || sv.Version > cur.Version {
			cp := sv
			latest[sv.SchemaID] = &cp
		}
	}
	result := make([]domain.SchemaVersion, 0, len(latest))
	for _, sv := range latest {
		result = append(result, *sv)
	}
	return result, nil
}

func (m *MemoryStore) DeleteSchemaField(_ context.Context, arg DeleteSchemaFieldParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fields, ok := m.schemaFields[arg.SchemaVersionID]
	if !ok {
		return domain.ErrNotFound
	}

	for i, f := range fields {
		if f.Path == arg.Path {
			m.schemaFields[arg.SchemaVersionID] = append(fields[:i], fields[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

// --- Tenants ---

func (m *MemoryStore) CreateTenant(_ context.Context, arg CreateTenantParams) (domain.Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	t := domain.Tenant{
		ID:            m.nextID(),
		Name:          arg.Name,
		SchemaID:      arg.SchemaID,
		SchemaVersion: arg.SchemaVersion,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	m.tenants[t.ID] = t
	return t, nil
}

func (m *MemoryStore) GetTenantByID(_ context.Context, id string) (domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.tenants[id]
	if !ok || t.DeletedAt != nil {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

func (m *MemoryStore) GetTenantByName(_ context.Context, name string) (domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tenants {
		if t.Name == name && t.DeletedAt == nil {
			return t, nil
		}
	}
	return domain.Tenant{}, domain.ErrNotFound
}

func (m *MemoryStore) GetTenantsByNames(_ context.Context, names []string) ([]domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nameSet := make(map[string]struct{}, len(names))
	for _, n := range names {
		nameSet[n] = struct{}{}
	}
	var result []domain.Tenant
	for _, t := range m.tenants {
		if t.DeletedAt != nil {
			continue
		}
		if _, ok := nameSet[t.Name]; ok {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *MemoryStore) ListTenants(_ context.Context, arg ListTenantsParams) ([]domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]domain.Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		if t.DeletedAt != nil {
			continue
		}
		if arg.AllowedTenantIDs != nil && !slices.Contains(arg.AllowedTenantIDs, t.ID) {
			continue
		}
		all = append(all, t)
	}
	return paginateTenants(all, arg.Cursor, int(arg.Offset), int(arg.Limit)), nil
}

func (m *MemoryStore) ListTenantsBySchema(_ context.Context, arg ListTenantsBySchemaParams) ([]domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []domain.Tenant
	for _, t := range m.tenants {
		if t.DeletedAt != nil || t.SchemaID != arg.SchemaID {
			continue
		}
		if arg.AllowedTenantIDs != nil && !slices.Contains(arg.AllowedTenantIDs, t.ID) {
			continue
		}
		filtered = append(filtered, t)
	}
	return paginateTenants(filtered, arg.Cursor, int(arg.Offset), int(arg.Limit)), nil
}

func (m *MemoryStore) UpdateTenantName(_ context.Context, arg UpdateTenantNameParams) (domain.Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[arg.ID]
	if !ok || t.DeletedAt != nil {
		return domain.Tenant{}, domain.ErrNotFound
	}
	t.Name = arg.Name
	t.UpdatedAt = time.Now()
	m.tenants[arg.ID] = t
	return t, nil
}

func (m *MemoryStore) UpdateTenantSchemaVersion(_ context.Context, arg UpdateTenantSchemaVersionParams) (domain.Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[arg.ID]
	if !ok || t.DeletedAt != nil {
		return domain.Tenant{}, domain.ErrNotFound
	}
	t.SchemaVersion = arg.SchemaVersion
	t.UpdatedAt = time.Now()
	m.tenants[arg.ID] = t
	return t, nil
}

func (m *MemoryStore) DeleteTenant(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[id]
	if !ok || t.DeletedAt != nil {
		return domain.ErrNotFound
	}
	now := time.Now()
	t.DeletedAt = &now
	m.tenants[id] = t
	return nil
}

// --- Field Locks ---

func (m *MemoryStore) CreateFieldLock(_ context.Context, arg CreateFieldLockParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[arg.TenantID]
	if !ok || t.DeletedAt != nil {
		return domain.ErrNotFound
	}

	lock := domain.TenantFieldLock{
		TenantID:     arg.TenantID,
		FieldPath:    arg.FieldPath,
		LockedValues: arg.LockedValues,
	}
	m.fieldLocks[arg.TenantID] = append(m.fieldLocks[arg.TenantID], lock)
	return nil
}

func (m *MemoryStore) DeleteFieldLock(_ context.Context, arg DeleteFieldLockParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	locks, ok := m.fieldLocks[arg.TenantID]
	if !ok {
		return domain.ErrNotFound
	}

	for i, l := range locks {
		if l.FieldPath == arg.FieldPath {
			m.fieldLocks[arg.TenantID] = append(locks[:i], locks[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound
}

func (m *MemoryStore) GetFieldLocks(_ context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locks := m.fieldLocks[tenantID]
	result := make([]domain.TenantFieldLock, len(locks))
	copy(result, locks)
	return result, nil
}

func (m *MemoryStore) ListFieldLocks(_ context.Context, tenantID string, arg ListFieldLocksParams) ([]domain.TenantFieldLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locks := m.fieldLocks[tenantID]
	all := make([]domain.TenantFieldLock, len(locks))
	copy(all, locks)
	return paginate(all, int(arg.Offset), int(arg.Limit)), nil
}

// --- Tenant config seeding ---

// SeedTenantConfig writes the tenant's version-1 config from schema defaults,
// mirroring the PG store: one config version plus one value per default. A nil
// or empty Values map is a no-op so no empty version 1 is created.
func (m *MemoryStore) SeedTenantConfig(_ context.Context, arg SeedTenantConfigParams) error {
	if len(arg.Values) == 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	cvID := m.nextID()
	desc := "Initial config from schema defaults"
	m.configVersions[cvID] = domain.ConfigVersion{
		ID:          cvID,
		TenantID:    arg.TenantID,
		Version:     1,
		Description: &desc,
		CreatedBy:   arg.Actor,
		CreatedAt:   time.Now(),
	}
	values := make([]domain.ConfigValue, 0, len(arg.Values))
	for path, v := range arg.Values {
		value := v.Value
		checksum := v.Checksum
		values = append(values, domain.ConfigValue{
			ConfigVersionID: cvID,
			FieldPath:       path,
			Value:           &value,
			Checksum:        &checksum,
		})
	}
	m.configValues[cvID] = values
	return nil
}

// ConfigVersionsForTenant returns the config versions seeded for a tenant.
// Test helper: the schema store does not otherwise expose config reads.
func (m *MemoryStore) ConfigVersionsForTenant(tenantID string) []domain.ConfigVersion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []domain.ConfigVersion
	for _, cv := range m.configVersions {
		if cv.TenantID == tenantID {
			out = append(out, cv)
		}
	}
	return out
}

// ConfigValuesForTenant returns field path -> value for the tenant's seeded
// config, collapsing across versions (only version 1 is ever seeded here).
// Test helper.
func (m *MemoryStore) ConfigValuesForTenant(tenantID string) map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]string)
	for _, cv := range m.configVersions {
		if cv.TenantID != tenantID {
			continue
		}
		for _, v := range m.configValues[cv.ID] {
			if v.Value != nil {
				out[v.FieldPath] = *v.Value
			}
		}
	}
	return out
}

// paginate applies offset and limit to a sorted slice.
func paginate[T any](items []T, offset, limit int) []T {
	if offset >= len(items) {
		return nil
	}
	items = items[offset:]
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

// paginateTenants sorts tenants by (created_at DESC, id DESC) and applies
// either keyset cursor or offset pagination, then enforces the limit.
func paginateTenants(tenants []domain.Tenant, cursor *pagination.PageCursor, offset, limit int) []domain.Tenant {
	sort.Slice(tenants, func(i, j int) bool {
		if tenants[i].CreatedAt.Equal(tenants[j].CreatedAt) {
			return tenants[i].ID > tenants[j].ID
		}
		return tenants[i].CreatedAt.After(tenants[j].CreatedAt)
	})

	if cursor != nil {
		var after []domain.Tenant
		for _, t := range tenants {
			if t.CreatedAt.Before(cursor.Time) ||
				(t.CreatedAt.Equal(cursor.Time) && t.ID < cursor.ID) {
				after = append(after, t)
			}
		}
		tenants = after
	} else {
		tenants = paginate(tenants, offset, limit)
		return tenants
	}

	if limit > 0 && limit < len(tenants) {
		tenants = tenants[:limit]
	}
	return tenants
}
