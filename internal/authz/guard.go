package authz

import "context"

// Action describes what the caller intends to do.
type Action string

const (
	// ActionRead — any authenticated caller may read.
	ActionRead Action = "read"
	// ActionWrite — requires admin or superadmin role; must be paired with a tenant-scoped Resource.
	ActionWrite Action = "write"
	// ActionAdmin — requires superadmin role; must be paired with a tenant-scoped Resource.
	ActionAdmin Action = "admin"
	// ActionGlobal — for schema-level operations that are intentionally not tenant-scoped (e.g.
	// CreateSchema, PublishSchema). Requires superadmin. TenantScopeGuard always skips this action.
	ActionGlobal Action = "global"
)

// Resource carries the subject of an authorization check.
type Resource struct {
	TenantID  string
	FieldPath string // non-empty only for field-level checks
	Value     string // attempted new value, for value-scoped field locks
}

// Guard is a single authorization check.
type Guard interface {
	Check(ctx context.Context, action Action, resource Resource) error
}

// ChainGuard runs a slice of guards in order, short-circuiting on the first error.
type ChainGuard []Guard

// Chain composes guards into a ChainGuard.
func Chain(guards ...Guard) ChainGuard {
	return ChainGuard(guards)
}

func (c ChainGuard) Check(ctx context.Context, action Action, resource Resource) error {
	for _, g := range c {
		if err := g.Check(ctx, action, resource); err != nil {
			return err
		}
	}
	return nil
}
