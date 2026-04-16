package grpctransport

import (
	"context"

	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/configclient"
)

// ConfigTransport implements [configclient.Transport] using gRPC.
type ConfigTransport struct {
	rpc  pb.ConfigServiceClient
	auth authConfig
}

// Compile-time check.
var _ configclient.Transport = (*ConfigTransport)(nil)

// NewConfigTransport creates a new gRPC-backed config transport.
func NewConfigTransport(conn grpc.ClientConnInterface, opts ...Option) *ConfigTransport {
	cfg := buildConfig(opts)
	return &ConfigTransport{
		rpc:  pb.NewConfigServiceClient(conn),
		auth: cfg.auth,
	}
}

func (t *ConfigTransport) GetField(ctx context.Context, req *configclient.GetFieldRequest) (*configclient.GetFieldResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetField(ctx, &pb.GetFieldRequest{
		TenantId:  req.TenantID,
		FieldPath: req.FieldPath,
		Version:   req.Version,
	})
	if err != nil {
		return nil, mapConfigError(err)
	}
	cv := resp.GetValue()
	return &configclient.GetFieldResponse{
		FieldPath: cv.GetFieldPath(),
		Value:     typedValueFromProto(cv.GetValue()),
		Checksum:  cv.GetChecksum(),
	}, nil
}

func (t *ConfigTransport) GetConfig(ctx context.Context, req *configclient.GetConfigRequest) (*configclient.GetConfigResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetConfig(ctx, &pb.GetConfigRequest{
		TenantId: req.TenantID,
		Version:  req.Version,
	})
	if err != nil {
		return nil, mapConfigError(err)
	}
	cfg := resp.GetConfig()
	values := make([]configclient.ConfigValue, len(cfg.GetValues()))
	for i, v := range cfg.GetValues() {
		values[i] = configValueFromProto(v)
	}
	return &configclient.GetConfigResponse{
		TenantID: cfg.GetTenantId(),
		Version:  cfg.GetVersion(),
		Values:   values,
	}, nil
}

func (t *ConfigTransport) GetFields(ctx context.Context, req *configclient.GetFieldsRequest) (*configclient.GetFieldsResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	resp, err := t.rpc.GetFields(ctx, &pb.GetFieldsRequest{
		TenantId:   req.TenantID,
		FieldPaths: req.FieldPaths,
		Version:    req.Version,
	})
	if err != nil {
		return nil, mapConfigError(err)
	}
	values := make([]configclient.ConfigValue, len(resp.GetValues()))
	for i, v := range resp.GetValues() {
		values[i] = configValueFromProto(v)
	}
	return &configclient.GetFieldsResponse{
		Values: values,
	}, nil
}

func (t *ConfigTransport) SetField(ctx context.Context, req *configclient.SetFieldRequest) (*configclient.SetFieldResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	protoReq := &pb.SetFieldRequest{
		TenantId:         req.TenantID,
		FieldPath:        req.FieldPath,
		Value:            typedValueToProto(req.Value),
		ExpectedChecksum: req.ExpectedChecksum,
	}
	if req.Description != "" {
		protoReq.Description = &req.Description
	}
	_, err := t.rpc.SetField(ctx, protoReq)
	if err != nil {
		return nil, mapConfigError(err)
	}
	return &configclient.SetFieldResponse{}, nil
}

func (t *ConfigTransport) SetFields(ctx context.Context, req *configclient.SetFieldsRequest) (*configclient.SetFieldsResponse, error) {
	ctx = applyAuth(ctx, t.auth)
	updates := make([]*pb.FieldUpdate, len(req.Updates))
	for i, u := range req.Updates {
		updates[i] = &pb.FieldUpdate{
			FieldPath:        u.FieldPath,
			Value:            typedValueToProto(u.Value),
			ExpectedChecksum: u.ExpectedChecksum,
		}
	}
	protoReq := &pb.SetFieldsRequest{
		TenantId: req.TenantID,
		Updates:  updates,
	}
	if req.Description != "" {
		protoReq.Description = &req.Description
	}
	_, err := t.rpc.SetFields(ctx, protoReq)
	if err != nil {
		return nil, mapConfigError(err)
	}
	return &configclient.SetFieldsResponse{}, nil
}

func (t *ConfigTransport) Subscribe(ctx context.Context, req *configclient.SubscribeRequest) (configclient.Subscription, error) {
	ctx = applyAuth(ctx, t.auth)
	stream, err := t.rpc.Subscribe(ctx, &pb.SubscribeRequest{
		TenantId:   req.TenantID,
		FieldPaths: req.FieldPaths,
	})
	if err != nil {
		return nil, mapConfigError(err)
	}
	return &grpcSubscription{stream: stream}, nil
}
