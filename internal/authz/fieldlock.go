package authz

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/storage/domain"
)

// FieldLockStore is the minimal interface FieldLockGuard needs.
type FieldLockStore interface {
	GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error)
}

// fieldLockCacheKey is the private context key for per-request lock memoization.
type fieldLockCacheKey struct{}

// WithFieldLockCache returns a context that carries a pre-fetched field lock
// list. FieldLockGuard.Check reads from this cache instead of the DB, so a
// bulk operation (e.g. SetFields) can fetch locks once and avoid N round-trips.
func WithFieldLockCache(ctx context.Context, locks []domain.TenantFieldLock) context.Context {
	return context.WithValue(ctx, fieldLockCacheKey{}, locks)
}

// FieldLockGuard blocks write operations on locked fields.
// No-ops for reads, superadmins, or when FieldPath is empty.
type FieldLockGuard struct {
	store FieldLockStore
}

// NewFieldLockGuard creates a FieldLockGuard backed by the given store.
func NewFieldLockGuard(store FieldLockStore) FieldLockGuard {
	return FieldLockGuard{store: store}
}

func (g FieldLockGuard) Check(ctx context.Context, action Action, r Resource) error {
	if action != ActionWrite || r.FieldPath == "" {
		return nil
	}
	if auth.IsSuperAdmin(ctx) {
		return nil
	}
	var locks []domain.TenantFieldLock
	if cached, ok := ctx.Value(fieldLockCacheKey{}).([]domain.TenantFieldLock); ok {
		locks = cached
	} else {
		var err error
		locks, err = g.store.GetFieldLocks(ctx, r.TenantID)
		if err != nil {
			return status.Error(codes.Internal, "failed to check field locks")
		}
	}
	for _, lock := range locks {
		if lock.FieldPath != r.FieldPath {
			continue
		}
		// A nil/empty raw LockedValues locks the whole field (every write blocked).
		if len(lock.LockedValues) == 0 {
			return status.Errorf(codes.FailedPrecondition, "field %s is locked", r.FieldPath)
		}
		// LockedValues is json.Marshal([]string): only the listed enum values are
		// locked. Block only when the attempted value is one of them.
		var values []string
		if err := json.Unmarshal(lock.LockedValues, &values); err != nil {
			// Fail closed: an unparseable lock blocks the write rather than
			// silently allowing it.
			return status.Errorf(codes.FailedPrecondition, "field %s is locked", r.FieldPath)
		}
		// A marshaled empty list ("[]") carries no values to scope on, so it also
		// locks the whole field.
		if len(values) == 0 {
			return status.Errorf(codes.FailedPrecondition, "field %s is locked", r.FieldPath)
		}
		for _, v := range values {
			if v == r.Value {
				return status.Errorf(codes.FailedPrecondition, "value %q for field %s is locked", r.Value, r.FieldPath)
			}
		}
		// Attempted value is not in this lock's set; keep scanning other locks.
	}
	return nil
}
