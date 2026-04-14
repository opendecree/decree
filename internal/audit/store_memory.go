package audit

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/opendecree/decree/internal/storage/domain"
)

// MemoryStore implements Store using in-memory slices.
// Suitable for testing and single-process deployments.
type MemoryStore struct {
	mu         sync.RWMutex
	writeLogs  []domain.AuditWriteLog
	usageStats []domain.UsageStat
	idCounter  int64
}

// NewMemoryStore creates a new in-memory audit store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) nextID() string {
	s.idCounter++
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", s.idCounter)
}

// AddWriteLog inserts a write log entry for testing purposes.
func (s *MemoryStore) AddWriteLog(entry domain.AuditWriteLog) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entry.ID == "" {
		entry.ID = s.nextID()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.writeLogs = append(s.writeLogs, entry)
}

func (s *MemoryStore) QueryAuditWriteLog(_ context.Context, arg QueryWriteLogParams) ([]domain.AuditWriteLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []domain.AuditWriteLog
	for _, e := range s.writeLogs {
		if arg.TenantID != "" && e.TenantID != arg.TenantID {
			continue
		}
		if arg.Actor != "" && e.Actor != arg.Actor {
			continue
		}
		if arg.FieldPath != "" && (e.FieldPath == nil || *e.FieldPath != arg.FieldPath) {
			continue
		}
		if arg.StartTime != nil && e.CreatedAt.Before(*arg.StartTime) {
			continue
		}
		if arg.EndTime != nil && e.CreatedAt.After(*arg.EndTime) {
			continue
		}
		filtered = append(filtered, e)
	}

	// Sort by CreatedAt DESC.
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	// Apply offset and limit.
	if int(arg.Offset) >= len(filtered) {
		return nil, nil
	}
	filtered = filtered[arg.Offset:]
	if arg.Limit > 0 && int(arg.Limit) < len(filtered) {
		filtered = filtered[:arg.Limit]
	}

	return filtered, nil
}

func (s *MemoryStore) GetFieldUsage(_ context.Context, arg GetFieldUsageParams) ([]domain.UsageStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []domain.UsageStat
	for _, st := range s.usageStats {
		if st.TenantID != arg.TenantID {
			continue
		}
		if st.FieldPath != arg.FieldPath {
			continue
		}
		if arg.StartTime != nil && st.PeriodStart.Before(*arg.StartTime) {
			continue
		}
		if arg.EndTime != nil && st.PeriodStart.After(*arg.EndTime) {
			continue
		}
		result = append(result, st)
	}
	return result, nil
}

func (s *MemoryStore) GetTenantUsage(_ context.Context, arg GetTenantUsageParams) ([]domain.TenantUsageRow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Aggregate by field path.
	type agg struct {
		readCount  int64
		lastReadAt *time.Time
	}
	byField := make(map[string]*agg)

	for _, st := range s.usageStats {
		if st.TenantID != arg.TenantID {
			continue
		}
		if arg.StartTime != nil && st.PeriodStart.Before(*arg.StartTime) {
			continue
		}
		if arg.EndTime != nil && st.PeriodStart.After(*arg.EndTime) {
			continue
		}
		a, ok := byField[st.FieldPath]
		if !ok {
			a = &agg{}
			byField[st.FieldPath] = a
		}
		a.readCount += st.ReadCount
		if st.LastReadAt != nil {
			if a.lastReadAt == nil || st.LastReadAt.After(*a.lastReadAt) {
				t := *st.LastReadAt
				a.lastReadAt = &t
			}
		}
	}

	result := make([]domain.TenantUsageRow, 0, len(byField))
	for fp, a := range byField {
		result = append(result, domain.TenantUsageRow{
			FieldPath:  fp,
			ReadCount:  a.readCount,
			LastReadAt: a.lastReadAt,
		})
	}

	// Sort by field path for deterministic output.
	sort.Slice(result, func(i, j int) bool {
		return result[i].FieldPath < result[j].FieldPath
	})

	return result, nil
}

func (s *MemoryStore) GetUnusedFields(_ context.Context, _ GetUnusedFieldsParams) ([]string, error) {
	// This method requires a join with schema fields which the audit store
	// does not have. Return an empty slice; meaningful results require a
	// real database or a helper that provides the set of known fields.
	return nil, nil
}

func (s *MemoryStore) UpsertUsageStats(_ context.Context, arg UpsertUsageStatsParams) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lastReadAt := arg.LastReadAt

	for i, st := range s.usageStats {
		if st.TenantID == arg.TenantID && st.FieldPath == arg.FieldPath && st.PeriodStart.Equal(arg.PeriodStart) {
			s.usageStats[i].ReadCount += arg.ReadCount
			s.usageStats[i].LastReadBy = arg.LastReadBy
			s.usageStats[i].LastReadAt = &lastReadAt
			return nil
		}
	}

	s.usageStats = append(s.usageStats, domain.UsageStat{
		TenantID:    arg.TenantID,
		FieldPath:   arg.FieldPath,
		PeriodStart: arg.PeriodStart,
		ReadCount:   arg.ReadCount,
		LastReadBy:  arg.LastReadBy,
		LastReadAt:  &lastReadAt,
	})
	return nil
}
