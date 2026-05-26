package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	"go.opentelemetry.io/otel/metric"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	celpkg "github.com/opendecree/decree/internal/schema/cel"
	"github.com/opendecree/decree/internal/storage/domain"
)

// Store defines the read-only data access needed by the validator factory.
// Implementations must return [domain.ErrNotFound] when an entity is not found.
// This interface is a subset of the config.Store — any config store implementation
// automatically satisfies it.
type Store interface {
	GetTenantByID(ctx context.Context, id string) (domain.Tenant, error)
	GetSchemaVersion(ctx context.Context, arg domain.SchemaVersionKey) (domain.SchemaVersion, error)
	GetSchemaFields(ctx context.Context, schemaVersionID string) ([]domain.SchemaField, error)
}

// ValidatorFactory builds and caches field validators per tenant.
type ValidatorFactory struct {
	store             Store
	cache             *ValidatorCache
	rulesCache        sync.Map // tenantID → []byte (raw dependent_required JSON)
	validationsCache  sync.Map // tenantID → []*pb.ValidationRule
	celEnvCache       sync.Map // tenantID → *cel.Env
	celProgramsCache  sync.Map // tenantID → []cel.Program
	celCache          *celpkg.Cache
	limits            Limits
	celCapCounter     metric.Int64Counter // nil when metrics are disabled
	celSoftErrCounter metric.Int64Counter // nil when metrics are disabled
}

// NewValidatorFactory creates a new validator factory. Pass [WithLimits]
// to override the JSON-Schema compile defaults.
func NewValidatorFactory(store Store, opts ...Option) *ValidatorFactory {
	o := resolveOptions(opts)
	return &ValidatorFactory{
		store:             store,
		cache:             NewValidatorCache(0),
		celCache:          celpkg.NewCache(),
		limits:            o.limits,
		celCapCounter:     o.celCapCounter,
		celSoftErrCounter: o.celSoftErrCounter,
	}
}

// CelCapCounter returns the OTEL counter for aggregate CEL cost cap
// exceedances (nil when metrics are disabled). Callers pass it to
// celpkg.Eval via celpkg.WithCapCounter.
func (f *ValidatorFactory) CelCapCounter() metric.Int64Counter {
	return f.celCapCounter
}

// CelSoftErrCounter returns the OTEL counter for CEL soft errors in lenient
// mode (nil when metrics are disabled). Callers pass it to celpkg.Eval via
// celpkg.WithSoftErrCounter.
func (f *ValidatorFactory) CelSoftErrCounter() metric.Int64Counter {
	return f.celSoftErrCounter
}

// Cache returns the underlying cache for invalidation.
func (f *ValidatorFactory) Cache() *ValidatorCache {
	return f.cache
}

// InvalidateRules drops every cached rule-derived artifact for a tenant:
// dependentRequired bytes, validations slice, the cel.Env, and the
// compiled cel.Program slice. Call this alongside Cache().Invalidate()
// whenever a tenant's schema version changes. Compiled programs that
// remain in the celCache are also dropped so a stale env reference cannot
// be reached on the next compile.
func (f *ValidatorFactory) InvalidateRules(tenantID string) {
	f.rulesCache.Delete(tenantID)
	f.validationsCache.Delete(tenantID)
	f.celEnvCache.Delete(tenantID)
	f.celProgramsCache.Delete(tenantID)
}

// GetDependentRequired returns the raw JSON-encoded dependentRequired rules
// for a tenant's bound schema version. Returns nil bytes for "no rules";
// callers should treat that as a no-op. Cached per tenant; invalidate via
// InvalidateRules when the tenant's schema binding changes.
//
// Returns []byte rather than the decoded proto type so the validation
// package does not have to import internal/schema for the unmarshal helper
// (avoiding a circular import). Decode at the call site.
func (f *ValidatorFactory) GetDependentRequired(ctx context.Context, tenantID string) ([]byte, error) {
	if v, ok := f.rulesCache.Load(tenantID); ok {
		return v.([]byte), nil
	}
	tenant, err := f.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sv, err := f.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, err
	}
	raw := sv.DependentRequired
	f.rulesCache.Store(tenantID, raw)
	return raw, nil
}

// GetValidators returns validators for a tenant's schema fields.
// Results are cached per tenant ID. Returns an error if the tenant or schema is not found.
func (f *ValidatorFactory) GetValidators(ctx context.Context, tenantID string) (map[string]*FieldValidator, error) {
	if cached, ok := f.cache.Get(tenantID); ok {
		return cached, nil
	}

	tenant, err := f.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	sv, err := f.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, err
	}

	fields, err := f.store.GetSchemaFields(ctx, sv.ID)
	if err != nil {
		return nil, err
	}

	validators := make(map[string]*FieldValidator, len(fields))
	for _, field := range fields {
		ft := field.FieldType.ToProto()
		var constraints *pb.FieldConstraints
		if field.Constraints != nil {
			constraints = &pb.FieldConstraints{}
			_ = json.Unmarshal(field.Constraints, constraints)
		}
		if f.limits.RegexMaxLength > 0 && constraints.GetRegex() != "" && len(constraints.GetRegex()) > f.limits.RegexMaxLength {
			return nil, fmt.Errorf("field %s: regex pattern exceeds maximum length of %d characters", field.Path, f.limits.RegexMaxLength)
		}
		validators[field.Path] = NewFieldValidator(field.Path, ft, field.Nullable, field.Sensitive, constraints, WithLimits(f.limits))
	}

	f.cache.Set(tenantID, validators)
	return validators, nil
}

// GetValidations returns the decoded list of CEL validation rules for a
// tenant's bound schema version. The shape returned is the proto slice so
// the runtime evaluator can index in parallel with the compiled program
// slice from GetCelArtifacts. Returns nil for "no rules" — callers should
// treat that as a no-op.
//
// Cached per tenant; invalidate via InvalidateRules when the tenant's
// schema binding changes.
func (f *ValidatorFactory) GetValidations(ctx context.Context, tenantID string) ([]*pb.ValidationRule, error) {
	if v, ok := f.validationsCache.Load(tenantID); ok {
		return v.([]*pb.ValidationRule), nil
	}
	tenant, err := f.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	sv, err := f.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, err
	}
	rules := decodeValidations(sv.Validations)
	f.validationsCache.Store(tenantID, rules)
	return rules, nil
}

// GetCelArtifacts returns the (env, programs) pair for a tenant's bound
// schema version. The slices are aligned by index with the validation rules
// returned by GetValidations; callers can pass them straight through to
// celpkg.Eval. Returns (nil, nil, nil) when the schema has no rules — same
// "treat as no-op" contract as GetValidations.
//
// Programs are compiled once and pinned to (schemaID, schemaVersion,
// ruleIndex) in the shared celpkg.Cache. Invalidation drops them.
func (f *ValidatorFactory) GetCelArtifacts(ctx context.Context, tenantID string) (*cel.Env, []cel.Program, error) {
	rules, err := f.GetValidations(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	if len(rules) == 0 {
		return nil, nil, nil
	}

	if env, ok := f.celEnvCache.Load(tenantID); ok {
		if progs, pok := f.celProgramsCache.Load(tenantID); pok {
			return env.(*cel.Env), progs.([]cel.Program), nil
		}
	}

	tenant, err := f.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, nil, err
	}
	sv, err := f.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, nil, err
	}
	domainFields, err := f.store.GetSchemaFields(ctx, sv.ID)
	if err != nil {
		return nil, nil, err
	}

	pbFields := make([]*pb.SchemaField, len(domainFields))
	for i, df := range domainFields {
		pbFields[i] = &pb.SchemaField{
			Path:     df.Path,
			Type:     df.FieldType.ToProto(),
			Nullable: df.Nullable,
		}
	}

	env, err := celpkg.BuildEnv(pbFields)
	if err != nil {
		return nil, nil, fmt.Errorf("build cel env: %w", err)
	}
	programs := make([]cel.Program, len(rules))
	for i, r := range rules {
		prog, err := f.celCache.ProgramFor(env, r, sv.SchemaID, sv.Version, i)
		if err != nil {
			return nil, nil, fmt.Errorf("compile validations[%d]: %w", i, err)
		}
		programs[i] = prog
	}

	f.celEnvCache.Store(tenantID, env)
	f.celProgramsCache.Store(tenantID, programs)
	return env, programs, nil
}

// decodeValidations is a private helper so the validation package does not
// have to import internal/schema for the unmarshal — keeping the layering
// clean. Empty/nil input returns nil.
func decodeValidations(raw []byte) []*pb.ValidationRule {
	if len(raw) == 0 {
		return nil
	}
	var rules []*pb.ValidationRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil
	}
	return rules
}
