package schema

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/authz"
	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/telemetry"
	"github.com/opendecree/decree/internal/validation"
)

// resolveSchema looks up a schema by UUID or name slug.
func (s *Service) resolveSchema(ctx context.Context, idOrName string) (domain.Schema, error) {
	if domain.IsUUID(idOrName) {
		return s.store.GetSchemaByID(ctx, idOrName)
	}
	return s.store.GetSchemaByName(ctx, idOrName)
}

// resolveTenant looks up a tenant by UUID or name slug.
func (s *Service) resolveTenant(ctx context.Context, idOrName string) (domain.Tenant, error) {
	if domain.IsUUID(idOrName) {
		return s.store.GetTenantByID(ctx, idOrName)
	}
	return s.store.GetTenantByName(ctx, idOrName)
}

// resolveTenantWithAccess resolves a tenant by UUID or slug, then checks
// that the caller has access to it. Returns the resolved tenant or a gRPC
// status error. Use this at the top of any tenant-scoped RPC handler.
func (s *Service) resolveTenantWithAccess(ctx context.Context, idOrName string) (domain.Tenant, error) {
	if idOrName == "" {
		return domain.Tenant{}, status.Error(codes.InvalidArgument, "tenant id or name required")
	}
	tenant, err := s.resolveTenant(ctx, idOrName)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Tenant{}, status.Error(codes.NotFound, "tenant not found")
		}
		return domain.Tenant{}, status.Error(codes.Internal, "failed to resolve tenant")
	}
	if err := s.guard.Check(ctx, authz.ActionRead, authz.Resource{TenantID: tenant.ID}); err != nil {
		return domain.Tenant{}, err
	}
	return tenant, nil
}

// errToStatus maps a domain store error to a gRPC status error.
func errToStatus(err error, notFoundMsg, failedMsg string) error {
	if errors.Is(err, domain.ErrNotFound) {
		return status.Error(codes.NotFound, notFoundMsg)
	}
	return status.Error(codes.Internal, failedMsg)
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// Option configures a Service.
type Option func(*serviceOptions)

type serviceOptions struct {
	logger    *slog.Logger
	metrics   *telemetry.SchemaMetrics
	validator *validation.ValidatorFactory
	limits    Limits
	guard     authz.Guard
}

// WithLogger sets the service logger. Defaults to slog.Default() when unset.
func WithLogger(l *slog.Logger) Option {
	return func(o *serviceOptions) { o.logger = l }
}

// WithMetrics wires schema metrics. Nil disables them.
func WithMetrics(m *telemetry.SchemaMetrics) Option {
	return func(o *serviceOptions) { o.metrics = m }
}

// WithValidators wires the schema validator factory. Production callers
// should pass the same factory the config service uses so cache
// invalidation is observed by both. Nil is acceptable for tests that do
// not exercise tenant updates.
func WithValidators(v *validation.ValidatorFactory) Option {
	return func(o *serviceOptions) { o.validator = v }
}

// WithLimits caps schema document size and field count. Zero fields mean
// no limit for that dimension. Defaults to [DefaultLimits] when unset.
func WithLimits(l Limits) Option {
	return func(o *serviceOptions) { o.limits = l }
}

// WithGuard overrides the default authorization guard chain.
func WithGuard(g authz.Guard) Option {
	return func(o *serviceOptions) { o.guard = g }
}

// Service implements the SchemaService gRPC server.
type Service struct {
	pb.UnimplementedSchemaServiceServer
	store     Store
	logger    *slog.Logger
	metrics   *telemetry.SchemaMetrics
	validator *validation.ValidatorFactory
	limits    Limits
	guard     authz.Guard
}

// NewService creates a new SchemaService. Only the store is required;
// everything else is optional and may be passed via With...() options.
func NewService(store Store, opts ...Option) *Service {
	o := serviceOptions{
		logger: slog.Default(),
		limits: DefaultLimits(),
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.guard == nil {
		o.guard = authz.Chain(authz.TenantScopeGuard{}, authz.RolePolicyGuard{})
	}
	return &Service{
		store:     store,
		logger:    o.logger,
		metrics:   o.metrics,
		validator: o.validator,
		limits:    o.limits,
		guard:     o.guard,
	}
}

// --- Schema operations ---

func (s *Service) CreateSchema(ctx context.Context, req *pb.CreateSchemaRequest) (*pb.CreateSchemaResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionAdmin, authz.Resource{}); err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !isValidSlug(req.Name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
	}
	if s.limits.MaxFields > 0 && len(req.Fields) > s.limits.MaxFields {
		return nil, status.Errorf(codes.InvalidArgument, "schema has %d fields, exceeds limit of %d", len(req.Fields), s.limits.MaxFields)
	}

	actor := s.getActor(ctx)
	checksum := computeChecksum(req.Fields)

	var sc domain.Schema
	var version domain.SchemaVersion
	var fields []domain.SchemaField

	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var err error
		sc, err = tx.CreateSchema(ctx, CreateSchemaParams{
			Name:        req.Name,
			Description: ptrString(req.GetDescription()),
		})
		if err != nil {
			return err
		}

		version, err = tx.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID: sc.ID,
			Version:  1,
			Checksum: checksum,
		})
		if err != nil {
			return err
		}

		fields, err = createFieldsOn(ctx, s.logger, tx, version.ID, req.Fields)
		if err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]string{"name": req.Name})
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			Actor:      actor,
			Action:     "create_schema",
			ObjectKind: "schema",
			NewValue:   ptrString(sc.ID),
			Metadata:   meta,
		})
	}); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "create schema transaction failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema")
	}

	return &pb.CreateSchemaResponse{
		Schema: schemaToProto(sc, version, fields),
	}, nil
}

func (s *Service) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "schema id or name required")
	}

	schema, err := s.resolveSchema(ctx, req.Id)
	if err != nil {
		return nil, errToStatus(err, "schema not found", "failed to get schema")
	}

	var version domain.SchemaVersion
	if req.Version != nil {
		version, err = s.store.GetSchemaVersion(ctx, GetSchemaVersionParams{
			SchemaID: schema.ID,
			Version:  *req.Version,
		})
	} else {
		version, err = s.store.GetLatestSchemaVersion(ctx, schema.ID)
	}
	if err != nil {
		return nil, errToStatus(err, "schema version not found", "failed to get schema version")
	}

	fields, err := s.store.GetSchemaFields(ctx, version.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get schema fields")
	}

	return &pb.GetSchemaResponse{
		Schema: schemaToProto(schema, version, fields),
	}, nil
}

func (s *Service) ListSchemas(ctx context.Context, req *pb.ListSchemasRequest) (*pb.ListSchemasResponse, error) {
	pageSize := pagination.ClampPageSize(req.PageSize, 50, 100)

	offset, err := pagination.DecodePageToken(req.PageToken)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page token")
	}

	schemas, err := s.store.ListSchemas(ctx, ListSchemasParams{
		Limit:  pageSize + 1,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list schemas")
	}

	nextToken := pagination.NextPageToken(pageSize, int32(len(schemas)), offset)
	if int32(len(schemas)) > pageSize {
		schemas = schemas[:pageSize]
	}

	// Fetch latest version for each schema.
	pbSchemas := make([]*pb.Schema, 0, len(schemas))
	for _, schema := range schemas {
		version, err := s.store.GetLatestSchemaVersion(ctx, schema.ID)
		if err != nil {
			continue // Schema with no versions — skip.
		}
		fields, err := s.store.GetSchemaFields(ctx, version.ID)
		if err != nil {
			continue
		}
		pbSchemas = append(pbSchemas, schemaToProto(schema, version, fields))
	}

	return &pb.ListSchemasResponse{
		Schemas:       pbSchemas,
		NextPageToken: nextToken,
	}, nil
}

func (s *Service) UpdateSchema(ctx context.Context, req *pb.UpdateSchemaRequest) (*pb.UpdateSchemaResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionAdmin, authz.Resource{}); err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "schema id or name required")
	}

	schema, err := s.resolveSchema(ctx, req.Id)
	if err != nil {
		return nil, errToStatus(err, "schema not found", "failed to get schema")
	}

	// Get latest version to derive from.
	latestVersion, err := s.store.GetLatestSchemaVersion(ctx, schema.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get latest version")
	}

	// Get existing fields.
	existingFields, err := s.store.GetSchemaFields(ctx, latestVersion.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get fields")
	}

	// Merge: start with existing, apply updates, remove deletions.
	fieldMap := make(map[string]*pb.SchemaField)
	for _, f := range existingFields {
		fieldMap[f.Path] = fieldToProto(f)
	}
	for _, path := range req.RemoveFields {
		delete(fieldMap, path)
	}
	for _, f := range req.Fields {
		fieldMap[f.Path] = f
	}

	mergedFields := make([]*pb.SchemaField, 0, len(fieldMap))
	for _, f := range fieldMap {
		mergedFields = append(mergedFields, f)
	}

	actor := s.getActor(ctx)
	checksum := computeChecksum(mergedFields)

	var newVersion domain.SchemaVersion
	var fields []domain.SchemaField

	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var err error
		newVersion, err = tx.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
			SchemaID:      schema.ID,
			Version:       latestVersion.Version + 1,
			ParentVersion: &latestVersion.Version,
			Description:   ptrString(req.GetVersionDescription()),
			Checksum:      checksum,
		})
		if err != nil {
			return err
		}

		fields, err = createFieldsOn(ctx, s.logger, tx, newVersion.ID, mergedFields)
		if err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{"schema_id": schema.ID, "version": newVersion.Version})
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			Actor:      actor,
			Action:     "update_schema",
			ObjectKind: "schema",
			NewValue:   ptrString(schema.ID),
			Metadata:   meta,
		})
	}); err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
			return nil, err
		}
		s.logger.ErrorContext(ctx, "update schema transaction failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to update schema")
	}

	return &pb.UpdateSchemaResponse{
		Schema: schemaToProto(schema, newVersion, fields),
	}, nil
}

func (s *Service) DeleteSchema(ctx context.Context, req *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionAdmin, authz.Resource{}); err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "schema id or name required")
	}
	schema, err := s.resolveSchema(ctx, req.Id)
	if err != nil {
		return nil, errToStatus(err, "schema not found", "failed to resolve schema")
	}

	actor := s.getActor(ctx)
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		if err := tx.DeleteSchema(ctx, schema.ID); err != nil {
			return err
		}
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			Actor:      actor,
			Action:     "delete_schema",
			ObjectKind: "schema",
			OldValue:   ptrString(schema.ID),
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "delete schema", "error", err)
		return nil, status.Error(codes.Internal, "failed to delete schema")
	}

	return &pb.DeleteSchemaResponse{}, nil
}

func (s *Service) PublishSchema(ctx context.Context, req *pb.PublishSchemaRequest) (*pb.PublishSchemaResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionAdmin, authz.Resource{}); err != nil {
		return nil, err
	}
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "schema id or name required")
	}

	schema, err := s.resolveSchema(ctx, req.Id)
	if err != nil {
		return nil, errToStatus(err, "schema not found", "failed to get schema")
	}

	actor := s.getActor(ctx)
	var version domain.SchemaVersion
	var fields []domain.SchemaField

	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var err error
		version, err = tx.PublishSchemaVersion(ctx, PublishSchemaVersionParams{
			SchemaID: schema.ID,
			Version:  req.Version,
		})
		if err != nil {
			return err
		}
		fields, err = tx.GetSchemaFields(ctx, version.ID)
		if err != nil {
			return err
		}
		meta, _ := json.Marshal(map[string]any{"schema_id": schema.ID, "version": req.Version})
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			Actor:      actor,
			Action:     "publish_schema",
			ObjectKind: "schema",
			NewValue:   ptrString(schema.ID),
			Metadata:   meta,
		})
	}); err != nil {
		return nil, errToStatus(err, "schema version not found", "failed to publish schema version")
	}

	s.metrics.RecordPublish(ctx)

	return &pb.PublishSchemaResponse{
		Schema: schemaToProto(schema, version, fields),
	}, nil
}

// --- Tenant operations ---

func (s *Service) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionAdmin, authz.Resource{}); err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !isValidSlug(req.Name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
	}

	if !domain.IsUUID(req.SchemaId) {
		return nil, status.Error(codes.InvalidArgument, "invalid schema id")
	}

	// Verify schema version exists and is published.
	version, err := s.store.GetSchemaVersion(ctx, GetSchemaVersionParams{
		SchemaID: req.SchemaId,
		Version:  req.SchemaVersion,
	})
	if err != nil {
		return nil, errToStatus(err, "schema version not found", "failed to get schema version")
	}
	if !version.Published {
		return nil, status.Error(codes.FailedPrecondition, "schema version must be published before assigning to a tenant")
	}

	actor := s.getActor(ctx)
	var tenant domain.Tenant

	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var err error
		tenant, err = tx.CreateTenant(ctx, CreateTenantParams{
			Name:          req.Name,
			SchemaID:      req.SchemaId,
			SchemaVersion: req.SchemaVersion,
		})
		if err != nil {
			return err
		}
		meta, _ := json.Marshal(map[string]any{"name": req.Name, "schema_id": req.SchemaId, "schema_version": req.SchemaVersion})
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenant.ID,
			Actor:      actor,
			Action:     "create_tenant",
			ObjectKind: "tenant",
			NewValue:   ptrString(tenant.ID),
			Metadata:   meta,
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "create tenant", "error", err)
		return nil, status.Error(codes.Internal, "failed to create tenant")
	}

	return &pb.CreateTenantResponse{
		Tenant: tenantToProto(tenant),
	}, nil
}

func (s *Service) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.GetTenantResponse, error) {
	tenant, err := s.resolveTenantWithAccess(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.GetTenantResponse{Tenant: tenantToProto(tenant)}, nil
}

func (s *Service) ListTenants(ctx context.Context, req *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
	pageSize := pagination.ClampPageSize(req.PageSize, 50, 500)

	offset, err := pagination.DecodePageToken(req.PageToken)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page token")
	}

	// Push tenant access filtering into the store so pagination is correct.
	allowedIDs := auth.AllowedTenantIDs(ctx)

	var tenants []domain.Tenant

	if req.SchemaId != nil && *req.SchemaId != "" {
		if !domain.IsUUID(*req.SchemaId) {
			return nil, status.Error(codes.InvalidArgument, "invalid schema id")
		}
		tenants, err = s.store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
			SchemaID:         *req.SchemaId,
			Limit:            pageSize + 1,
			Offset:           offset,
			AllowedTenantIDs: allowedIDs,
		})
	} else {
		tenants, err = s.store.ListTenants(ctx, ListTenantsParams{
			Limit:            pageSize + 1,
			Offset:           offset,
			AllowedTenantIDs: allowedIDs,
		})
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list tenants")
	}

	nextToken := pagination.NextPageToken(pageSize, int32(len(tenants)), offset)
	if int32(len(tenants)) > pageSize {
		tenants = tenants[:pageSize]
	}

	pbTenants := make([]*pb.Tenant, 0, len(tenants))
	for _, t := range tenants {
		pbTenants = append(pbTenants, tenantToProto(t))
	}

	return &pb.ListTenantsResponse{
		Tenants:       pbTenants,
		NextPageToken: nextToken,
	}, nil
}

func (s *Service) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{}); err != nil {
		return nil, err
	}
	resolved, err := s.resolveTenantWithAccess(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	tenantID := resolved.ID

	actor := s.getActor(ctx)
	var tenant domain.Tenant

	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var err error
		if req.Name != nil && *req.Name != "" {
			if !isValidSlug(*req.Name) {
				return status.Error(codes.InvalidArgument, "name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
			}
			tenant, err = tx.UpdateTenantName(ctx, UpdateTenantNameParams{
				ID:   tenantID,
				Name: *req.Name,
			})
			if err != nil {
				return errToStatus(err, "tenant not found", "failed to update tenant name")
			}
		}

		if req.SchemaVersion != nil {
			tenant, err = tx.UpdateTenantSchemaVersion(ctx, UpdateTenantSchemaVersionParams{
				ID:            tenantID,
				SchemaVersion: *req.SchemaVersion,
			})
			if err != nil {
				return errToStatus(err, "tenant not found", "failed to update tenant schema version")
			}
		}

		if req.Name == nil && req.SchemaVersion == nil {
			tenant, err = tx.GetTenantByID(ctx, tenantID)
			if err != nil {
				return errToStatus(err, "tenant not found", "failed to get tenant")
			}
		}

		meta, _ := json.Marshal(map[string]any{"name": req.Name, "schema_version": req.SchemaVersion})
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenantID,
			Actor:      actor,
			Action:     "update_tenant",
			ObjectKind: "tenant",
			NewValue:   ptrString(tenantID),
			Metadata:   meta,
		})
	}); err != nil {
		return nil, err
	}

	// Invalidate cached validators if schema version changed.
	if req.SchemaVersion != nil && s.validator != nil {
		s.validator.Cache().Invalidate(tenantID)
		s.validator.InvalidateRules(tenantID)
	}

	return &pb.UpdateTenantResponse{
		Tenant: tenantToProto(tenant),
	}, nil
}

func (s *Service) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{}); err != nil {
		return nil, err
	}
	tenant, err := s.resolveTenantWithAccess(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	actor := s.getActor(ctx)
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		if err := tx.DeleteTenant(ctx, tenant.ID); err != nil {
			return err
		}
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenant.ID,
			Actor:      actor,
			Action:     "delete_tenant",
			ObjectKind: "tenant",
			OldValue:   ptrString(tenant.ID),
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "delete tenant", "error", err)
		return nil, status.Error(codes.Internal, "failed to delete tenant")
	}

	return &pb.DeleteTenantResponse{}, nil
}

// --- Field locking ---

func (s *Service) LockField(ctx context.Context, req *pb.LockFieldRequest) (*pb.LockFieldResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{}); err != nil {
		return nil, err
	}
	tenant, err := s.resolveTenantWithAccess(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	var lockedValues []byte
	if len(req.LockedValues) > 0 {
		lockedValues, _ = json.Marshal(req.LockedValues)
	}

	actor := s.getActor(ctx)
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		if err := tx.CreateFieldLock(ctx, CreateFieldLockParams{
			TenantID:     tenant.ID,
			FieldPath:    req.FieldPath,
			LockedValues: lockedValues,
		}); err != nil {
			return err
		}
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenant.ID,
			Actor:      actor,
			Action:     "lock_field",
			ObjectKind: "lock",
			FieldPath:  ptrString(req.FieldPath),
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "lock field", "error", err)
		return nil, status.Error(codes.Internal, "failed to lock field")
	}

	return &pb.LockFieldResponse{}, nil
}

func (s *Service) UnlockField(ctx context.Context, req *pb.UnlockFieldRequest) (*pb.UnlockFieldResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionWrite, authz.Resource{}); err != nil {
		return nil, err
	}
	tenant, err := s.resolveTenantWithAccess(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	actor := s.getActor(ctx)
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		if err := tx.DeleteFieldLock(ctx, DeleteFieldLockParams{
			TenantID:  tenant.ID,
			FieldPath: req.FieldPath,
		}); err != nil {
			return err
		}
		return tx.InsertAuditWriteLog(ctx, InsertAuditWriteLogParams{
			TenantID:   tenant.ID,
			Actor:      actor,
			Action:     "unlock_field",
			ObjectKind: "lock",
			FieldPath:  ptrString(req.FieldPath),
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "unlock field", "error", err)
		return nil, status.Error(codes.Internal, "failed to unlock field")
	}

	return &pb.UnlockFieldResponse{}, nil
}

func (s *Service) ListFieldLocks(ctx context.Context, req *pb.ListFieldLocksRequest) (*pb.ListFieldLocksResponse, error) {
	tenant, err := s.resolveTenantWithAccess(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	locks, err := s.store.GetFieldLocks(ctx, tenant.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list field locks")
	}

	pbLocks := make([]*pb.FieldLock, 0, len(locks))
	for _, l := range locks {
		pbLocks = append(pbLocks, fieldLockToProto(l))
	}

	return &pb.ListFieldLocksResponse{
		Locks: pbLocks,
	}, nil
}

// --- Import/export ---

func (s *Service) ExportSchema(ctx context.Context, req *pb.ExportSchemaRequest) (*pb.ExportSchemaResponse, error) {
	// Load the schema via GetSchema to reuse version resolution.
	getResp, err := s.GetSchema(ctx, &pb.GetSchemaRequest{
		Id:      req.Id,
		Version: req.Version,
	})
	if err != nil {
		return nil, err // Already a gRPC status error.
	}
	if getResp == nil || getResp.Schema == nil {
		return nil, status.Error(codes.Internal, "unexpected nil schema response")
	}

	specVersion := ""
	if req.SpecVersion != nil {
		specVersion = *req.SpecVersion
	}
	data, err := MarshalSchemaAt(getResp.Schema, specVersion)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	return &pb.ExportSchemaResponse{YamlContent: data}, nil
}

func (s *Service) ImportSchema(ctx context.Context, req *pb.ImportSchemaRequest) (*pb.ImportSchemaResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	if err := s.guard.Check(ctx, authz.ActionAdmin, authz.Resource{}); err != nil {
		return nil, err
	}
	if s.limits.MaxDocBytes > 0 && len(req.YamlContent) > s.limits.MaxDocBytes {
		return nil, status.Errorf(codes.InvalidArgument, "schema document is %d bytes, exceeds limit of %d", len(req.YamlContent), s.limits.MaxDocBytes)
	}
	parsed, err := Dispatch(req.YamlContent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid schema YAML: %v", err)
	}

	fields := parsed.Fields
	if s.limits.MaxFields > 0 && len(fields) > s.limits.MaxFields {
		return nil, status.Errorf(codes.InvalidArgument, "schema has %d fields, exceeds limit of %d", len(fields), s.limits.MaxFields)
	}
	// Validate field constraints (including regex compilation) before any
	// storage write so that bad patterns are caught immediately.
	if err := validateFieldConstraintsBatch(fields); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	depReqs := parsed.DependentRequired
	if err := validateDependentRequiredAgainstFields(depReqs, fields); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	depReqJSON, err := marshalDependentRequired(depReqs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to encode dependentRequired: %v", err)
	}
	validationsJSON, err := MarshalValidations(parsed.Validations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to encode validations: %v", err)
	}
	checksum := computeChecksum(fields)

	// Check if schema already exists by name.
	existing, err := s.store.GetSchemaByName(ctx, parsed.Name)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, status.Error(codes.Internal, "failed to look up schema")
	}

	if errors.Is(err, domain.ErrNotFound) {
		// New schema — create with v1.
		resp, err := s.importCreateNew(ctx, parsed, fields, checksum, depReqJSON, validationsJSON)
		if err != nil || !req.AutoPublish {
			return resp, err
		}
		return s.autoPublish(ctx, resp)
	}

	// Existing schema — check if identical to latest version.
	latestVersion, err := s.store.GetLatestSchemaVersion(ctx, existing.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get latest version")
	}

	if latestVersion.Checksum == checksum {
		// No changes — return existing schema.
		existingFields, err := s.store.GetSchemaFields(ctx, latestVersion.ID)
		if err != nil {
			return nil, status.Error(codes.Internal, "failed to get fields")
		}
		return &pb.ImportSchemaResponse{
			Schema: schemaToProto(existing, latestVersion, existingFields),
		}, status.Error(codes.AlreadyExists, "schema is identical to the latest version")
	}

	// Create new version.
	resp, err := s.importNewVersion(ctx, existing, latestVersion, parsed, fields, checksum, depReqJSON, validationsJSON)
	if err != nil || !req.AutoPublish {
		return resp, err
	}
	return s.autoPublish(ctx, resp)
}

func (s *Service) importCreateNew(ctx context.Context, parsed *pb.Schema, fields []*pb.SchemaField, checksum string, depReqJSON, validationsJSON []byte) (*pb.ImportSchemaResponse, error) {
	schema, err := s.store.CreateSchema(ctx, CreateSchemaParams{
		Name:        parsed.Name,
		Description: ptrString(parsed.Description),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "import: create schema", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema")
	}

	version, err := s.store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID:          schema.ID,
		Version:           1,
		Description:       ptrString(parsed.VersionDescription),
		Checksum:          checksum,
		DependentRequired: depReqJSON,
		Validations:       validationsJSON,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "import: create version", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema version")
	}

	dbFields, err := s.createFields(ctx, version.ID, fields)
	if err != nil {
		return nil, err
	}

	return &pb.ImportSchemaResponse{
		Schema: schemaToProto(schema, version, dbFields),
	}, nil
}

func (s *Service) importNewVersion(ctx context.Context, schema domain.Schema, latestVersion domain.SchemaVersion, parsed *pb.Schema, fields []*pb.SchemaField, checksum string, depReqJSON, validationsJSON []byte) (*pb.ImportSchemaResponse, error) {
	newVersion, err := s.store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID:          schema.ID,
		Version:           latestVersion.Version + 1,
		ParentVersion:     &latestVersion.Version,
		Description:       ptrString(parsed.VersionDescription),
		Checksum:          checksum,
		DependentRequired: depReqJSON,
		Validations:       validationsJSON,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "import: create new version", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema version")
	}

	dbFields, err := s.createFields(ctx, newVersion.ID, fields)
	if err != nil {
		return nil, err
	}

	return &pb.ImportSchemaResponse{
		Schema: schemaToProto(schema, newVersion, dbFields),
	}, nil
}

// --- Helpers ---

func (s *Service) autoPublish(ctx context.Context, resp *pb.ImportSchemaResponse) (*pb.ImportSchemaResponse, error) {
	schema := resp.Schema
	pubResp, err := s.PublishSchema(ctx, &pb.PublishSchemaRequest{
		Id:      schema.Id,
		Version: schema.Version,
	})
	if err != nil {
		return nil, err
	}
	return &pb.ImportSchemaResponse{Schema: pubResp.Schema}, nil
}

func (s *Service) getActor(ctx context.Context) string {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok {
		return "unknown"
	}
	return claims.Subject
}

func (s *Service) createFields(ctx context.Context, versionID string, fields []*pb.SchemaField) ([]domain.SchemaField, error) {
	return createFieldsOn(ctx, s.logger, s.store, versionID, fields)
}

func createFieldsOn(ctx context.Context, logger *slog.Logger, store Store, versionID string, fields []*pb.SchemaField) ([]domain.SchemaField, error) {
	if err := validateNoPrefixOverlap(fields); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	result := make([]domain.SchemaField, 0, len(fields))
	for _, f := range fields {
		if err := validateFieldConstraints(f); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		}

		var constraints []byte
		if f.Constraints != nil {
			constraints, _ = json.Marshal(f.Constraints)
		}
		var examples []byte
		if len(f.Examples) > 0 {
			examples, _ = json.Marshal(f.Examples)
		}
		var externalDocs []byte
		if f.ExternalDocs != nil {
			externalDocs, _ = json.Marshal(f.ExternalDocs)
		}

		dbField, err := store.CreateSchemaField(ctx, CreateSchemaFieldParams{
			SchemaVersionID: versionID,
			Path:            f.Path,
			FieldType:       domain.FieldTypeFromProto(f.Type),
			Constraints:     constraints,
			Nullable:        f.Nullable,
			Deprecated:      f.Deprecated,
			RedirectTo:      f.RedirectTo,
			DefaultValue:    f.DefaultValue,
			Description:     f.Description,
			Title:           f.Title,
			Example:         f.Example,
			Examples:        examples,
			ExternalDocs:    externalDocs,
			Tags:            f.Tags,
			Format:          f.Format,
			ReadOnly:        f.ReadOnly,
			WriteOnce:       f.WriteOnce,
			Sensitive:       f.Sensitive,
		})
		if err != nil {
			logger.ErrorContext(ctx, "create schema field", "path", f.Path, "error", err)
			return nil, status.Errorf(codes.Internal, "failed to create field %s", f.Path)
		}
		result = append(result, dbField)
	}
	return result, nil
}
