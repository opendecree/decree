package authz

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/storage/domain"
)

// FieldLockStore is the minimal interface FieldLockGuard needs.
type FieldLockStore interface {
	GetFieldLocks(ctx context.Context, tenantID string) ([]domain.TenantFieldLock, error)
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
	locks, err := g.store.GetFieldLocks(ctx, r.TenantID)
	if err != nil {
		return status.Error(codes.Internal, "failed to check field locks")
	}
	for _, lock := range locks {
		if lock.FieldPath == r.FieldPath {
			return status.Errorf(codes.PermissionDenied, "field %s is locked", r.FieldPath)
		}
	}
	return nil
}
