package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/auth"
	"github.com/zeevdr/central-config-service/internal/cache"
	"github.com/zeevdr/central-config-service/internal/pubsub"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
	"github.com/zeevdr/central-config-service/internal/telemetry"
	"github.com/zeevdr/central-config-service/internal/validation"
)

const defaultCacheTTL = 5 * time.Minute

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
}

// NewService creates a new ConfigService.
func NewService(store Store, cache cache.ConfigCache, pub pubsub.Publisher, sub pubsub.Subscriber, logger *slog.Logger, cacheMetrics *telemetry.CacheMetrics, configMetrics *telemetry.ConfigMetrics, validators *validation.ValidatorFactory) *Service {
	return &Service{
		store:        store,
		cache:        cache,
		publisher:    pub,
		subscriber:   sub,
		logger:       logger,
		cacheMetrics: cacheMetrics,
		metrics:      configMetrics,
		validators:   validators,
	}
}

// --- Read operations ---

func (s *Service) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	// Resolve version.
	version, err := s.resolveVersion(ctx, tenantID, req.Version)
	if err != nil {
		return nil, err
	}

	// If descriptions not requested, try cache.
	if !req.IncludeDescriptions {
		if cached, err := s.cache.Get(ctx, req.TenantId, version); err == nil && cached != nil {
			s.cacheMetrics.Hit(ctx)
			values := make([]*pb.ConfigValue, 0, len(cached))
			for path, val := range cached {
				v := val
				values = append(values, &pb.ConfigValue{
					FieldPath: path,
					Value:     stringToTypedValue(&v, pb.FieldType_FIELD_TYPE_STRING),
					Checksum:  computeChecksum(val),
				})
			}
			return &pb.GetConfigResponse{
				Config: &pb.Config{TenantId: req.TenantId, Version: version, Values: values},
			}, nil
		}
		s.cacheMetrics.Miss(ctx)
	}

	// Fetch from DB.
	rows, err := s.store.GetFullConfigAtVersion(ctx, dbstore.GetFullConfigAtVersionParams{
		TenantID: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get config")
	}

	values := make([]*pb.ConfigValue, 0, len(rows))
	cacheMap := make(map[string]string, len(rows))
	for _, row := range rows {
		cv := &pb.ConfigValue{
			FieldPath: row.FieldPath,
			Value:     stringToTypedValue(row.Value, pb.FieldType_FIELD_TYPE_STRING),
			Checksum:  derefString(row.Checksum),
		}
		if req.IncludeDescriptions && row.Description != nil {
			cv.Description = row.Description
		}
		values = append(values, cv)
		cacheMap[row.FieldPath] = derefString(row.Value)
	}

	// Populate cache (values only, no descriptions).
	if !req.IncludeDescriptions {
		if err := s.cache.Set(ctx, req.TenantId, version, cacheMap, defaultCacheTTL); err != nil {
			s.logger.WarnContext(ctx, "failed to populate cache", "error", err)
		}
	}

	return &pb.GetConfigResponse{
		Config: &pb.Config{TenantId: req.TenantId, Version: version, Values: values},
	}, nil
}

func (s *Service) GetField(ctx context.Context, req *pb.GetFieldRequest) (*pb.GetFieldResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	version, err := s.resolveVersion(ctx, tenantID, req.Version)
	if err != nil {
		return nil, err
	}

	row, err := s.store.GetConfigValueAtVersion(ctx, dbstore.GetConfigValueAtVersionParams{
		TenantID:  tenantID,
		FieldPath: req.FieldPath,
		Version:   version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "field not found")
		}
		return nil, status.Error(codes.Internal, "failed to get field")
	}

	cv := &pb.ConfigValue{
		FieldPath: row.FieldPath,
		Value:     stringToTypedValue(row.Value, pb.FieldType_FIELD_TYPE_STRING),
		Checksum:  derefString(row.Checksum),
	}
	if req.IncludeDescription && row.Description != nil {
		cv.Description = row.Description
	}

	return &pb.GetFieldResponse{Value: cv}, nil
}

func (s *Service) GetFields(ctx context.Context, req *pb.GetFieldsRequest) (*pb.GetFieldsResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	version, err := s.resolveVersion(ctx, tenantID, req.Version)
	if err != nil {
		return nil, err
	}

	values := make([]*pb.ConfigValue, 0, len(req.FieldPaths))
	for _, path := range req.FieldPaths {
		row, err := s.store.GetConfigValueAtVersion(ctx, dbstore.GetConfigValueAtVersionParams{
			TenantID:  tenantID,
			FieldPath: path,
			Version:   version,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue // Skip missing fields.
			}
			return nil, status.Error(codes.Internal, "failed to get field")
		}
		cv := &pb.ConfigValue{
			FieldPath: row.FieldPath,
			Value:     stringToTypedValue(row.Value, pb.FieldType_FIELD_TYPE_STRING),
			Checksum:  derefString(row.Checksum),
		}
		if req.IncludeDescriptions && row.Description != nil {
			cv.Description = row.Description
		}
		values = append(values, cv)
	}

	return &pb.GetFieldsResponse{Values: values}, nil
}

// --- Write operations ---

func (s *Service) SetField(ctx context.Context, req *pb.SetFieldRequest) (*pb.SetFieldResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	actor := s.getActor(ctx)

	// Pre-transaction validation (reads only).
	if req.ExpectedChecksum != nil {
		if err := s.checkChecksum(ctx, tenantID, req.FieldPath, *req.ExpectedChecksum); err != nil {
			return nil, err
		}
	}
	if err := s.checkFieldLock(ctx, tenantID, req.FieldPath); err != nil {
		return nil, err
	}
	if err := s.validateField(ctx, tenantID, req.TenantId, req.FieldPath, req.Value); err != nil {
		return nil, err
	}

	latestVersion, err := s.getOrCreateVersion(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	oldValue := s.getCurrentValue(ctx, tenantID, req.FieldPath, latestVersion)

	// Transaction: version + value + audit.
	var newVersion dbstore.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		newVersion, txErr = tx.CreateConfigVersion(ctx, dbstore.CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     latestVersion + 1,
			Description: ptrString(req.GetDescription()),
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create config version: %w", txErr)
		}

		valStr := typedValueToString(req.Value)
		if txErr = tx.SetConfigValue(ctx, dbstore.SetConfigValueParams{
			ConfigVersionID: newVersion.ID,
			FieldPath:       req.FieldPath,
			Value:           valStr,
			Checksum:        checksumPtr(valStr),
			Description:     ptrString(req.GetValueDescription()),
		}); txErr != nil {
			return fmt.Errorf("set config value: %w", txErr)
		}

		newValueStr := typedValueToString(req.Value)
		return tx.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
			TenantID:      tenantID,
			Actor:         actor,
			Action:        "set_field",
			FieldPath:     ptrString(req.FieldPath),
			OldValue:      ptrString(oldValue),
			NewValue:      newValueStr,
			ConfigVersion: &newVersion.Version,
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "set field transaction failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to set field")
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, req.TenantId); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}
	s.publishChange(ctx, req.TenantId, newVersion.Version, req.FieldPath, oldValue, typedValueToDisplayString(req.Value), actor)

	s.metrics.RecordWrite(ctx, req.TenantId, "set_field")
	s.metrics.RecordVersion(ctx, req.TenantId, int64(newVersion.Version))

	return &pb.SetFieldResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

func (s *Service) SetFields(ctx context.Context, req *pb.SetFieldsRequest) (*pb.SetFieldsResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	actor := s.getActor(ctx)

	// Pre-transaction validation (reads only).
	for _, update := range req.Updates {
		if update.ExpectedChecksum != nil {
			if err := s.checkChecksum(ctx, tenantID, update.FieldPath, *update.ExpectedChecksum); err != nil {
				return nil, err
			}
		}
		if err := s.checkFieldLock(ctx, tenantID, update.FieldPath); err != nil {
			return nil, err
		}
		if err := s.validateField(ctx, tenantID, req.TenantId, update.FieldPath, update.Value); err != nil {
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

	// Transaction: version + all values + all audit entries.
	var newVersion dbstore.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		newVersion, txErr = tx.CreateConfigVersion(ctx, dbstore.CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     latestVersion + 1,
			Description: ptrString(req.GetDescription()),
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create config version: %w", txErr)
		}

		for i, update := range req.Updates {
			updateValStr := typedValueToString(update.Value)
			if txErr = tx.SetConfigValue(ctx, dbstore.SetConfigValueParams{
				ConfigVersionID: newVersion.ID,
				FieldPath:       update.FieldPath,
				Value:           updateValStr,
				Checksum:        checksumPtr(updateValStr),
				Description:     ptrString(update.GetValueDescription()),
			}); txErr != nil {
				return fmt.Errorf("set config value %s: %w", update.FieldPath, txErr)
			}

			newValueStr := typedValueToString(update.Value)
			if txErr = tx.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
				TenantID:      tenantID,
				Actor:         actor,
				Action:        "set_field",
				FieldPath:     ptrString(update.FieldPath),
				OldValue:      ptrString(changes[i].oldValue),
				NewValue:      newValueStr,
				ConfigVersion: &newVersion.Version,
			}); txErr != nil {
				return fmt.Errorf("insert audit log for %s: %w", update.FieldPath, txErr)
			}
		}

		return nil
	}); err != nil {
		s.logger.ErrorContext(ctx, "set fields transaction failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to set fields")
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, req.TenantId); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}
	for _, ch := range changes {
		s.publishChange(ctx, req.TenantId, newVersion.Version, ch.fieldPath, ch.oldValue, ch.newValue, actor)
	}

	s.metrics.RecordWrite(ctx, req.TenantId, "set_fields")
	s.metrics.RecordVersion(ctx, req.TenantId, int64(newVersion.Version))

	return &pb.SetFieldsResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

// --- Version operations ---

func (s *Service) ListVersions(ctx context.Context, req *pb.ListVersionsRequest) (*pb.ListVersionsResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	versions, err := s.store.ListConfigVersions(ctx, dbstore.ListConfigVersionsParams{
		TenantID: tenantID,
		Limit:    pageSize,
		Offset:   0,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list versions")
	}

	pbVersions := make([]*pb.ConfigVersion, 0, len(versions))
	for _, v := range versions {
		pbVersions = append(pbVersions, configVersionToProto(v))
	}

	return &pb.ListVersionsResponse{Versions: pbVersions}, nil
}

func (s *Service) GetVersion(ctx context.Context, req *pb.GetVersionRequest) (*pb.GetVersionResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	version, err := s.store.GetConfigVersion(ctx, dbstore.GetConfigVersionParams{
		TenantID: tenantID,
		Version:  req.Version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "version not found")
		}
		return nil, status.Error(codes.Internal, "failed to get version")
	}

	return &pb.GetVersionResponse{ConfigVersion: configVersionToProto(version)}, nil
}

func (s *Service) RollbackToVersion(ctx context.Context, req *pb.RollbackToVersionRequest) (*pb.RollbackToVersionResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	actor := s.getActor(ctx)

	// Pre-transaction reads.
	targetRows, err := s.store.GetFullConfigAtVersion(ctx, dbstore.GetFullConfigAtVersionParams{
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

	// Transaction: new version + copied values + audit.
	var newVersion dbstore.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		newVersion, txErr = tx.CreateConfigVersion(ctx, dbstore.CreateConfigVersionParams{
			TenantID:    tenantID,
			Version:     latest.Version + 1,
			Description: &desc,
			CreatedBy:   actor,
		})
		if txErr != nil {
			return fmt.Errorf("create rollback version: %w", txErr)
		}

		for _, row := range targetRows {
			if txErr = tx.SetConfigValue(ctx, dbstore.SetConfigValueParams{
				ConfigVersionID: newVersion.ID,
				FieldPath:       row.FieldPath,
				Value:           row.Value,
				Checksum:        row.Checksum,
				Description:     row.Description,
			}); txErr != nil {
				return fmt.Errorf("copy field %s: %w", row.FieldPath, txErr)
			}
		}

		newValue := fmt.Sprintf("v%d", req.Version)
		return tx.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
			TenantID:      tenantID,
			Actor:         actor,
			Action:        "rollback",
			FieldPath:     ptrString(""),
			OldValue:      ptrString(""),
			NewValue:      &newValue,
			ConfigVersion: &newVersion.Version,
		})
	}); err != nil {
		s.logger.ErrorContext(ctx, "rollback transaction failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to rollback")
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, req.TenantId); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}

	s.metrics.RecordWrite(ctx, req.TenantId, "rollback")
	s.metrics.RecordVersion(ctx, req.TenantId, int64(newVersion.Version))

	return &pb.RollbackToVersionResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

// --- Subscriptions ---

func (s *Service) Subscribe(req *pb.SubscribeRequest, stream grpc.ServerStreamingServer[pb.SubscribeResponse]) error {
	ctx := stream.Context()

	events, cancel, err := s.subscriber.Subscribe(ctx, req.TenantId)
	if err != nil {
		return status.Error(codes.Internal, "failed to subscribe")
	}
	defer cancel()

	filterPaths := make(map[string]struct{}, len(req.FieldPaths))
	for _, p := range req.FieldPaths {
		filterPaths[p] = struct{}{}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
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
					OldValue:  stringToTypedValue(ptrString(event.OldValue), pb.FieldType_FIELD_TYPE_STRING),
					NewValue:  stringToTypedValue(strPtr(event.NewValue), pb.FieldType_FIELD_TYPE_STRING),
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
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
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
	dbRows, err := s.store.GetFullConfigAtVersion(ctx, dbstore.GetFullConfigAtVersionParams{
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

	// Get version description.
	var description string
	cv, err := s.store.GetConfigVersion(ctx, dbstore.GetConfigVersionParams{
		TenantID: tenantID,
		Version:  version,
	})
	if err == nil && cv.Description != nil {
		description = *cv.Description
	}

	doc := configToYAML(version, description, rows, fieldTypes)
	data, err := marshalConfigYAML(doc)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal YAML")
	}

	return &pb.ExportConfigResponse{YamlContent: data}, nil
}

func (s *Service) ImportConfig(ctx context.Context, req *pb.ImportConfigRequest) (*pb.ImportConfigResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	doc, err := unmarshalConfigYAML(req.YamlContent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid config YAML: %v", err)
	}

	actor := s.getActor(ctx)

	// Verify tenant exists.
	if _, err := s.store.GetTenantByID(ctx, tenantID); err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}

	// Get schema field types for type-aware conversion.
	fieldTypes, err := s.getFieldTypeMap(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	// Convert YAML values to string representations.
	values, err := yamlToConfigValues(doc, fieldTypes)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "value conversion error: %v", err)
	}

	// Check field locks and validate.
	for _, v := range values {
		if err := s.checkFieldLock(ctx, tenantID, v.FieldPath); err != nil {
			return nil, err
		}
		// Convert string value to TypedValue for validation.
		ft := fieldTypes[v.FieldPath]
		tv := stringToTypedValue(&v.Value, ft)
		if err := s.validateField(ctx, tenantID, req.TenantId, v.FieldPath, tv); err != nil {
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
	changes := make([]changeRecord, 0, len(values))
	for _, v := range values {
		changes = append(changes, changeRecord{
			fieldPath: v.FieldPath,
			oldValue:  s.getCurrentValue(ctx, tenantID, v.FieldPath, latestVersion),
			newValue:  v.Value,
		})
	}

	// Import description.
	desc := "Import from YAML"
	if req.Description != nil {
		desc = *req.Description
	} else if doc.Description != "" {
		desc = doc.Description
	}

	// Transaction: version + all values + audit entries.
	var newVersion dbstore.ConfigVersion
	if err := s.store.RunInTx(ctx, func(tx Store) error {
		var txErr error
		newVersion, txErr = tx.CreateConfigVersion(ctx, dbstore.CreateConfigVersionParams{
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
			if txErr = tx.SetConfigValue(ctx, dbstore.SetConfigValueParams{
				ConfigVersionID: newVersion.ID,
				FieldPath:       v.FieldPath,
				Value:           importValStr,
				Checksum:        checksumPtr(importValStr),
				Description:     v.Description,
			}); txErr != nil {
				return fmt.Errorf("set config value %s: %w", v.FieldPath, txErr)
			}

			if txErr = tx.InsertAuditWriteLog(ctx, dbstore.InsertAuditWriteLogParams{
				TenantID:      tenantID,
				Actor:         actor,
				Action:        "import",
				FieldPath:     ptrString(v.FieldPath),
				OldValue:      ptrString(changes[i].oldValue),
				NewValue:      strPtr(v.Value),
				ConfigVersion: &newVersion.Version,
			}); txErr != nil {
				return fmt.Errorf("insert audit log for %s: %w", v.FieldPath, txErr)
			}
		}

		return nil
	}); err != nil {
		s.logger.ErrorContext(ctx, "import config transaction failed", "error", err)
		return nil, status.Error(codes.Internal, "failed to import config")
	}

	// Post-transaction side effects.
	if err := s.cache.Invalidate(ctx, req.TenantId); err != nil {
		s.logger.WarnContext(ctx, "failed to invalidate cache", "error", err)
	}
	for _, ch := range changes {
		s.publishChange(ctx, req.TenantId, newVersion.Version, ch.fieldPath, ch.oldValue, ch.newValue, actor)
	}

	s.metrics.RecordWrite(ctx, req.TenantId, "import")
	s.metrics.RecordVersion(ctx, req.TenantId, int64(newVersion.Version))

	return &pb.ImportConfigResponse{ConfigVersion: configVersionToProto(newVersion)}, nil
}

// getFieldTypeMap fetches the tenant's schema fields and builds a map of field path to proto FieldType.
func (s *Service) getFieldTypeMap(ctx context.Context, tenantID pgtype.UUID) (map[string]pb.FieldType, error) {
	tenant, err := s.store.GetTenantByID(ctx, tenantID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "tenant not found")
	}

	sv, err := s.store.GetSchemaVersion(ctx, dbstore.GetSchemaVersionParams{
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

	result := make(map[string]pb.FieldType, len(fields))
	for _, f := range fields {
		result[f.Path] = dbFieldTypeToProto(f.FieldType)
	}
	return result, nil
}

// --- Helpers ---

func (s *Service) resolveVersion(ctx context.Context, tenantID pgtype.UUID, requested *int32) (int32, error) {
	if requested != nil {
		return *requested, nil
	}
	latest, err := s.store.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil // No versions yet.
		}
		return 0, status.Error(codes.Internal, "failed to get latest version")
	}
	return latest.Version, nil
}

func (s *Service) getOrCreateVersion(ctx context.Context, tenantID pgtype.UUID) (int32, error) {
	latest, err := s.store.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

func (s *Service) getCurrentValue(ctx context.Context, tenantID pgtype.UUID, fieldPath string, version int32) string {
	if version == 0 {
		return ""
	}
	row, err := s.store.GetConfigValueAtVersion(ctx, dbstore.GetConfigValueAtVersionParams{
		TenantID:  tenantID,
		FieldPath: fieldPath,
		Version:   version,
	})
	if err != nil {
		return ""
	}
	return derefString(row.Value)
}

func (s *Service) checkChecksum(ctx context.Context, tenantID pgtype.UUID, fieldPath, expected string) error {
	latest, err := s.store.GetLatestConfigVersion(ctx, tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // No existing value — checksum check passes.
		}
		return status.Error(codes.Internal, "failed to get latest version")
	}
	row, err := s.store.GetConfigValueAtVersion(ctx, dbstore.GetConfigValueAtVersionParams{
		TenantID:  tenantID,
		FieldPath: fieldPath,
		Version:   latest.Version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // Field doesn't exist yet.
		}
		return status.Error(codes.Internal, "failed to get current value for checksum")
	}
	actual := derefString(row.Checksum)
	if actual != expected {
		return status.Errorf(codes.Aborted, "checksum mismatch for %s: expected %s, got %s", fieldPath, expected, actual)
	}
	return nil
}

func (s *Service) checkFieldLock(ctx context.Context, tenantID pgtype.UUID, fieldPath string) error {
	claims, ok := auth.ClaimsFromContext(ctx)
	if !ok || claims.Role == auth.RoleSuperAdmin {
		return nil // SuperAdmin bypasses locks.
	}

	locks, err := s.store.GetFieldLocks(ctx, tenantID)
	if err != nil {
		return status.Error(codes.Internal, "failed to check field locks")
	}
	for _, lock := range locks {
		if lock.FieldPath == fieldPath {
			return status.Errorf(codes.PermissionDenied, "field %s is locked", fieldPath)
		}
	}
	return nil
}

// validateField validates a typed value against the schema constraints.
// In strict mode, rejects fields not defined in the schema.
func (s *Service) validateField(ctx context.Context, tenantID pgtype.UUID, tenantIDStr, fieldPath string, value *pb.TypedValue) error {
	if s.validators == nil {
		return nil
	}
	validators, err := s.validators.GetValidators(ctx, tenantID, tenantIDStr)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to get validators", "error", err)
		return nil // don't block writes on validator lookup failure
	}
	v, ok := validators[fieldPath]
	if !ok {
		return status.Errorf(codes.InvalidArgument, "field %s is not defined in the schema", fieldPath)
	}
	if err := v.Validate(value); err != nil {
		return status.Errorf(codes.InvalidArgument, "%v", err)
	}
	return nil
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
