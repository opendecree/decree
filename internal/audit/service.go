package audit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/zeevdr/central-config-service/api/centralconfig/v1"
	"github.com/zeevdr/central-config-service/internal/storage/dbstore"
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

	params := dbstore.QueryAuditWriteLogParams{
		Limit:  pageSize,
		Offset: 0,
	}

	if req.TenantId != nil {
		id, err := parseUUID(*req.TenantId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
		}
		params.Column1 = id
	}
	if req.Actor != nil {
		params.Column2 = *req.Actor
	}
	if req.FieldPath != nil {
		params.Column3 = *req.FieldPath
	}
	if req.StartTime != nil {
		params.Column4 = pgtype.Timestamptz{Time: req.StartTime.AsTime(), Valid: true}
	}
	if req.EndTime != nil {
		params.Column5 = pgtype.Timestamptz{Time: req.EndTime.AsTime(), Valid: true}
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
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	params := dbstore.GetFieldUsageParams{
		TenantID:  tenantID,
		FieldPath: req.FieldPath,
	}
	if req.StartTime != nil {
		params.Column3 = pgtype.Timestamptz{Time: req.StartTime.AsTime(), Valid: true}
	}
	if req.EndTime != nil {
		params.Column4 = pgtype.Timestamptz{Time: req.EndTime.AsTime(), Valid: true}
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
		if stat.LastReadAt.Valid {
			lastReadAt = timestamppb.New(stat.LastReadAt.Time)
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
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	params := dbstore.GetTenantUsageParams{
		TenantID: tenantID,
	}
	if req.StartTime != nil {
		params.Column2 = pgtype.Timestamptz{Time: req.StartTime.AsTime(), Valid: true}
	}
	if req.EndTime != nil {
		params.Column3 = pgtype.Timestamptz{Time: req.EndTime.AsTime(), Valid: true}
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
		// LastReadAt comes as interface{} from the MAX() aggregate.
		if t, ok := row.LastReadAt.(pgtype.Timestamptz); ok && t.Valid {
			stat.LastReadAt = timestamppb.New(t.Time)
		}
		fieldStats = append(fieldStats, stat)
	}

	return &pb.GetTenantUsageResponse{FieldStats: fieldStats}, nil
}

func (s *Service) GetUnusedFields(ctx context.Context, req *pb.GetUnusedFieldsRequest) (*pb.GetUnusedFieldsResponse, error) {
	tenantID, err := parseUUID(req.TenantId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid tenant id")
	}

	since := pgtype.Timestamptz{Time: req.Since.AsTime(), Valid: true}

	paths, err := s.store.GetUnusedFields(ctx, dbstore.GetUnusedFieldsParams{
		ID:         tenantID,
		LastReadAt: since,
	})
	if err != nil {
		s.logger.ErrorContext(ctx, "get unused fields", "error", err)
		return nil, status.Error(codes.Internal, "failed to get unused fields")
	}

	return &pb.GetUnusedFieldsResponse{FieldPaths: paths}, nil
}

// --- Helpers ---

func parseUUID(s string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(s); err != nil {
		return id, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	return id, nil
}

func uuidToString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", id.Bytes[0:4], id.Bytes[4:6], id.Bytes[6:8], id.Bytes[8:10], id.Bytes[10:16])
}

func auditEntryToProto(e dbstore.AuditWriteLog) *pb.AuditEntry {
	entry := &pb.AuditEntry{
		Id:        uuidToString(e.ID),
		TenantId:  uuidToString(e.TenantID),
		Actor:     e.Actor,
		Action:    e.Action,
		CreatedAt: timestamppb.New(e.CreatedAt.Time),
	}
	entry.FieldPath = e.FieldPath
	entry.OldValue = e.OldValue
	entry.NewValue = e.NewValue
	entry.ConfigVersion = e.ConfigVersion
	return entry
}
