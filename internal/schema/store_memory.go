package schema

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"sync"
	"time"

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
}

// NewMemoryStore creates a new in-memory schema store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		schemas:        make(map[string]domain.Schema),
		schemaVersions: make(map[string]domain.SchemaVersion),
		schemaFields:   make(map[string][]domain.SchemaField),
		tenants:        make(map[string]domain.Tenant),
		fieldLocks:     make(map[string][]domain.TenantFieldLock),
	}
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

	// Check name uniqueness.
	for _, s := range m.schemas {
		if s.Name == arg.Name {
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
	if !ok {
		return domain.Schema{}, domain.ErrNotFound
	}
	return s, nil
}

func (m *MemoryStore) GetSchemaByName(_ context.Context, name string) (domain.Schema, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.schemas {
		if s.Name == name {
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
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	return paginate(all, int(arg.Offset), int(arg.Limit)), nil
}

func (m *MemoryStore) DeleteSchema(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schemas[id]; !ok {
		return domain.ErrNotFound
	}

	// Cascade: delete versions + their fields.
	for vid, sv := range m.schemaVersions {
		if sv.SchemaID == id {
			delete(m.schemaFields, vid)
			delete(m.schemaVersions, vid)
		}
	}

	// Cascade: delete tenants + their locks.
	for tid, t := range m.tenants {
		if t.SchemaID == id {
			delete(m.fieldLocks, tid)
			delete(m.tenants, tid)
		}
	}

	delete(m.schemas, id)
	return nil
}

// --- Schema Versions ---

func (m *MemoryStore) CreateSchemaVersion(_ context.Context, arg CreateSchemaVersionParams) (domain.SchemaVersion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schemas[arg.SchemaID]; !ok {
		return domain.SchemaVersion{}, domain.ErrNotFound
	}

	sv := domain.SchemaVersion{
		ID:            m.nextID(),
		SchemaID:      arg.SchemaID,
		Version:       arg.Version,
		ParentVersion: arg.ParentVersion,
		Description:   arg.Description,
		Checksum:      arg.Checksum,
		Published:     false,
		CreatedAt:     time.Now(),
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
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

func (m *MemoryStore) ListTenants(_ context.Context, arg ListTenantsParams) ([]domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]domain.Tenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		all = append(all, t)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	return paginate(all, int(arg.Offset), int(arg.Limit)), nil
}

func (m *MemoryStore) ListTenantsBySchema(_ context.Context, arg ListTenantsBySchemaParams) ([]domain.Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []domain.Tenant
	for _, t := range m.tenants {
		if t.SchemaID == arg.SchemaID {
			filtered = append(filtered, t)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })

	return paginate(filtered, int(arg.Offset), int(arg.Limit)), nil
}

func (m *MemoryStore) UpdateTenantName(_ context.Context, arg UpdateTenantNameParams) (domain.Tenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.tenants[arg.ID]
	if !ok {
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
	if !ok {
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

	if _, ok := m.tenants[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m.fieldLocks, id)
	delete(m.tenants, id)
	return nil
}

// --- Field Locks ---

func (m *MemoryStore) CreateFieldLock(_ context.Context, arg CreateFieldLockParams) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tenants[arg.TenantID]; !ok {
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
