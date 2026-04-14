package config

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/opendecree/decree/internal/storage/domain"
)

// MemoryStore implements Store using in-memory maps.
// Safe for concurrent use. Transactions are serialized via mutex.
type MemoryStore struct {
	mu sync.RWMutex

	idCounter int64

	configVersions map[string]domain.ConfigVersion     // id → version
	configValues   map[string][]domain.ConfigValue     // configVersionID → values
	tenants        map[string]domain.Tenant            // id → tenant
	schemaVersions map[string]domain.SchemaVersion     // compositeKey → version
	schemaFields   map[string][]domain.SchemaField     // schemaVersionID → fields
	fieldLocks     map[string][]domain.TenantFieldLock // tenantID → locks
	auditLog       []auditEntry
}

type auditEntry struct {
	params    InsertAuditWriteLogParams
	createdAt time.Time
}

// NewMemoryStore creates a new in-memory config store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		configVersions: make(map[string]domain.ConfigVersion),
		configValues:   make(map[string][]domain.ConfigValue),
		tenants:        make(map[string]domain.Tenant),
		schemaVersions: make(map[string]domain.SchemaVersion),
		schemaFields:   make(map[string][]domain.SchemaField),
		fieldLocks:     make(map[string][]domain.TenantFieldLock),
	}
}

func (s *MemoryStore) nextID() string {
	s.idCounter++
	return fmt.Sprintf("mem-%08d", s.idCounter)
}

func (s *MemoryStore) RunInTx(_ context.Context, fn func(Store) error) error {
	// Serialized execution — the mutex is already held for writes.
	return fn(s)
}

// --- Config versions ---

func (s *MemoryStore) CreateConfigVersion(_ context.Context, arg CreateConfigVersionParams) (domain.ConfigVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v := domain.ConfigVersion{
		ID:          s.nextID(),
		TenantID:    arg.TenantID,
		Version:     arg.Version,
		Description: arg.Description,
		CreatedBy:   arg.CreatedBy,
		CreatedAt:   time.Now(),
	}
	s.configVersions[v.ID] = v
	return v, nil
}

func (s *MemoryStore) GetConfigVersion(_ context.Context, arg GetConfigVersionParams) (domain.ConfigVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, v := range s.configVersions {
		if v.TenantID == arg.TenantID && v.Version == arg.Version {
			return v, nil
		}
	}
	return domain.ConfigVersion{}, domain.ErrNotFound
}

func (s *MemoryStore) GetLatestConfigVersion(_ context.Context, tenantID string) (domain.ConfigVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest domain.ConfigVersion
	found := false
	for _, v := range s.configVersions {
		if v.TenantID == tenantID && (!found || v.Version > latest.Version) {
			latest = v
			found = true
		}
	}
	if !found {
		return domain.ConfigVersion{}, domain.ErrNotFound
	}
	return latest, nil
}

func (s *MemoryStore) ListConfigVersions(_ context.Context, arg ListConfigVersionsParams) ([]domain.ConfigVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []domain.ConfigVersion
	for _, v := range s.configVersions {
		if v.TenantID == arg.TenantID {
			result = append(result, v)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version > result[j].Version })

	// Pagination.
	if int(arg.Offset) < len(result) {
		result = result[arg.Offset:]
	} else {
		result = nil
	}
	if arg.Limit > 0 && int(arg.Limit) < len(result) {
		result = result[:arg.Limit]
	}
	return result, nil
}

// --- Config values ---

func (s *MemoryStore) SetConfigValue(_ context.Context, arg SetConfigValueParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.configValues[arg.ConfigVersionID] = append(s.configValues[arg.ConfigVersionID], domain.ConfigValue{
		ConfigVersionID: arg.ConfigVersionID,
		FieldPath:       arg.FieldPath,
		Value:           arg.Value,
		Checksum:        arg.Checksum,
		Description:     arg.Description,
	})
	return nil
}

func (s *MemoryStore) GetConfigValues(_ context.Context, configVersionID string) ([]domain.ConfigValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.configValues[configVersionID], nil
}

func (s *MemoryStore) GetConfigValueAtVersion(_ context.Context, arg GetConfigValueAtVersionParams) (GetConfigValueAtVersionRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find the latest version <= arg.Version for this tenant+field.
	var bestVersion int32
	var bestValue *domain.ConfigValue
	for _, cv := range s.configVersions {
		if cv.TenantID == arg.TenantID && cv.Version <= arg.Version {
			for i, val := range s.configValues[cv.ID] {
				if val.FieldPath == arg.FieldPath && cv.Version > bestVersion {
					bestVersion = cv.Version
					bestValue = &s.configValues[cv.ID][i]
				}
			}
		}
	}
	if bestValue == nil {
		return GetConfigValueAtVersionRow{}, domain.ErrNotFound
	}
	return GetConfigValueAtVersionRow{
		FieldPath:   bestValue.FieldPath,
		Value:       bestValue.Value,
		Checksum:    bestValue.Checksum,
		Description: bestValue.Description,
	}, nil
}

func (s *MemoryStore) GetFullConfigAtVersion(_ context.Context, arg GetFullConfigAtVersionParams) ([]GetFullConfigAtVersionRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect latest value for each field path up to the given version.
	latest := make(map[string]GetFullConfigAtVersionRow)
	latestVer := make(map[string]int32)

	for _, cv := range s.configVersions {
		if cv.TenantID == arg.TenantID && cv.Version <= arg.Version {
			for _, val := range s.configValues[cv.ID] {
				if cv.Version > latestVer[val.FieldPath] {
					latestVer[val.FieldPath] = cv.Version
					latest[val.FieldPath] = GetFullConfigAtVersionRow{
						FieldPath:   val.FieldPath,
						Value:       val.Value,
						Checksum:    val.Checksum,
						Description: val.Description,
					}
				}
			}
		}
	}

	result := make([]GetFullConfigAtVersionRow, 0, len(latest))
	for _, row := range latest {
		result = append(result, row)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].FieldPath < result[j].FieldPath })
	return result, nil
}

// --- Tenant/schema lookups (for validation) ---

func (s *MemoryStore) GetTenantByID(_ context.Context, id string) (domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.tenants[id]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

// SetTenant is a test helper to seed tenant data.
func (s *MemoryStore) SetTenant(t domain.Tenant) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tenants[t.ID] = t
}

func (s *MemoryStore) GetSchemaFields(_ context.Context, schemaVersionID string) ([]domain.SchemaField, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fields := s.schemaFields[schemaVersionID]
	if fields == nil {
		return []domain.SchemaField{}, nil
	}
	return fields, nil
}

// SetSchemaFields is a test helper to seed schema field data.
func (s *MemoryStore) SetSchemaFields(schemaVersionID string, fields []domain.SchemaField) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.schemaFields[schemaVersionID] = fields
}

func (s *MemoryStore) GetSchemaVersion(_ context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := fmt.Sprintf("%s:%d", arg.SchemaID, arg.Version)
	sv, ok := s.schemaVersions[key]
	if !ok {
		return domain.SchemaVersion{}, domain.ErrNotFound
	}
	return sv, nil
}

// SetSchemaVersion is a test helper to seed schema version data.
func (s *MemoryStore) SetSchemaVersion(sv domain.SchemaVersion) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("%s:%d", sv.SchemaID, sv.Version)
	s.schemaVersions[key] = sv
}

// --- Field locks ---

func (s *MemoryStore) GetFieldLocks(_ context.Context, tenantID string) ([]domain.TenantFieldLock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.fieldLocks[tenantID], nil
}

// --- Audit ---

func (s *MemoryStore) InsertAuditWriteLog(_ context.Context, arg InsertAuditWriteLogParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLog = append(s.auditLog, auditEntry{params: arg, createdAt: time.Now()})
	return nil
}
