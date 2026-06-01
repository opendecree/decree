package grpctransport

import (
	"context"

	"google.golang.org/grpc"

	pb "github.com/opendecree/decree/api/centralconfig/v1"
	"github.com/opendecree/decree/sdk/adminclient"
)

// SchemaTransport implements [adminclient.SchemaTransport] using gRPC.
type SchemaTransport struct {
	rpc  pb.SchemaServiceClient
	auth authConfig
}

// Compile-time check.
var _ adminclient.SchemaTransport = (*SchemaTransport)(nil)

// NewSchemaTransport creates a new gRPC-backed schema transport.
// WithRole (or WithBearerToken) is required; construction returns an error if omitted.
func NewSchemaTransport(conn grpc.ClientConnInterface, opts ...Option) (*SchemaTransport, error) {
	cfg, err := buildConfig(opts)
	if err != nil {
		return nil, err
	}
	return &SchemaTransport{
		rpc:  pb.NewSchemaServiceClient(conn),
		auth: cfg.auth,
	}, nil
}

func (t *SchemaTransport) CreateSchema(ctx context.Context, req *adminclient.CreateSchemaRequest) (*adminclient.Schema, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	protoReq := &pb.CreateSchemaRequest{
		Name:   req.Name,
		Fields: fieldsToProto(req.Fields),
	}
	if req.Description != "" {
		protoReq.Description = &req.Description
	}
	resp, err := t.rpc.CreateSchema(ctx, protoReq, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return schemaFromProto(resp.GetSchema()), nil
}

func (t *SchemaTransport) GetSchema(ctx context.Context, id string, version *int32) (*adminclient.Schema, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.GetSchema(ctx, &pb.GetSchemaRequest{
		Id:      id,
		Version: version,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return schemaFromProto(resp.GetSchema()), nil
}

func (t *SchemaTransport) ListSchemas(ctx context.Context, pageSize int32, pageToken string) (*adminclient.ListSchemasResponse, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.ListSchemas(ctx, &pb.ListSchemasRequest{
		PageSize:  pageSize,
		PageToken: pageToken,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	schemas := make([]*adminclient.Schema, len(resp.GetSchemas()))
	for i, s := range resp.GetSchemas() {
		schemas[i] = schemaFromProto(s)
	}
	return &adminclient.ListSchemasResponse{
		Schemas:       schemas,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

func (t *SchemaTransport) UpdateSchema(ctx context.Context, req *adminclient.UpdateSchemaRequest) (*adminclient.Schema, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	protoReq := &pb.UpdateSchemaRequest{
		Id:           req.ID,
		Fields:       fieldsToProto(req.AddOrModify),
		RemoveFields: req.RemoveFields,
	}
	if req.VersionDescription != "" {
		protoReq.VersionDescription = &req.VersionDescription
	}
	resp, err := t.rpc.UpdateSchema(ctx, protoReq, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return schemaFromProto(resp.GetSchema()), nil
}

func (t *SchemaTransport) PublishSchema(ctx context.Context, id string, version int32) (*adminclient.Schema, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.PublishSchema(ctx, &pb.PublishSchemaRequest{
		Id:      id,
		Version: version,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return schemaFromProto(resp.GetSchema()), nil
}

func (t *SchemaTransport) DeleteSchema(ctx context.Context, id string) error {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return err
	}
	_, err = t.rpc.DeleteSchema(ctx, &pb.DeleteSchemaRequest{Id: id}, callOpts...)
	return mapAdminError(err)
}

func (t *SchemaTransport) ExportSchema(ctx context.Context, id string, version *int32) ([]byte, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.ExportSchema(ctx, &pb.ExportSchemaRequest{
		Id:      id,
		Version: version,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return resp.GetYamlContent(), nil
}

func (t *SchemaTransport) ImportSchema(ctx context.Context, yamlContent []byte, autoPublish bool) (*adminclient.Schema, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.ImportSchema(ctx, &pb.ImportSchemaRequest{
		YamlContent: yamlContent,
		AutoPublish: autoPublish,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return schemaFromProto(resp.GetSchema()), nil
}

// --- Tenant methods ---

func (t *SchemaTransport) CreateTenant(ctx context.Context, req *adminclient.CreateTenantRequest) (*adminclient.Tenant, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.CreateTenant(ctx, &pb.CreateTenantRequest{
		Name:          req.Name,
		SchemaId:      req.SchemaID,
		SchemaVersion: req.SchemaVersion,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return tenantFromProto(resp.GetTenant()), nil
}

func (t *SchemaTransport) GetTenant(ctx context.Context, id string) (*adminclient.Tenant, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.GetTenant(ctx, &pb.GetTenantRequest{Id: id}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return tenantFromProto(resp.GetTenant()), nil
}

func (t *SchemaTransport) ListTenants(ctx context.Context, schemaID *string, pageSize int32, pageToken string) (*adminclient.ListTenantsResponse, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.ListTenants(ctx, &pb.ListTenantsRequest{
		SchemaId:  schemaID,
		PageSize:  pageSize,
		PageToken: pageToken,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	tenants := make([]*adminclient.Tenant, len(resp.GetTenants()))
	for i, tt := range resp.GetTenants() {
		tenants[i] = tenantFromProto(tt)
	}
	return &adminclient.ListTenantsResponse{
		Tenants:       tenants,
		NextPageToken: resp.GetNextPageToken(),
	}, nil
}

func (t *SchemaTransport) UpdateTenant(ctx context.Context, req *adminclient.UpdateTenantRequest) (*adminclient.Tenant, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.UpdateTenant(ctx, &pb.UpdateTenantRequest{
		Id:            req.ID,
		Name:          req.Name,
		SchemaVersion: req.SchemaVersion,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	return tenantFromProto(resp.GetTenant()), nil
}

func (t *SchemaTransport) DeleteTenant(ctx context.Context, id string) error {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return err
	}
	_, err = t.rpc.DeleteTenant(ctx, &pb.DeleteTenantRequest{Id: id}, callOpts...)
	return mapAdminError(err)
}

// --- Field lock methods ---

func (t *SchemaTransport) LockField(ctx context.Context, tenantID, fieldPath string, lockedValues []string) error {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return err
	}
	_, err = t.rpc.LockField(ctx, &pb.LockFieldRequest{
		TenantId:     tenantID,
		FieldPath:    fieldPath,
		LockedValues: lockedValues,
	}, callOpts...)
	return mapAdminError(err)
}

func (t *SchemaTransport) UnlockField(ctx context.Context, tenantID, fieldPath string) error {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return err
	}
	_, err = t.rpc.UnlockField(ctx, &pb.UnlockFieldRequest{
		TenantId:  tenantID,
		FieldPath: fieldPath,
	}, callOpts...)
	return mapAdminError(err)
}

func (t *SchemaTransport) ListFieldLocks(ctx context.Context, tenantID string) ([]adminclient.FieldLock, error) {
	ctx, callOpts, err := applyAuth(ctx, t.auth)
	if err != nil {
		return nil, err
	}
	resp, err := t.rpc.ListFieldLocks(ctx, &pb.ListFieldLocksRequest{
		TenantId: tenantID,
	}, callOpts...)
	if err != nil {
		return nil, mapAdminError(err)
	}
	locks := make([]adminclient.FieldLock, len(resp.GetLocks()))
	for i, l := range resp.GetLocks() {
		locks[i] = adminclient.FieldLock{
			TenantID:     l.GetTenantId(),
			FieldPath:    l.GetFieldPath(),
			LockedValues: l.GetLockedValues(),
		}
	}
	return locks, nil
}
