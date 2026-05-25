package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/cel-go/cel"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/audit"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/authz"
	"github.com/opendecree/decree/internal/cache"
	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/pubsub"
	"github.com/opendecree/decree/internal/schema"
	celpkg "github.com/opendecree/decree/internal/schema/cel"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/telemetry"
	"github.com/opendecree/decree/internal/validation"
)

// dependentRequiredError wraps a CheckDependentRequired error returned
// from inside a transaction so the outer status mapping can distinguish a
// validation violation (InvalidArgument) from a generic tx failure
// (Internal).
type dependentRequiredError struct{ err error }

func (e *dependentRequiredError) Error() string { return e.err.Error() }
func (e *dependentRequiredError) Unwrap() error { return e.err }

// validationError wraps one-or-more CEL rule failures aggregated by
// celpkg.Eval. Peer to dependentRequiredError — both map to
// codes.InvalidArgument at the gRPC boundary via mapCrossFieldErr.
type validationError struct{ err error }

func (e *validationError) Error() string { return e.err.Error() }
func (e *validationError) Unwrap() error { return e.err }

const (
	defaultCacheTTL = 5 * time.Minute

	// getFieldsConcurrency caps in-flight per-field reads in GetFields.
	// Bounded so a large FieldPaths request cannot exhaust the DB pool.
	getFieldsConcurrency = 16
)

// Option configures a Service.
type Option func(*serviceOptions)

type serviceOptions struct {
	logger       *slog.Logger
	cacheMetrics *telemetry.CacheMetrics
	metrics      *telemetry.ConfigMetrics
	validators   *validation.ValidatorFactory
	recorder     *audit.UsageRecorder
	guard        authz.Guard
	limits       Limits
}

// WithLogger sets the service logger. Defaults to slog.Default() when unset.
func WithLogger(l *slog.Logger) Option {
	return func(o *serviceOptions) { o.logger = l }
}

// WithCacheMetrics wires cache hit/miss metrics. Nil disables them.
func WithCacheMetrics(m *telemetry.CacheMetrics) Option {
	return func(o *serviceOptions) { o.cacheMetrics = m }
}

// WithMetrics wires write/version metrics. Nil disables them.
func WithMetrics(m *telemetry.ConfigMetrics) Option {
	return func(o *serviceOptions) { o.metrics = m }
}

// WithValidators wires the schema validator factory. Nil disables per-field
// validation and dependentRequired checks.
func WithValidators(v *validation.ValidatorFactory) Option {
	return func(o *serviceOptions) { o.validators = v }
}

// WithRecorder wires an audit usage recorder. Nil disables read tracking.
func WithRecorder(r *audit.UsageRecorder) Option {
	return func(o *serviceOptions) { o.recorder = r }
}

// WithGuard overrides the default authorization guard chain.
func WithGuard(g authz.Guard) Option {
	return func(o *serviceOptions) { o.guard = g }
}

// WithLimits caps the number of entries in repeated request fields.
// Defaults to [DefaultLimits] when unset.
func WithLimits(l Limits) Option {
	return func(o *serviceOptions) { o.limits = l }
}

// Service implements the ConfigService gRPC server.
type Service struct {
	pb.UnimplementedConfigServiceServer
	store        Store
	cache        cache.ConfigCache
	publisher    pubsub.Publisher
	subscriber   pubsub.Subscriber
	logger       *slog.Logger
	cacheMetrics *telemetry.CacheMetrics
	metrics      *telemetry.ConfigMetrics
	validators   *validation.ValidatorFactory
	recorder     *audit.UsageRecorder
	guard        authz.Guard
	limits       Limits
}

// NewService creates a new ConfigService. The four required dependencies
// (store, cache, publisher, subscriber) are positional; everything else is
// optional and may be passed via With...() options.
func NewService(store Store, cache cache.ConfigCache, publisher pubsub.Publisher, subscriber pubsub.Subscriber, opts ...Option) *Service {
	o := serviceOptions{logger: slog.Default(), limits: DefaultLimits()}
	for _, opt := range opts {
		opt(&o)
	}
	if o.guard == nil {
		o.guard = authz.Chain(
			authz.TenantScopeGuard{},
			authz.RolePolicyGuard{},
			authz.NewFieldLockGuard(store),
		)
	}
	return &Service{
		store:        store,
		cache:        cache,
		publisher:    publisher,
		subscriber:   subscriber,
		logger:       o.logger,
		cacheMetrics: o.cacheMetrics,
		metrics:      o.metrics,
		validators:   o.validators,
		recorder:     o.recorder,
		guard:        o.guard,
		limits:       o.limits,
	}
}

// resolveTenantID resolves a tenant UUID or name slug to a canonical UUID.
// Returns a gRPC status error on failure — callers can return the error directly.
// Slug resolution happens before access checks — access checks require the UUID,
// and all downstream store operations use UUIDs as primary keys.
func (s *Service) resolveTenantID(ctx context.Context, idOrName string) (string, error) {
	if idOrName == "" {
		return "", status.Error(codes.InvalidArgument, "tenant id or name required")
	}
	if domain.IsUUID(idOrName) {
		return idOrName, nil
	}
	tenant, err := s.store.GetTenantByName(ctx, idOrName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return "", status.Error(codes.NotFound, "tenant not found")
		}
		return "", status.Error(codes.Internal, "failed to resolve tenant")
	}
	return tenant.ID, nil
}

// resolveTenantWithAccess resolves a tenant and checks caller access in one step.
// Returns a gRPC status error on failure — callers can return the error directly.
func (s *Service) resolveTenantWithAccess(ctx context.Context, idOrName string, action authz.Action) (string, error) {
	tenantID, err := s.resolveTenantID(ctx, idOrName)
	if err != nil {
		return "", err
	}
	if err := s.guard.Check(ctx, action, authz.Resource{TenantID: tenantID}); err != nil {
		return "", err
	}
	return tenantID, nil
}

// errToStatus maps a domain store error to a gRPC status error.
func errToStatus(err error, notFoundMsg, failedMsg string) error {
	if errors.Is(err, domain.ErrNotFound) {
		return status.Error(codes.NotFound, notFoundMsg)
	}
	return status.Error(codes.Internal, failedMsg)
}

// --- Read operations ---

func (s *Service) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return nil, err
	}

	// Version resolution and type-map lookup hit different stores
	// (config vs schema/validators) and are independent — fan out.
	// Plain errgroup (no WithContext) keeps the parent ctx unchanged so
	// downstream calls and tests see the same ctx identity.
	var (
		version int32
		types   map[string]domain.FieldType
		g       errgroup.Group
	)
	g.Go(func() error {
		v, err := s.resolveVersion(ctx, tenantID, req.Version)
		if err != nil {
			return err
		}
		version = v
		return nil
	})
	g.Go(func() error {
		var err error
		types, err = s.fieldTypeMap(ctx, tenantID)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// If descriptions not requested, try cache.
	if !req.IncludeDescriptions {
		if cached, err := s.cache.Get(ctx, tenantID, version); err == nil && cached != nil {
			s.cacheMetrics.Hit(ctx)
			values := make([]*pb.ConfigValue, 0, len(cached))
			paths := make([]string, 0, len(cached))
			for path, val := range cached {
				v := val
				values = append(values, &pb.ConfigValue{
					FieldPath: path,
					Value:     stringToTypedValue(&v, lookupFieldType(types, path)),
					Checksum:  computeChecksum(val),
				})
				paths = append(paths, path)
			}
			s.recorder.RecordReads(tenantID, paths, s.actorPtr(ctx))
			return &pb.GetConfigResponse{
				Config: &pb.Config{TenantId: tenantID, Version: version, Values: values},
			}, nil
		}
		s.cacheMetrics.Miss(ctx)
	}

	// Fetch from DB.
	rows, err := s.store.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get config")
	}

	sensitiveFields, err := s.getSensitiveFieldSet(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	values := make([]*pb.ConfigValue, 0, len(rows))
	cacheMap := make(map[string]string, len(rows))
	for _, row := range rows {
		displayValue := redactIfSensitive(sensitiveFields[row.FieldPath], derefString(row.Value))
		cv := &pb.ConfigValue{
			FieldPath: row.FieldPath,
			Value:     stringToTypedValue(&displayValue, lookupFieldType(types, row.FieldPath)),
			Checksum:  derefString(row.Checksum),
		}
		if req.IncludeDescriptions && row.Description != nil {
			cv.Description = row.Description
		}
		values = append(values, cv)
		cacheMap[row.FieldPath] = displayValue
	}

	// Populate cache (values only, no descriptions).
	if !req.IncludeDescriptions {
		if err := s.cache.Set(ctx, tenantID, version, cacheMap, defaultCacheTTL); err != nil {
			s.logger.WarnContext(ctx, "failed to populate cache", "error", err)
		}
	}

	// Record usage for all returned fields.
	paths := make([]string, 0, len(values))
	for _, v := range values {
		paths = append(paths, v.FieldPath)
	}
	s.recorder.RecordReads(tenantID, paths, s.actorPtr(ctx))

	return &pb.GetConfigResponse{
		Config: &pb.Config{TenantId: tenantID, Version: version, Values: values},
	}, nil
}

func (s *Service) GetField(ctx context.Context, req *pb.GetFieldRequest) (*pb.GetFieldResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return nil, err
	}

	version, err := s.resolveVersion(ctx, tenantID, req.Version)
	if err != nil {
		return nil, err
	}

	row, err := s.store.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{
		TenantID:  tenantID,
		FieldPath: req.FieldPath,
		Version:   version,
	})
	if err != nil {
		return nil, errToStatus(err, "field not found", "failed to get field")
	}

	types, err := s.fieldTypeMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	cv := &pb.ConfigValue{
		FieldPath: row.FieldPath,
		Value:     stringToTypedValue(row.Value, lookupFieldType(types, row.FieldPath)),
		Checksum:  derefString(row.Checksum),
	}
	if req.IncludeDescription && row.Description != nil {
		cv.Description = row.Description
	}

	s.recorder.RecordRead(tenantID, req.FieldPath, s.actorPtr(ctx))

	return &pb.GetFieldResponse{Value: cv}, nil
}

func (s *Service) GetFields(ctx context.Context, req *pb.GetFieldsRequest) (*pb.GetFieldsResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if s.limits.MaxListLen > 0 && len(req.FieldPaths) > s.limits.MaxListLen {
		return nil, status.Errorf(codes.InvalidArgument, "request has %d field_paths, exceeds limit of %d", len(req.FieldPaths), s.limits.MaxListLen)
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return nil, err
	}

	// Version and type-map are independent — fan out. Plain errgroup keeps
	// parent ctx identity for downstream calls + tests.
	var (
		version int32
		types   map[string]domain.FieldType
	)
	{
		var g errgroup.Group
		g.Go(func() error {
			v, err := s.resolveVersion(ctx, tenantID, req.Version)
			if err != nil {
				return err
			}
			version = v
			return nil
		})
		g.Go(func() error {
			var err error
			types, err = s.fieldTypeMap(ctx, tenantID)
			return err
		})
		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	// Fan out per-field reads. Slot index preserves request order; nil slots
	// are missing fields and get filtered after the group completes.
	rows := make([]*pb.ConfigValue, len(req.FieldPaths))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(getFieldsConcurrency)
	for i, path := range req.FieldPaths {
		g.Go(func() error {
			row, err := s.store.GetConfigValueAtVersion(gctx, GetConfigValueAtVersionParams{
				TenantID:  tenantID,
				FieldPath: path,
				Version:   version,
			})
			if err != nil {
				if errors.Is(err, domain.ErrNotFound) {
					return nil // Missing field: leave slot nil.
				}
				return status.Error(codes.Internal, "failed to get field")
			}
			cv := &pb.ConfigValue{
				FieldPath: row.FieldPath,
				Value:     stringToTypedValue(row.Value, lookupFieldType(types, row.FieldPath)),
				Checksum:  derefString(row.Checksum),
			}
			if req.IncludeDescriptions && row.Description != nil {
				cv.Description = row.Description
			}
			rows[i] = cv
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	values := make([]*pb.ConfigValue, 0, len(rows))
	for _, cv := range rows {
		if cv != nil {
			values = append(values, cv)
		}
	}

	// Record usage for all returned fields.
	fieldPaths := make([]string, 0, len(values))
	for _, v := range values {
		fieldPaths = append(fieldPaths, v.FieldPath)
	}
	s.recorder.RecordReads(tenantID, fieldPaths, s.actorPtr(ctx))

	return &pb.GetFieldsResponse{Values: values}, nil
}

// --- Write operations ---

func (s *Service) SetField(ctx context.Context, req *pb.SetFieldRequest) (*pb.SetFieldResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantID(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{TenantID: tenantID, FieldPath: req.FieldPath}); err != nil {
		return nil, err
	}

	actor := s.getActor(ctx)

	// Pre-transaction validation (schema/lock checks only).
	if err := s.validateField(ctx, tenantID, req.FieldPath, req.Value); err != nil {
		return nil, err
	}

	latestVersion, err := s.getOrCreateVersion(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	oldValue := s.getCurrentValue(ctx, tenantID, req.FieldPath, latestVersion)

	sensitiveFields, err := s.getSensitiveFieldSet(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	depRules, err := s.fetchDependentRequiredRules(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load dependentRequired rules")
	}

	// Transaction: version + value + audit + dependentRequired check.
	// Re-reading the latest version inside the tx and verifying the checksum
	// there closes the TOCTOU window: concurrent writers must serialise through
	// the UNIQUE(tenant_id, version) constraint on CreateConfigVersion, so at
	// most one writer succeeds per version slot.
	var newVersion domain.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		txLockedVersion, txErr := txLatestVersion(ctx, tx, tenantID)
		if txErr != nil {
			return txErr
		}
		if req.ExpectedChecksum != nil {
			if txErr = checkChecksumAtVersion(ctx, tx, tenantID, req.FieldPath, *req.ExpectedChecksum, txLockedVersion); txErr != nil {
				return txErr
			}
		}
		newVersion, txErr = tx.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     txLockedVersion + 1,
			Description: ptrString(req.GetDescription()),
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create config version: %w", txErr)
		}

		valStr := typedValueToString(req.Value)
		if txErr = tx.SetConfigValue(ctx, SetConfigValueParams{
			ConfigVersionID: newVersion.ID,
			FieldPath:       req.FieldPath,
			Value:           valStr,
			Checksum:        checksumPtr(valStr),
			Description:     ptrString(req.GetValueDescription()),
		}); txErr != nil {
			return fmt.Errorf("set config value: %w", txErr)
		}

		if txErr = s.enforceDependentRequiredInTx(ctx, tx, tenantID, newVersion.Version, depRules); txErr != nil {
			return txErr
		}

		newValueStr := typedValueToString(req.Value)
		redactedOld := redactIfSensitive(sensitiveFields[req.FieldPath], oldValue)
		redactedNew := redactIfSensitive(sensitiveFields[req.FieldPath], derefString(newValueStr))
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:      tenantID,
			Actor:         actor,
			Action:        "set_field",
			ObjectKind:    "field",
			FieldPath:     ptrString(req.FieldPath),
			OldValue:      ptrString(redactedOld),
			NewValue:      ptrString(redactedNew),
			ConfigVersion: &newVersion.Version,
		})
	}); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "concurrent write conflict; retry with the latest checksum")
		}
		if st, ok := status.FromError(err); ok {
			return nil, st.Err()
		}
		return nil, mapDependentRequiredErr(err, func() error {
			s.logger.ErrorContext(ctx, "set field transaction failed", "error", err)
			return status.Error(codes.Internal, "failed to set field")
		})
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, tenantID); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}
	s.publishChange(ctx, tenantID, newVersion.Version, req.FieldPath,
		redactIfSensitive(sensitiveFields[req.FieldPath], oldValue),
		redactIfSensitive(sensitiveFields[req.FieldPath], typedValueToDisplayString(req.Value)),
		actor)

	s.metrics.RecordWrite(ctx, tenantID, "set_field")
	s.metrics.RecordVersion(ctx, tenantID, int64(newVersion.Version))

	return &pb.SetFieldResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

func (s *Service) SetFields(ctx context.Context, req *pb.SetFieldsRequest) (*pb.SetFieldsResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if s.limits.MaxListLen > 0 && len(req.Updates) > s.limits.MaxListLen {
		return nil, status.Errorf(codes.InvalidArgument, "request has %d updates, exceeds limit of %d", len(req.Updates), s.limits.MaxListLen)
	}
	// Upfront role + tenant check before the per-field loop (loop may be empty).
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionWrite)
	if err != nil {
		return nil, err
	}

	actor := s.getActor(ctx)

	// Fetch field locks once for the tenant and memoize in context so that each
	// per-field Check call reads from context instead of issuing a DB round-trip.
	if locks, err := s.store.GetFieldLocks(ctx, tenantID); err != nil {
		s.logger.ErrorContext(ctx, "failed to fetch field locks", "error", err)
		return nil, status.Error(codes.Internal, "failed to check field locks")
	} else {
		ctx = authz.WithFieldLockCache(ctx, locks)
	}

	// Pre-transaction validation (schema/lock checks only).
	for _, update := range req.Updates {
		if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{TenantID: tenantID, FieldPath: update.FieldPath}); err != nil {
			return nil, err
		}
		if err := s.validateField(ctx, tenantID, update.FieldPath, update.Value); err != nil {
			return nil, err
		}
	}

	latestVersion, err := s.getOrCreateVersion(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Collect old values for audit and change events.
	type changeRecord struct {
		fieldPath string
		oldValue  string
		newValue  string
	}
	changes := make([]changeRecord, 0, len(req.Updates))
	for _, update := range req.Updates {
		changes = append(changes, changeRecord{
			fieldPath: update.FieldPath,
			oldValue:  s.getCurrentValue(ctx, tenantID, update.FieldPath, latestVersion),
			newValue:  typedValueToDisplayString(update.Value),
		})
	}

	sensitiveFieldsMulti, err := s.getSensitiveFieldSet(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	depRules, err := s.fetchDependentRequiredRules(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load dependentRequired rules")
	}

	// Transaction: version + all values + all audit entries + dependentRequired check.
	// Checksums are verified inside the tx after re-reading the latest version so
	// concurrent writes cannot bypass the check (TOCTOU fix for #417).
	var newVersion domain.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		txLockedVersion, txErr := txLatestVersion(ctx, tx, tenantID)
		if txErr != nil {
			return txErr
		}
		for _, update := range req.Updates {
			if update.ExpectedChecksum != nil {
				if txErr = checkChecksumAtVersion(ctx, tx, tenantID, update.FieldPath, *update.ExpectedChecksum, txLockedVersion); txErr != nil {
					return txErr
				}
			}
		}
		newVersion, txErr = tx.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     txLockedVersion + 1,
			Description: ptrString(req.GetDescription()),
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create config version: %w", txErr)
		}

		for i, update := range req.Updates {
			updateValStr := typedValueToString(update.Value)
			if txErr = tx.SetConfigValue(ctx, SetConfigValueParams{
				ConfigVersionID: newVersion.ID,
				FieldPath:       update.FieldPath,
				Value:           updateValStr,
				Checksum:        checksumPtr(updateValStr),
				Description:     ptrString(update.GetValueDescription()),
			}); txErr != nil {
				return fmt.Errorf("set config value %s: %w", update.FieldPath, txErr)
			}

			newValueStr := typedValueToString(update.Value)
			redactedNew := redactIfSensitive(sensitiveFieldsMulti[update.FieldPath], derefString(newValueStr))
			redactedOld := redactIfSensitive(sensitiveFieldsMulti[update.FieldPath], changes[i].oldValue)
			if txErr = tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
				TenantID:      tenantID,
				Actor:         actor,
				Action:        "set_field",
				ObjectKind:    "field",
				FieldPath:     ptrString(update.FieldPath),
				OldValue:      ptrString(redactedOld),
				NewValue:      ptrString(redactedNew),
				ConfigVersion: &newVersion.Version,
			}); txErr != nil {
				return fmt.Errorf("insert audit log for %s: %w", update.FieldPath, txErr)
			}
		}

		return s.enforceDependentRequiredInTx(ctx, tx, tenantID, newVersion.Version, depRules)
	}); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "concurrent write conflict; retry with the latest checksum")
		}
		if st, ok := status.FromError(err); ok {
			return nil, st.Err()
		}
		return nil, mapDependentRequiredErr(err, func() error {
			s.logger.ErrorContext(ctx, "set fields transaction failed", "error", err)
			return status.Error(codes.Internal, "failed to set fields")
		})
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, tenantID); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}
	for _, ch := range changes {
		s.publishChange(ctx, tenantID, newVersion.Version, ch.fieldPath,
			redactIfSensitive(sensitiveFieldsMulti[ch.fieldPath], ch.oldValue),
			redactIfSensitive(sensitiveFieldsMulti[ch.fieldPath], ch.newValue),
			actor)
	}

	s.metrics.RecordWrite(ctx, tenantID, "set_fields")
	s.metrics.RecordVersion(ctx, tenantID, int64(newVersion.Version))

	return &pb.SetFieldsResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

// --- Version operations ---

func (s *Service) ListVersions(ctx context.Context, req *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return nil, err
	}

	pageSize := pagination.ClampPageSize(req.PageSize, 50, 500)

	offset, err := pagination.DecodePageToken(req.PageToken)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page token")
	}

	versions, err := s.store.ListConfigVersions(ctx, ListConfigVersionsParams{
		TenantID: tenantID,
		Limit:    pageSize + 1,
		Offset:   offset,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list versions")
	}

	nextToken := pagination.NextPageToken(pageSize, int32(len(versions)), offset)
	if int32(len(versions)) > pageSize {
		versions = versions[:pageSize]
	}

	pbVersions := make([]*pb.ConfigVersion, 0, len(versions))
	for _, v := range versions {
		pbVersions = append(pbVersions, configVersionToProto(v))
	}

	return &pb.ListVersionsResponse{
		Versions:      pbVersions,
		NextPageToken: nextToken,
	}, nil
}

func (s *Service) GetVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return nil, err
	}

	version, err := s.store.GetConfigVersion(ctx, GetConfigVersionParams{
		TenantID: tenantID,
		Version:  req.Version,
	})
	if err != nil {
		return nil, errToStatus(err, "version not found", "failed to get version")
	}

	return &pb.GetVersionResponse{ConfigVersion: configVersionToProto(version)}, nil
}

func (s *Service) RollbackToVersion(ctx context.Context, req *pb.RollbackToVersionRequest) (*pb.RollbackToVersionResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionWrite)
	if err != nil {
		return nil, err
	}

	actor := s.getActor(ctx)

	// Pre-transaction reads.
	targetRows, err := s.store.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID,
		Version:  req.Version,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get target version config")
	}
	if len(targetRows) == 0 {
		return nil, status.Error(codes.NotFound, "target version not found or empty")
	}

	latest, err := s.store.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get latest version")
	}

	desc := fmt.Sprintf("Rollback to version %d", req.Version)
	if req.Description != nil {
		desc = *req.Description
	}

	depRules, err := s.fetchDependentRequiredRules(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load dependentRequired rules")
	}

	// Transaction: new version + copied values + audit + dependentRequired check.
	var newVersion domain.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		newVersion, txErr = tx.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     latest.Version + 1,
			Description: &desc,
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create rollback version: %w", txErr)
		}

		for _, row := range targetRows {
			if txErr = tx.SetConfigValue(ctx, SetConfigValueParams{
				ConfigVersionID: newVersion.ID,
				FieldPath:       row.FieldPath,
				Value:           row.Value,
				Checksum:        row.Checksum,
				Description:     row.Description,
			}); txErr != nil {
				return fmt.Errorf("copy field %s: %w", row.FieldPath, txErr)
			}
		}

		if txErr = s.enforceDependentRequiredInTx(ctx, tx, tenantID, newVersion.Version, depRules); txErr != nil {
			return txErr
		}

		newValue := fmt.Sprintf("v%d", req.Version)
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:      tenantID,
			Actor:         actor,
			Action:        "rollback",
			ObjectKind:    "field",
			FieldPath:     ptrString(""),
			OldValue:      ptrString(""),
			NewValue:      &newValue,
			ConfigVersion: &newVersion.Version,
		})
	}); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "concurrent write conflict; retry the rollback")
		}
		return nil, mapDependentRequiredErr(err, func() error {
			s.logger.ErrorContext(ctx, "rollback transaction failed", "error", err)
			return status.Error(codes.Internal, "failed to rollback")
		})
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, tenantID); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}

	s.metrics.RecordWrite(ctx, tenantID, "rollback")
	s.metrics.RecordVersion(ctx, tenantID, int64(newVersion.Version))

	return &pb.RollbackToVersionResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

// --- Subscriptions ---

func (s *Service) Subscribe(req *pb.SubscribeRequest, stream grpc.ServerStreamingServer[pb.SubscribeResponse]) error {
	ctx := stream.Context()

	if err := auth.MustHaveClaims(ctx); err != nil {
		return err
	}
	if s.limits.MaxListLen > 0 && len(req.FieldPaths) > s.limits.MaxListLen {
		return status.Errorf(codes.InvalidArgument, "request has %d field_paths, exceeds limit of %d", len(req.FieldPaths), s.limits.MaxListLen)
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return err
	}

	// Subscribe to pubsub first so no live events are missed while we replay.
	events, cancel, err := s.subscriber.Subscribe(ctx, tenantID)
	if err != nil {
		return status.Error(codes.Internal, "failed to subscribe")
	}
	defer cancel()

	types, err := s.fieldTypeMap(ctx, tenantID)
	if err != nil {
		return err
	}
	filterPaths := make(map[string]struct{}, len(req.FieldPaths))
	for _, p := range req.FieldPaths {
		filterPaths[p] = struct{}{}
	}

	// watermark is the highest version already sent during replay. Live events
	// at or below this version are skipped to prevent duplicates.
	var watermark int32

	if req.StartVersion != nil {
		// Determine the latest version at subscribe time so we know where replay ends.
		latestCV, latestErr := s.store.GetLatestConfigVersion(ctx, tenantID)
		if latestErr == nil {
			watermark = latestCV.Version
		}

		rows, replayErr := s.store.GetConfigValuesSince(ctx, GetConfigValuesSinceParams{
			TenantID:     tenantID,
			StartVersion: *req.StartVersion,
		})
		if replayErr != nil {
			return status.Error(codes.Internal, "failed to replay events")
		}

		for _, row := range rows {
			if len(filterPaths) > 0 {
				if _, ok := filterPaths[row.FieldPath]; !ok {
					continue
				}
			}
			ftype := lookupFieldType(types, row.FieldPath)
			if err := stream.Send(&pb.SubscribeResponse{
				Change: &pb.ConfigChange{
					TenantId:  tenantID,
					Version:   row.Version,
					FieldPath: row.FieldPath,
					NewValue:  stringToTypedValue(row.Value, ftype),
					ChangedBy: row.CreatedBy,
					ChangedAt: timestamppb.New(row.ChangedAt),
				},
			}); err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}

			// Skip events already covered by the replay window.
			if event.Version <= watermark {
				continue
			}

			// Filter by field paths if specified.
			if len(filterPaths) > 0 {
				if _, ok := filterPaths[event.FieldPath]; !ok {
					continue
				}
			}

			if err := stream.Send(&pb.SubscribeResponse{
				Change: &pb.ConfigChange{
					TenantId:  event.TenantID,
					Version:   event.Version,
					FieldPath: event.FieldPath,
					OldValue:  stringToTypedValue(ptrString(event.OldValue), lookupFieldType(types, event.FieldPath)),
					NewValue:  stringToTypedValue(strPtr(event.NewValue), lookupFieldType(types, event.FieldPath)),
					ChangedBy: event.ChangedBy,
					ChangedAt: timestamppb.New(event.ChangedAt),
				},
			}); err != nil {
				return err
			}
		}
	}
}

// --- Import/export ---

func (s *Service) ExportConfig(ctx context.Context, req *pb.ExportConfigRequest) (*pb.ExportConfigResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionRead)
	if err != nil {
		return nil, err
	}

	version, err := s.resolveVersion(ctx, tenantID, req.Version)
	if err != nil {
		return nil, err
	}
	if version == 0 {
		return nil, status.Error(codes.NotFound, "no config versions exist for this tenant")
	}

	// Get schema field types for typed value conversion.
	fieldTypes, err := s.getFieldTypeMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Fetch all config values at the requested version.
	dbRows, err := s.store.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get config values")
	}
	if len(dbRows) == 0 {
		return nil, status.Error(codes.NotFound, "no config values at this version")
	}

	rows := make([]configRow, len(dbRows))
	for i, r := range dbRows {
		rows[i] = configRow{FieldPath: r.FieldPath, Value: derefString(r.Value), Description: r.Description}
	}

	sensitiveFieldsExport, err := s.getSensitiveFieldSet(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for i, row := range rows {
		rows[i].Value = redactIfSensitive(sensitiveFieldsExport[row.FieldPath], row.Value)
	}

	// Get version description.
	var description string
	cv, err := s.store.GetConfigVersion(ctx, GetConfigVersionParams{
		TenantID: tenantID,
		Version:  version,
	})
	if err == nil && cv.Description != nil {
		description = *cv.Description
	}

	specVersion := ""
	if req.SpecVersion != nil {
		specVersion = *req.SpecVersion
	}
	data, err := MarshalConfigAt(version, description, rows, fieldTypes, specVersion)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &pb.ExportConfigResponse{YamlContent: data}, nil
}

func (s *Service) ImportConfig(ctx context.Context, req *pb.ImportConfigRequest) (*pb.ImportConfigResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if s.limits.MaxDocBytes > 0 && len(req.YamlContent) > s.limits.MaxDocBytes {
		return nil, status.Errorf(codes.InvalidArgument, "config document is %d bytes, exceeds limit of %d", len(req.YamlContent), s.limits.MaxDocBytes)
	}

	// Upfront role + tenant check before any store reads.
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId, authz.ActionWrite)
	if err != nil {
		return nil, err
	}

	actor := s.getActor(ctx)

	// Verify tenant exists.
	if _, err := s.store.GetTenantByID(ctx, tenantID); err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}

	// Get schema field types for type-aware parsing.
	fieldTypes, err := s.getFieldTypeMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Dispatch to the parser registered for the document's spec_version,
	// converting YAML values to canonical string form along the way.
	parsed, err := DispatchImport(req.YamlContent, fieldTypes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid config YAML: %v", err)
	}
	values := parsed.Values

	// Check field locks and validate.
	for _, v := range values {
		if s.limits.MaxFieldValueBytes > 0 && len(v.Value) > s.limits.MaxFieldValueBytes {
			return nil, status.Errorf(codes.InvalidArgument, "field %q value is %d bytes, exceeds limit of %d", v.FieldPath, len(v.Value), s.limits.MaxFieldValueBytes)
		}
		if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{TenantID: tenantID, FieldPath: v.FieldPath}); err != nil {
			return nil, err
		}
		// Convert string value to TypedValue for validation.
		ft := fieldTypes[v.FieldPath]
		tv := stringToTypedValue(&v.Value, ft)
		if err := s.validateField(ctx, tenantID, v.FieldPath, tv); err != nil {
			return nil, err
		}
	}

	latestVersion, err := s.getOrCreateVersion(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Filter values based on import mode.
	values = s.filterByImportMode(ctx, tenantID, latestVersion, values, req.Mode)
	if len(values) == 0 {
		return nil, status.Error(codes.AlreadyExists, "no changes to apply")
	}

	// Collect old values for audit and change events. Fan out the per-field
	// reads — getCurrentValue is N independent point lookups; large imports
	// would otherwise pay the round-trip cost N times sequentially.
	type changeRecord struct {
		fieldPath string
		oldValue  string
		newValue  string
	}
	changes := make([]changeRecord, len(values))
	{
		changeG, changeCtx := errgroup.WithContext(ctx)
		changeG.SetLimit(getFieldsConcurrency)
		for i, v := range values {
			changeG.Go(func() error {
				changes[i] = changeRecord{
					fieldPath: v.FieldPath,
					oldValue:  s.getCurrentValue(changeCtx, tenantID, v.FieldPath, latestVersion),
					newValue:  v.Value,
				}
				return nil
			})
		}
		_ = changeG.Wait() // getCurrentValue swallows errors internally.
	}

	// Import description.
	desc := "Import from YAML"
	if req.Description != nil {
		desc = *req.Description
	} else if parsed.Description != "" {
		desc = parsed.Description
	}

	sensitiveFields, err := s.getSensitiveFieldSet(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	depRules, err := s.fetchDependentRequiredRules(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load dependentRequired rules")
	}

	// Transaction: version + all values + audit entries + dependentRequired check.
	var newVersion domain.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		newVersion, txErr = tx.CreateConfigVersion(ctx, CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     latestVersion + 1,
			Description: &desc,
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create config version: %w", txErr)
		}

		for i, v := range values {
			importValStr := strPtr(v.Value)
			if txErr = tx.SetConfigValue(ctx, SetConfigValueParams{
				ConfigVersionID: newVersion.ID,
				FieldPath:       v.FieldPath,
				Value:           importValStr,
				Checksum:        checksumPtr(importValStr),
				Description:     v.Description,
			}); txErr != nil {
				return fmt.Errorf("set config value %s: %w", v.FieldPath, txErr)
			}

			isSensitive := sensitiveFields[v.FieldPath]
			if txErr = tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
				TenantID:      tenantID,
				Actor:         actor,
				Action:        "import",
				ObjectKind:    "field",
				FieldPath:     ptrString(v.FieldPath),
				OldValue:      ptrString(redactIfSensitive(isSensitive, changes[i].oldValue)),
				NewValue:      strPtr(redactIfSensitive(isSensitive, v.Value)),
				ConfigVersion: &newVersion.Version,
			}); txErr != nil {
				return fmt.Errorf("insert audit log for %s: %w", v.FieldPath, txErr)
			}
		}

		return s.enforceDependentRequiredInTx(ctx, tx, tenantID, newVersion.Version, depRules)
	}); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return nil, status.Error(codes.Aborted, "concurrent write conflict; retry the import")
		}
		return nil, mapDependentRequiredErr(err, func() error {
			s.logger.ErrorContext(ctx, "import config transaction failed", "error", err)
			return status.Error(codes.Internal, "failed to import config")
		})
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, tenantID); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}
	for _, ch := range changes {
		s.publishChange(ctx, tenantID, newVersion.Version, ch.fieldPath,
			redactIfSensitive(sensitiveFields[ch.fieldPath], ch.oldValue),
			redactIfSensitive(sensitiveFields[ch.fieldPath], ch.newValue),
			actor)
	}

	s.metrics.RecordWrite(ctx, tenantID, "import")
	s.metrics.RecordVersion(ctx, tenantID, int64(newVersion.Version))

	return &pb.ImportConfigResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

// filterByImportMode filters config values based on the import mode.
func (s *Service) filterByImportMode(ctx context.Context, tenantID string, latestVersion int32, values []configValueImport, mode pb.ImportMode) []configValueImport {
	switch mode {
	case pb.ImportMode_IMPORT_MODE_REPLACE:
		// Replace: use all values as-is.
		return values

	case pb.ImportMode_IMPORT_MODE_DEFAULTS:
		// Defaults: only include values for fields that have no current value.
		var filtered []configValueImport
		for _, v := range values {
			current := s.getCurrentValue(ctx, tenantID, v.FieldPath, latestVersion)
			if current == "" {
				// Check if the field truly has no value (not just empty string).
				_, err := s.store.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{
					TenantID:  tenantID,
					FieldPath: v.FieldPath,
					Version:   latestVersion,
				})
				if err != nil {
					// Field doesn't exist — include it.
					filtered = append(filtered, v)
				}
				// Field exists (even if empty) — skip.
			}
			// Field has a non-empty value — skip.
		}
		return filtered

	default:
		// Merge (default): only include values that differ from current.
		if latestVersion == 0 {
			return values // No existing config — include all.
		}
		var filtered []configValueImport
		for _, v := range values {
			current := s.getCurrentValue(ctx, tenantID, v.FieldPath, latestVersion)
			if current != v.Value {
				filtered = append(filtered, v)
			}
		}
		return filtered
	}
}

// getFieldTypeMap fetches the tenant's schema fields and builds a map of field path to domain FieldType.
func (s *Service) getFieldTypeMap(ctx context.Context, tenantID string) (map[string]domain.FieldType, error) {
	tenant, err := s.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}

	sv, err := s.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get schema version")
	}

	fields, err := s.store.GetSchemaFields(ctx, sv.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get schema fields")
	}

	result := make(map[string]domain.FieldType, len(fields))
	for _, f := range fields {
		result[f.Path] = f.FieldType
	}
	return result, nil
}

// getSensitiveFieldSet returns a set of field paths that are marked sensitive
// for the tenant's current schema version.
func (s *Service) getSensitiveFieldSet(ctx context.Context, tenantID string) (map[string]bool, error) {
	tenant, err := s.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}
	sv, err := s.store.GetSchemaVersion(ctx, domain.SchemaVersionKey{
		SchemaID: tenant.SchemaID,
		Version:  tenant.SchemaVersion,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get schema version")
	}
	fields, err := s.store.GetSchemaFields(ctx, sv.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get schema fields")
	}
	result := make(map[string]bool, len(fields))
	for _, f := range fields {
		if f.Sensitive {
			result[f.Path] = true
		}
	}
	return result, nil
}

// --- Helpers ---

func (s *Service) resolveVersion(ctx context.Context, tenantID string, requested *int32) (int32, error) {
	if requested != nil {
		return *requested, nil
	}
	latest, err := s.store.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, nil // No versions yet.
		}
		return 0, status.Error(codes.Internal, "failed to get latest version")
	}
	return latest.Version, nil
}

func (s *Service) getOrCreateVersion(ctx context.Context, tenantID string) (int32, error) {
	latest, err := s.store.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, nil
		}
		return 0, status.Error(codes.Internal, "failed to get latest version")
	}
	return latest.Version, nil
}

func (s *Service) getActor(ctx context.Context) string {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return "unknown"
	}
	return claims.Subject
}

func (s *Service) actorPtr(ctx context.Context) *string {
	actor := s.getActor(ctx)
	if actor == "unknown" {
		return nil
	}
	return &actor
}

func (s *Service) getCurrentValue(ctx context.Context, tenantID string, fieldPath string, version int32) string {
	if version == 0 {
		return ""
	}
	row, err := s.store.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{
		TenantID:  tenantID,
		FieldPath: fieldPath,
		Version:   version,
	})
	if err != nil {
		return ""
	}
	return derefString(row.Value)
}

// txLatestVersion returns the latest config version number for tenantID using
// the provided Store (intended to be a tx-bound store). Returns 0 when no
// versions exist yet. Must be called inside RunInTx so that the subsequent
// CreateConfigVersion is in the same serialisable scope.
func txLatestVersion(ctx context.Context, tx Store, tenantID string) (int32, error) {
	latest, err := tx.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return 0, nil
		}
		return 0, status.Error(codes.Internal, "failed to get latest version")
	}
	return latest.Version, nil
}

// checkChecksumAtVersion verifies that the stored checksum for fieldPath at
// version matches expected. Returns nil when version is 0 or the field does
// not exist yet (first write is always allowed regardless of checksum).
func checkChecksumAtVersion(ctx context.Context, tx Store, tenantID, fieldPath, expected string, version int32) error {
	if version == 0 {
		return nil
	}
	row, err := tx.GetConfigValueAtVersion(ctx, GetConfigValueAtVersionParams{
		TenantID:  tenantID,
		FieldPath: fieldPath,
		Version:   version,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil
		}
		return status.Error(codes.Internal, "failed to get current value for checksum")
	}
	actual := derefString(row.Checksum)
	if actual != expected {
		return status.Errorf(codes.Aborted, "checksum mismatch for %s: expected %s, got %s", fieldPath, expected, actual)
	}
	return nil
}

// validateField validates a typed value against the schema constraints.
// In strict mode, rejects fields not defined in the schema.
// Returns codes.Internal if the validator store is unavailable — fail-closed
// is correct here because silently skipping validation would allow writes that
// should be rejected (security/correctness bug).
func (s *Service) validateField(ctx context.Context, tenantID, fieldPath string, value *pb.TypedValue) error {
	if s.validators == nil {
		return nil
	}
	validators, err := s.validators.GetValidators(ctx, tenantID)
	if err != nil {
		s.logger.ErrorContext(ctx, "validator lookup failed; rejecting write to fail-closed",
			"tenant_id", tenantID, "field", fieldPath, "error", err)
		return status.Error(codes.Internal, "validator lookup failed")
	}
	v, ok := validators[fieldPath]
	if !ok {
		return status.Errorf(codes.InvalidArgument, "field %s is not defined in the schema", fieldPath)
	}
	if err := v.Validate(value); err != nil {
		s.logger.DebugContext(ctx, "field validation failed", "field", fieldPath, "error", err)
		return status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return nil
}

// fetchDependentRequiredRules returns the decoded dependentRequired rules
// for a tenant's bound schema version. Returns (nil, nil) when there are no
// rules — callers can skip the runtime check entirely. Caches via the
// validator factory's per-tenant rules cache.
func (s *Service) fetchDependentRequiredRules(ctx context.Context, tenantID string) ([]*pb.DependentRequiredEntry, error) {
	if s.validators == nil {
		return nil, nil
	}
	raw, err := s.validators.GetDependentRequired(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return schema.UnmarshalDependentRequired(raw), nil
}

// enforceCrossFieldInTx runs dependentRequired then CEL validations
// against the post-merge snapshot at `version`. Both checks read the same
// snapshot to avoid a second tx round-trip; the read is bound to `tx` so
// MVCC sees the writes staged earlier in this transaction.
//
// Returns *dependentRequiredError or *validationError on rule failure so
// the outer RunInTx caller can map to codes.InvalidArgument; returns a
// wrapped store error verbatim on snapshot-read failure.
//
// No-op when both depRules and the tenant's CEL programs are empty.
func (s *Service) enforceCrossFieldInTx(
	ctx context.Context,
	tx Store,
	tenantID string,
	version int32,
	depRules []*pb.DependentRequiredEntry,
	celArtifacts *celArtifacts,
) error {
	hasDep := len(depRules) > 0
	hasCEL := celArtifacts != nil && len(celArtifacts.Programs) > 0
	if !hasDep && !hasCEL {
		return nil
	}
	rows, err := tx.GetFullConfigAtVersion(ctx, GetFullConfigAtVersionParams{
		TenantID: tenantID,
		Version:  version,
	})
	if err != nil {
		return fmt.Errorf("read snapshot for cross-field validation: %w", err)
	}
	if hasDep {
		present := make(map[string]struct{}, len(rows))
		for _, row := range rows {
			if row.Value != nil {
				present[row.FieldPath] = struct{}{}
			}
		}
		if err := schema.CheckDependentRequired(depRules, present); err != nil {
			return &dependentRequiredError{err: err}
		}
	}
	if hasCEL {
		snapshot := make([]celpkg.SnapshotRow, len(rows))
		for i, row := range rows {
			snapshot[i] = celpkg.SnapshotRow{FieldPath: row.FieldPath, Value: row.Value}
		}
		tenantMeta, err := celArtifacts.tenantBinding(ctx, tx, tenantID)
		if err != nil {
			return fmt.Errorf("load tenant for CEL activation: %w", err)
		}
		types := pbFieldTypeMap(celArtifacts.FieldTypes)
		act := celpkg.BuildActivation(snapshot, types, tenantMeta)
		failed, softErrs, evalErr := celpkg.Eval(celArtifacts.Programs, act, celArtifacts.Rules)
		if evalErr != nil {
			return fmt.Errorf("evaluate validations: %w", evalErr)
		}
		for _, soft := range softErrs {
			s.logger.WarnContext(ctx, "CEL rule eval soft-error",
				"tenant", tenantID, "version", version, "error", soft)
		}
		if len(failed) > 0 {
			return &validationError{err: aggregatedFailureError(failed)}
		}
	}
	return nil
}

// enforceDependentRequiredInTx preserves the original call-site signature
// used by the four mutating RPCs. It just forwards to enforceCrossFieldInTx
// with the CEL artifacts wired in transparently.
func (s *Service) enforceDependentRequiredInTx(ctx context.Context, tx Store, tenantID string, version int32, rules []*pb.DependentRequiredEntry) error {
	cel, err := s.fetchCelArtifactsForTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	return s.enforceCrossFieldInTx(ctx, tx, tenantID, version, rules, cel)
}

// mapDependentRequiredErr converts a tx error into the right gRPC status:
// InvalidArgument when the error wraps *dependentRequiredError or
// *validationError, the caller's fallback otherwise. Use after RunInTx
// returns.
func mapDependentRequiredErr(err error, fallback func() error) error {
	var dre *dependentRequiredError
	if errors.As(err, &dre) {
		return status.Errorf(codes.InvalidArgument, "%v", dre.err)
	}
	var ve *validationError
	if errors.As(err, &ve) {
		return status.Errorf(codes.InvalidArgument, "%v", ve.err)
	}
	return fallback()
}

// celArtifacts bundles the per-tenant artifacts needed for CEL evaluation.
// Built once per write outside the tx and threaded through
// enforceCrossFieldInTx.
type celArtifacts struct {
	Rules      []*pb.ValidationRule
	Programs   []cel.Program
	FieldTypes map[string]domain.FieldType
	getTenant  func(ctx context.Context, tx Store, tenantID string) (celpkg.TenantBinding, error)
}

func (a *celArtifacts) tenantBinding(ctx context.Context, tx Store, tenantID string) (celpkg.TenantBinding, error) {
	if a == nil || a.getTenant == nil {
		return celpkg.TenantBinding{ID: tenantID}, nil
	}
	return a.getTenant(ctx, tx, tenantID)
}

// fetchCelArtifactsForTenant loads (rules, programs, fieldTypes) for a
// tenant and packages them for enforceCrossFieldInTx. Returns nil when the
// tenant has no CEL rules; callers treat nil as no-op.
func (s *Service) fetchCelArtifactsForTenant(ctx context.Context, tenantID string) (*celArtifacts, error) {
	if s.validators == nil {
		return nil, nil
	}
	rules, err := s.validators.GetValidations(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load validations")
	}
	if len(rules) == 0 {
		return nil, nil
	}
	_, programs, err := s.validators.GetCelArtifacts(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to compile validations")
	}
	types, err := s.fieldTypeMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return &celArtifacts{
		Rules:      rules,
		Programs:   programs,
		FieldTypes: types,
		getTenant: func(ctx context.Context, tx Store, id string) (celpkg.TenantBinding, error) {
			t, err := tx.GetTenantByID(ctx, id)
			if err != nil {
				return celpkg.TenantBinding{ID: id}, err
			}
			return celpkg.TenantBinding{ID: t.ID, Name: t.Name}, nil
		},
	}, nil
}

// pbFieldTypeMap converts a domain-typed map into the pb.FieldType map that
// celpkg.BuildActivation expects. Keeping the conversion here means the cel
// package does not need to import internal/storage/domain.
func pbFieldTypeMap(in map[string]domain.FieldType) map[string]pb.FieldType {
	if in == nil {
		return nil
	}
	out := make(map[string]pb.FieldType, len(in))
	for path, ft := range in {
		out[path] = ft.ToProto()
	}
	return out
}

// aggregatedFailureError joins every failed-rule message into a single
// error. The shape mirrors LintValidations' aggregation contract — one
// multi-line message that gRPC surfaces as InvalidArgument.
func aggregatedFailureError(failed []celpkg.FailedRule) error {
	msgs := make([]error, 0, len(failed))
	for _, f := range failed {
		msgs = append(msgs, fmt.Errorf("validations[%d]: %s", f.Index, f.Message))
	}
	return errors.Join(msgs...)
}

// fieldTypeMap returns a map of field path -> domain field type for a tenant's schema.
// Returns (nil, nil) if validators are not configured (all fields treated as STRING).
// Returns an error if the validator store is unavailable — callers must propagate
// this error to avoid returning data with silently wrong types.
func (s *Service) fieldTypeMap(ctx context.Context, tenantID string) (map[string]domain.FieldType, error) {
	if s.validators == nil {
		return nil, nil
	}
	validators, err := s.validators.GetValidators(ctx, tenantID)
	if err != nil {
		s.logger.ErrorContext(ctx, "validator lookup failed for field type resolution",
			"tenant_id", tenantID, "error", err)
		return nil, status.Error(codes.Internal, "validator lookup failed")
	}
	m := make(map[string]domain.FieldType, len(validators))
	for path, v := range validators {
		m[path] = v.DomainFieldType()
	}
	s.logger.DebugContext(ctx, "resolved field types for tenant", "tenant", tenantID, "fields", len(m))
	return m, nil
}

// lookupFieldType returns the field type from a type map, defaulting to STRING.
func lookupFieldType(types map[string]domain.FieldType, fieldPath string) domain.FieldType {
	if types != nil {
		if ft, ok := types[fieldPath]; ok {
			return ft
		}
	}
	return domain.FieldTypeString
}

func (s *Service) publishChange(ctx context.Context, tenantID string, version int32, fieldPath, oldValue, newValue, actor string) {
	event := pubsub.ConfigChangeEvent{
		TenantID:  tenantID,
		Version:   version,
		FieldPath: fieldPath,
		OldValue:  oldValue,
		NewValue:  newValue,
		ChangedBy: actor,
		ChangedAt: time.Now(),
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		s.logger.WarnContext(ctx, "failed to publish change event", "error", err)
	}
}
