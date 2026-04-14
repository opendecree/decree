package audit

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/internal/storage/domain"
)

// Service implements the AuditService gRPC server.
type Service struct {
	pb.UnimplementedAuditServiceServer
	store  Store
	logger *slog.Logger
}

// NewService creates a new AuditService.
func NewService(store Store, logger *slog.Logger) *Service {
	return &Service{
		store:  store,
		logger: logger,
	}
}

func (s *Service) QueryWriteLog(ctx context.Context, req *pb.QueryWriteLogRequest) (*pb.QueryWriteLogResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 50
	}

	params := QueryWriteLogParams{
		Limit:  pageSize,
		Offset: 0,
	}
	if req.TenantId != nil {
		if !isValidUUID(*req.TenantId) {
			return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
		}
		params.TenantID = *req.TenantId
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

	pbEntries := make([]*pb.AuditEntry, 0, len(entries))
	for _, e := range entries {
		pbEntries = append(pbEntries, auditEntryToProto(e))
	}

	return &pb.QueryWriteLogResponse{Entries: pbEntries}, nil
}

func (s *Service) GetFieldUsage(ctx context.Context, req *pb.GetFieldUsageRequest) (*pb.GetFieldUsageResponse, error) {
	if !isValidUUID(req.TenantId) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	params := GetFieldUsageParams{
		TenantID:  req.TenantId,
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
			TenantId:   req.TenantId,
			FieldPath:  req.FieldPath,
			ReadCount:  totalReads,
			LastReadBy: lastReadBy,
			LastReadAt: lastReadAt,
		},
	}, nil
}

func (s *Service) GetTenantUsage(ctx context.Context, req *pb.GetTenantUsageRequest) (*pb.GetTenantUsageResponse, error) {
	if !isValidUUID(req.TenantId) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	params := GetTenantUsageParams{
		TenantID: req.TenantId,
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
			TenantId:  req.TenantId,
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
	if !isValidUUID(req.TenantId) {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	paths, err := s.store.GetUnusedFields(ctx, GetUnusedFieldsParams{
		TenantID: req.TenantId,
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
