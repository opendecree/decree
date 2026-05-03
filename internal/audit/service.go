package audit

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/auth"
	"github.com/opendecree/decree/internal/authz"
	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/storage/domain"
)

// TenantResolver resolves a tenant UUID or name slug to a UUID.
type TenantResolver func(ctx context.Context, idOrName string) (string, error)

// Service implements the AuditService gRPC server.
type Service struct {
	pb.UnimplementedAuditServiceServer
	store         Store
	logger        *slog.Logger
	resolveTenant TenantResolver
	guard         authz.Guard
}

// NewService creates a new AuditService.
func NewService(store Store, logger *slog.Logger, resolver TenantResolver) *Service {
	return &Service{
		store:         store,
		logger:        logger,
		resolveTenant: resolver,
		guard:         authz.Chain(authz.TenantScopeGuard{}),
	}
}

// resolveTenantWithAccess resolves a tenant and checks read access in one step.
// Returns a gRPC status error on failure — callers can return the error directly.
func (s *Service) resolveTenantWithAccess(ctx context.Context, idOrName string) (string, error) {
	tenantID, err := s.resolveTenantID(ctx, idOrName)
	if err != nil {
		return "", err
	}
	if err := s.guard.Check(ctx, authz.ActionRead, authz.Resource{TenantID: tenantID}); err != nil {
		return "", err
	}
	return tenantID, nil
}

// resolveTenantID resolves a tenant UUID or slug. If no resolver is set, requires UUID.
func (s *Service) resolveTenantID(ctx context.Context, idOrName string) (string, error) {
	if idOrName == "" {
		return "", status.Error(codes.InvalidArgument, "tenant id or name required")
	}
	if isValidUUID(idOrName) {
		return idOrName, nil
	}
	if s.resolveTenant != nil {
		resolved, err := s.resolveTenant(ctx, idOrName)
		if err != nil {
			return "", status.Errorf(codes.NotFound, "tenant %q not found", idOrName)
		}
		return resolved, nil
	}
	return "", status.Error(codes.InvalidArgument, "invalid tenant id")
}

func (s *Service) QueryWriteLog(ctx context.Context, req *pb.QueryWriteLogRequest) (*pb.QueryWriteLogResponse, error) {
	if err := auth.MustHaveClaims(ctx); err != nil {
		return nil, err
	}
	pageSize := pagination.ClampPageSize(req.PageSize, 50, 500)

	offset, err := pagination.DecodePageToken(req.PageToken)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid page token")
	}

	params := QueryWriteLogParams{
		Limit:  pageSize + 1,
		Offset: offset,
	}
	if req.TenantId != nil {
		resolved, err := s.resolveTenantID(ctx, *req.TenantId)
		if err != nil {
			return nil, err
		}
		if err := s.guard.Check(ctx, authz.ActionRead, authz.Resource{TenantID: resolved}); err != nil {
			return nil, err
		}
		params.TenantID = resolved
	} else if claims, ok := auth.ClaimsFromContext(ctx); ok && !claims.IsSuperAdmin() {
		return nil, status.Error(codes.PermissionDenied, "tenant_id required for non-superadmin callers")
	}
	if req.Actor != nil {
		params.Actor = *req.Actor
	}
	if req.FieldPath != nil {
		params.FieldPath = *req.FieldPath
	}
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		params.StartTime = &t
	}
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		params.EndTime = &t
	}

	entries, err := s.store.QueryAuditWriteLog(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "query audit write log", "error", err)
		return nil, status.Error(codes.Internal, "failed to query audit log")
	}

	nextToken := pagination.NextPageToken(pageSize, int32(len(entries)), offset)
	if int32(len(entries)) > pageSize {
		entries = entries[:pageSize]
	}

	pbEntries := make([]*pb.AuditEntry, 0, len(entries))
	for _, e := range entries {
		pbEntries = append(pbEntries, auditEntryToProto(e))
	}

	return &pb.QueryWriteLogResponse{
		Entries:       pbEntries,
		NextPageToken: nextToken,
	}, nil
}

func (s *Service) GetFieldUsage(ctx context.Context, req *pb.GetFieldUsageRequest) (*pb.GetFieldUsageResponse, error) {
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	params := GetFieldUsageParams{
		TenantID:  tenantID,
		FieldPath: req.FieldPath,
	}
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		params.StartTime = &t
	}
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		params.EndTime = &t
	}

	stats, err := s.store.GetFieldUsage(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "get field usage", "error", err)
		return nil, status.Error(codes.Internal, "failed to get field usage")
	}

	// Aggregate across periods.
	var totalReads int64
	var lastReadBy *string
	var lastReadAt *timestamppb.Timestamp
	for _, stat := range stats {
		totalReads += stat.ReadCount
		if stat.LastReadBy != nil {
			lastReadBy = stat.LastReadBy
		}
		if stat.LastReadAt != nil {
			lastReadAt = timestamppb.New(*stat.LastReadAt)
		}
	}

	return &pb.GetFieldUsageResponse{
		Stats: &pb.UsageStats{
			TenantId:   tenantID,
			FieldPath:  req.FieldPath,
			ReadCount:  totalReads,
			LastReadBy: lastReadBy,
			LastReadAt: lastReadAt,
		},
	}, nil
}

func (s *Service) GetTenantUsage(ctx context.Context, req *pb.GetTenantUsageRequest) (*pb.GetTenantUsageResponse, error) {
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	params := GetTenantUsageParams{
		TenantID: tenantID,
	}
	if req.StartTime != nil {
		t := req.StartTime.AsTime()
		params.StartTime = &t
	}
	if req.EndTime != nil {
		t := req.EndTime.AsTime()
		params.EndTime = &t
	}

	rows, err := s.store.GetTenantUsage(ctx, params)
	if err != nil {
		s.logger.ErrorContext(ctx, "get tenant usage", "error", err)
		return nil, status.Error(codes.Internal, "failed to get tenant usage")
	}

	fieldStats := make([]*pb.UsageStats, 0, len(rows))
	for _, row := range rows {
		stat := &pb.UsageStats{
			TenantId:  tenantID,
			FieldPath: row.FieldPath,
			ReadCount: row.ReadCount,
		}
		if row.LastReadAt != nil {
			stat.LastReadAt = timestamppb.New(*row.LastReadAt)
		}
		fieldStats = append(fieldStats, stat)
	}

	return &pb.GetTenantUsageResponse{FieldStats: fieldStats}, nil
}

func (s *Service) GetUnusedFields(ctx context.Context, req *pb.GetUnusedFieldsRequest) (*pb.GetUnusedFieldsResponse, error) {
	tenantID, err := s.resolveTenantWithAccess(ctx, req.TenantId)
	if err != nil {
		return nil, err
	}

	paths, err := s.store.GetUnusedFields(ctx, GetUnusedFieldsParams{
		TenantID: tenantID,
		Since:    req.Since.AsTime(),
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "get unused fields", "error", err)
		return nil, status.Error(codes.Internal, "failed to get unused fields")
	}

	return &pb.GetUnusedFieldsResponse{FieldPaths: paths}, nil
}

// --- Helpers ---

func isValidUUID(s string) bool {
	// Simple length + format check. Full validation happens in the store layer.
	if len(s) != 36 {
		return false
	}
	return s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-'
}

func auditEntryToProto(e domain.AuditWriteLog) *pb.AuditEntry {
	entry := &pb.AuditEntry{
		Id:        e.ID,
		TenantId:  e.TenantID,
		Actor:     e.Actor,
		Action:    e.Action,
		CreatedAt: timestamppb.New(e.CreatedAt),
	}
	entry.FieldPath = e.FieldPath
	entry.OldValue = e.OldValue
	entry.NewValue = e.NewValue
	entry.ConfigVersion = e.ConfigVersion
	return entry
}
