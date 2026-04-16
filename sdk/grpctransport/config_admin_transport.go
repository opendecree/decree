package grpctransport

import (
	"context"

	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
)

// AdminConfigTransport implements [adminclient.ConfigTransport] using gRPC.
type AdminConfigTransport struct {
	rpc  pb.ConfigServiceClient
	auth authConfig
}

// Compile-time check.
var _ adminclient.ConfigTransport = (*AdminConfigTransport)(nil)

// NewAdminConfigTransport creates a new gRPC-backed admin config transport.
func NewAdminConfigTransport(conn grpc.ClientConnInterface, opts ...Option) *AdminConfigTransport {
	cfg := buildConfig(opts)
	return &AdminConfigTransport{
		rpc:  pb.NewConfigServiceClient(conn),
		auth: cfg.auth,
	}
}

func (t *AdminConfigTransport) ListVersions(ctx context.Context, tenantID string, pageSize int32, pageToken string) (*adminclient.ListVersionsResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.ListVersions(ctx, &pb.ListVersionsRequest{
		TenantId:  tenantID,
		PageSize:  pageSize,
		PageToken: pageToken,
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	versions := make([]*adminclient.Version, len(resp.GetVersions()))
	for i, v := range resp.GetVersions() {
		versions[i] = versionFromProto(v)
	}
	return &adminclient.ListVersionsResponse{
		Versions:      versions,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

func (t *AdminConfigTransport) GetVersion(ctx context.Context, tenantID string, version int32) (*adminclient.Version, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetVersion(ctx, &pb.GetVersionRequest{
		TenantId: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	return versionFromProto(resp.GetConfigVersion()), nil
}

func (t *AdminConfigTransport) RollbackToVersion(ctx context.Context, tenantID string, version int32, description string) (*adminclient.Version, error) {
	ctx = applyAuth(ctx, t.auth)
	protoReq := &pb.RollbackToVersionRequest{
		TenantId: tenantID,
		Version:  version,
	}
	if description != "" {
		protoReq.Description = &description
	}
	resp, err := t.rpc.RollbackToVersion(ctx, protoReq)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return versionFromProto(resp.GetConfigVersion()), nil
}

func (t *AdminConfigTransport) ExportConfig(ctx context.Context, tenantID string, version *int32) ([]byte, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.ExportConfig(ctx, &pb.ExportConfigRequest{
		TenantId: tenantID,
		Version:  version,
	})
	if err != nil {
		return nil, mapAdminError(err)
	}
	return resp.GetYamlContent(), nil
}

func (t *AdminConfigTransport) ImportConfig(ctx context.Context, req *adminclient.ImportConfigRequest) (*adminclient.Version, error) {
	ctx = applyAuth(ctx, t.auth)
	protoReq := &pb.ImportConfigRequest{
		TenantId:    req.TenantID,
		YamlContent: req.YamlContent,
		Mode:        pb.ImportMode(req.Mode),
	}
	if req.Description != "" {
		protoReq.Description = &req.Description
	}
	resp, err := t.rpc.ImportConfig(ctx, protoReq)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return versionFromProto(resp.GetConfigVersion()), nil
}
