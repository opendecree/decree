package grpctransport

import (
	"context"
	"time"

	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/opendecree/decree/sdk/adminclient"
)

// AuditTransport implements [adminclient.AuditTransport] using gRPC.
type AuditTransport struct {
	rpc  pb.AuditServiceClient
	auth authConfig
}

// Compile-time check.
var _ adminclient.AuditTransport = (*AuditTransport)(nil)

// NewAuditTransport creates a new gRPC-backed audit transport.
func NewAuditTransport(conn grpc.ClientConnInterface, opts ...Option) *AuditTransport {
	cfg := buildConfig(opts)
	return &AuditTransport{
		rpc:  pb.NewAuditServiceClient(conn),
		auth: cfg.auth,
	}
}

func (t *AuditTransport) QueryWriteLog(ctx context.Context, req *adminclient.QueryWriteLogRequest) (*adminclient.QueryWriteLogResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	protoReq := &pb.QueryWriteLogRequest{
		TenantId:  req.TenantID,
		Actor:     req.Actor,
		FieldPath: req.FieldPath,
		StartTime: timeToProto(req.StartTime),
		EndTime:   timeToProto(req.EndTime),
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
	}
	resp, err := t.rpc.QueryWriteLog(ctx, protoReq)
	if err != nil {
		return nil, mapAdminError(err)
	}
	entries := make([]*adminclient.AuditEntry, len(resp.GetEntries()))
	for i, e := range resp.GetEntries() {
		entries[i] = auditEntryFromProto(e)
	}
	return &adminclient.QueryWriteLogResponse{
		Entries:       entries,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

func (t *AuditTransport) GetFieldUsage(ctx context.Context, tenantID, fieldPath string, start, end *time.Time) (*adminclient.UsageStats, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetFieldUsage(ctx, &pb.GetFieldUsageRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
		StartTime: timeToProto(start),
		EndTime:   timeToProto(end),
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	return usageStatsFromProto(resp.GetStats()), nil
}

func (t *AuditTransport) GetTenantUsage(ctx context.Context, tenantID string, start, end *time.Time) ([]*adminclient.UsageStats, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetTenantUsage(ctx, &pb.GetTenantUsageRequest{
		TenantId:  tenantID,
		StartTime: timeToProto(start),
		EndTime:   timeToProto(end),
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	stats := make([]*adminclient.UsageStats, len(resp.GetFieldStats()))
	for i, s := range resp.GetFieldStats() {
		stats[i] = usageStatsFromProto(s)
	}
	return stats, nil
}

func (t *AuditTransport) GetUnusedFields(ctx context.Context, tenantID string, since time.Time) ([]string, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetUnusedFields(ctx, &pb.GetUnusedFieldsRequest{
		TenantId: tenantID,
		Since:    timestamppb.New(since),
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	return resp.GetFieldPaths(), nil
}
