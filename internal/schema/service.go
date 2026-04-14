package schema

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/storage/domain"
	"github.com/opendecree/decree/internal/telemetry"
	"github.com/opendecree/decree/internal/validation"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// validUUID checks whether s is a valid UUID string.
func validUUID(s string) bool {
	return uuidRe.MatchString(s)
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// Service implements the SchemaService gRPC server.
type Service struct {
	pb.UnimplementedSchemaServiceServer
	store          Store
	logger         *slog.Logger
	metrics        *telemetry.SchemaMetrics
	validatorCache *validation.ValidatorCache
}

// NewService creates a new SchemaService.
func NewService(store Store, logger *slog.Logger, metrics *telemetry.SchemaMetrics, validatorCache *validation.ValidatorCache) *Service {
	return &Service{
		store:          store,
		logger:         logger,
		metrics:        metrics,
		validatorCache: validatorCache,
	}
}

// --- Schema operations ---

func (s *Service) CreateSchema(ctx context.Context, req *pb.CreateSchemaRequest) (*pb.CreateSchemaResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !isValidSlug(req.Name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
	}

	schema, err := s.store.CreateSchema(ctx, CreateSchemaParams{
		Name:        req.Name,
		Description: ptrString(req.GetDescription()),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "create schema", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema")
	}

	// Create initial version (v1).
	checksum := computeChecksum(req.Fields)
	version, err := s.store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID: schema.ID,
		Version:  1,
		Checksum: checksum,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "create schema version", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema version")
	}

	// Create fields.
	fields, err := s.createFields(ctx, version.ID, req.Fields)
	if err != nil {
		return nil, err
	}

	return &pb.CreateSchemaResponse{
		Schema: schemaToProto(schema, version, fields),
	}, nil
}

func (s *Service) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (*pb.GetSchemaResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid schema id")
	}

	schema, err := s.store.GetSchemaByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schema not found")
		}
		return nil, status.Error(codes.Internal, "failed to get schema")
	}

	var version domain.SchemaVersion
	if req.Version != nil {
		version, err = s.store.GetSchemaVersion(ctx, GetSchemaVersionParams{
			SchemaID: req.Id,
			Version:  *req.Version,
		})
	} else {
		version, err = s.store.GetLatestSchemaVersion(ctx, req.Id)
	}
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schema version not found")
		}
		return nil, status.Error(codes.Internal, "failed to get schema version")
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
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	var offset int32
	// TODO: implement cursor-based pagination using req.PageToken.

	schemas, err := s.store.ListSchemas(ctx, ListSchemasParams{
		Limit:  pageSize,
		Offset: offset,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list schemas")
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
		Schemas: pbSchemas,
	}, nil
}

func (s *Service) UpdateSchema(ctx context.Context, req *pb.UpdateSchemaRequest) (*pb.UpdateSchemaResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid schema id")
	}

	schema, err := s.store.GetSchemaByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schema not found")
		}
		return nil, status.Error(codes.Internal, "failed to get schema")
	}

	// Get latest version to derive from.
	latestVersion, err := s.store.GetLatestSchemaVersion(ctx, req.Id)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get latest version")
	}
	// Published versions are immutable — always create a new version regardless.

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

	// Collect merged fields.
	mergedFields := make([]*pb.SchemaField, 0, len(fieldMap))
	for _, f := range fieldMap {
		mergedFields = append(mergedFields, f)
	}

	checksum := computeChecksum(mergedFields)
	newVersion, err := s.store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID:      req.Id,
		Version:       latestVersion.Version + 1,
		ParentVersion: &latestVersion.Version,
		Description:   ptrString(req.GetVersionDescription()),
		Checksum:      checksum,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "create schema version", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema version")
	}

	fields, err := s.createFields(ctx, newVersion.ID, mergedFields)
	if err != nil {
		return nil, err
	}

	return &pb.UpdateSchemaResponse{
		Schema: schemaToProto(schema, newVersion, fields),
	}, nil
}

func (s *Service) DeleteSchema(ctx context.Context, req *pb.DeleteSchemaRequest) (*pb.DeleteSchemaResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid schema id")
	}

	if err := s.store.DeleteSchema(ctx, req.Id); err != nil {
		s.logger.ErrorContext(ctx, "delete schema", "error", err)
		return nil, status.Error(codes.Internal, "failed to delete schema")
	}

	return &pb.DeleteSchemaResponse{}, nil
}

func (s *Service) PublishSchema(ctx context.Context, req *pb.PublishSchemaRequest) (*pb.PublishSchemaResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid schema id")
	}

	schema, err := s.store.GetSchemaByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schema not found")
		}
		return nil, status.Error(codes.Internal, "failed to get schema")
	}

	version, err := s.store.PublishSchemaVersion(ctx, PublishSchemaVersionParams{
		SchemaID: req.Id,
		Version:  req.Version,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schema version not found")
		}
		return nil, status.Error(codes.Internal, "failed to publish schema version")
	}

	fields, err := s.store.GetSchemaFields(ctx, version.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get fields")
	}

	s.metrics.RecordPublish(ctx)

	return &pb.PublishSchemaResponse{
		Schema: schemaToProto(schema, version, fields),
	}, nil
}

// --- Tenant operations ---

func (s *Service) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.CreateTenantResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if !isValidSlug(req.Name) {
		return nil, status.Error(codes.InvalidArgument, "name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
	}

	if !validUUID(req.SchemaId) {
		return nil, status.Error(codes.InvalidArgument, "invalid schema id")
	}

	// Verify schema version exists and is published.
	version, err := s.store.GetSchemaVersion(ctx, GetSchemaVersionParams{
		SchemaID: req.SchemaId,
		Version:  req.SchemaVersion,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "schema version not found")
		}
		return nil, status.Error(codes.Internal, "failed to get schema version")
	}
	if !version.Published {
		return nil, status.Error(codes.FailedPrecondition, "schema version must be published before assigning to a tenant")
	}

	tenant, err := s.store.CreateTenant(ctx, CreateTenantParams{
		Name:          req.Name,
		SchemaID:      req.SchemaId,
		SchemaVersion: req.SchemaVersion,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "create tenant", "error", err)
		return nil, status.Error(codes.Internal, "failed to create tenant")
	}

	return &pb.CreateTenantResponse{
		Tenant: tenantToProto(tenant),
	}, nil
}

func (s *Service) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.GetTenantResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}
	if err := auth.CheckTenantAccess(ctx, req.Id); err != nil {
		return nil, err
	}

	tenant, err := s.store.GetTenantByID(ctx, req.Id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "tenant not found")
		}
		return nil, status.Error(codes.Internal, "failed to get tenant")
	}

	return &pb.GetTenantResponse{
		Tenant: tenantToProto(tenant),
	}, nil
}

func (s *Service) ListTenants(ctx context.Context, req *pb.ListTenantsRequest) (*pb.ListTenantsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	// Push tenant access filtering into the store so pagination is correct.
	allowedIDs := auth.AllowedTenantIDs(ctx)

	var tenants []domain.Tenant
	var err error

	if req.SchemaId != nil && *req.SchemaId != "" {
		if !validUUID(*req.SchemaId) {
			return nil, status.Error(codes.InvalidArgument, "invalid schema id")
		}
		tenants, err = s.store.ListTenantsBySchema(ctx, ListTenantsBySchemaParams{
			SchemaID:         *req.SchemaId,
			Limit:            pageSize,
			Offset:           0,
			AllowedTenantIDs: allowedIDs,
		})
	} else {
		tenants, err = s.store.ListTenants(ctx, ListTenantsParams{
			Limit:            pageSize,
			Offset:           0,
			AllowedTenantIDs: allowedIDs,
		})
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list tenants")
	}

	pbTenants := make([]*pb.Tenant, 0, len(tenants))
	for _, t := range tenants {
		pbTenants = append(pbTenants, tenantToProto(t))
	}

	return &pb.ListTenantsResponse{
		Tenants: pbTenants,
	}, nil
}

func (s *Service) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.UpdateTenantResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}
	if err := auth.CheckTenantAccess(ctx, req.Id); err != nil {
		return nil, err
	}

	var tenant domain.Tenant

	if req.Name != nil && *req.Name != "" {
		if !isValidSlug(*req.Name) {
			return nil, status.Error(codes.InvalidArgument, "name must be a slug: lowercase alphanumeric and hyphens, 1-63 chars")
		}
		var err error
		tenant, err = s.store.UpdateTenantName(ctx, UpdateTenantNameParams{
			ID:   req.Id,
			Name: *req.Name,
		})
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, status.Error(codes.NotFound, "tenant not found")
			}
			return nil, status.Error(codes.Internal, "failed to update tenant name")
		}
	}

	if req.SchemaVersion != nil {
		var err error
		tenant, err = s.store.UpdateTenantSchemaVersion(ctx, UpdateTenantSchemaVersionParams{
			ID:            req.Id,
			SchemaVersion: *req.SchemaVersion,
		})
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, status.Error(codes.NotFound, "tenant not found")
			}
			return nil, status.Error(codes.Internal, "failed to update tenant schema version")
		}
		// Invalidate cached validators — tenant now uses different field definitions.
		if s.validatorCache != nil {
			s.validatorCache.Invalidate(req.Id)
		}
	}

	// If neither field was updated, just fetch current state.
	if req.Name == nil && req.SchemaVersion == nil {
		var err error
		tenant, err = s.store.GetTenantByID(ctx, req.Id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, status.Error(codes.NotFound, "tenant not found")
			}
			return nil, status.Error(codes.Internal, "failed to get tenant")
		}
	}

	return &pb.UpdateTenantResponse{
		Tenant: tenantToProto(tenant),
	}, nil
}

func (s *Service) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	if !validUUID(req.Id) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}
	if err := auth.CheckTenantAccess(ctx, req.Id); err != nil {
		return nil, err
	}

	if err := s.store.DeleteTenant(ctx, req.Id); err != nil {
		s.logger.ErrorContext(ctx, "delete tenant", "error", err)
		return nil, status.Error(codes.Internal, "failed to delete tenant")
	}

	return &pb.DeleteTenantResponse{}, nil
}

// --- Field locking ---

func (s *Service) LockField(ctx context.Context, req *pb.LockFieldRequest) (*pb.LockFieldResponse, error) {
	if !validUUID(req.TenantId) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	var lockedValues []byte
	if len(req.LockedValues) > 0 {
		lockedValues, _ = json.Marshal(req.LockedValues)
	}

	if err := s.store.CreateFieldLock(ctx, CreateFieldLockParams{
		TenantID:     req.TenantId,
		FieldPath:    req.FieldPath,
		LockedValues: lockedValues,
	}); err != nil {
		s.logger.ErrorContext(ctx, "lock field", "error", err)
		return nil, status.Error(codes.Internal, "failed to lock field")
	}

	return &pb.LockFieldResponse{}, nil
}

func (s *Service) UnlockField(ctx context.Context, req *pb.UnlockFieldRequest) (*pb.UnlockFieldResponse, error) {
	if !validUUID(req.TenantId) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	if err := s.store.DeleteFieldLock(ctx, DeleteFieldLockParams{
		TenantID:  req.TenantId,
		FieldPath: req.FieldPath,
	}); err != nil {
		s.logger.ErrorContext(ctx, "unlock field", "error", err)
		return nil, status.Error(codes.Internal, "failed to unlock field")
	}

	return &pb.UnlockFieldResponse{}, nil
}

func (s *Service) ListFieldLocks(ctx context.Context, req *pb.ListFieldLocksRequest) (*pb.ListFieldLocksResponse, error) {
	if !validUUID(req.TenantId) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	locks, err := s.store.GetFieldLocks(ctx, req.TenantId)
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

	doc := schemaToYAML(getResp.Schema)
	data, err := marshalSchemaYAML(doc)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal schema to YAML")
	}

	return &pb.ExportSchemaResponse{YamlContent: data}, nil
}

func (s *Service) ImportSchema(ctx context.Context, req *pb.ImportSchemaRequest) (*pb.ImportSchemaResponse, error) {
	doc, err := unmarshalSchemaYAML(req.YamlContent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid schema YAML: %v", err)
	}

	fields := yamlToProtoFields(doc)
	checksum := computeChecksum(fields)

	// Check if schema already exists by name.
	existing, err := s.store.GetSchemaByName(ctx, doc.Name)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, status.Error(codes.Internal, "failed to look up schema")
	}

	if errors.Is(err, domain.ErrNotFound) {
		// New schema — create with v1.
		resp, err := s.importCreateNew(ctx, doc, fields, checksum)
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
	resp, err := s.importNewVersion(ctx, existing, latestVersion, doc, fields, checksum)
	if err != nil || !req.AutoPublish {
		return resp, err
	}
	return s.autoPublish(ctx, resp)
}

func (s *Service) importCreateNew(ctx context.Context, doc *SchemaYAML, fields []*pb.SchemaField, checksum string) (*pb.ImportSchemaResponse, error) {
	schema, err := s.store.CreateSchema(ctx, CreateSchemaParams{
		Name:        doc.Name,
		Description: ptrString(doc.Description),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "import: create schema", "error", err)
		return nil, status.Error(codes.Internal, "failed to create schema")
	}

	version, err := s.store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID:    schema.ID,
		Version:     1,
		Description: ptrString(doc.VersionDescription),
		Checksum:    checksum,
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

func (s *Service) importNewVersion(ctx context.Context, schema domain.Schema, latestVersion domain.SchemaVersion, doc *SchemaYAML, fields []*pb.SchemaField, checksum string) (*pb.ImportSchemaResponse, error) {
	newVersion, err := s.store.CreateSchemaVersion(ctx, CreateSchemaVersionParams{
		SchemaID:      schema.ID,
		Version:       latestVersion.Version + 1,
		ParentVersion: &latestVersion.Version,
		Description:   ptrString(doc.VersionDescription),
		Checksum:      checksum,
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

func (s *Service) createFields(ctx context.Context, versionID string, fields []*pb.SchemaField) ([]domain.SchemaField, error) {
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

		dbField, err := s.store.CreateSchemaField(ctx, CreateSchemaFieldParams{
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
			s.logger.ErrorContext(ctx, "create schema field", "path", f.Path, "error", err)
			return nil, status.Errorf(codes.Internal, "failed to create field %s", f.Path)
		}
		result = append(result, dbField)
	}
	return result, nil
}
