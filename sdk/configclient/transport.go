package configclient

import "context"

// Transport abstracts the underlying RPC mechanism for config operations.
// The default implementation uses gRPC (see the grpctransport package).
type Transport interface {
	GetField(ctx context.Context, req *GetFieldRequest) (*GetFieldResponse, error)
	GetConfig(ctx context.Context, req *GetConfigRequest) (*GetConfigResponse, error)
	GetFields(ctx context.Context, req *GetFieldsRequest) (*GetFieldsResponse, error)
	SetField(ctx context.Context, req *SetFieldRequest) (*SetFieldResponse, error)
	SetFields(ctx context.Context, req *SetFieldsRequest) (*SetFieldsResponse, error)
	Subscribe(ctx context.Context, req *SubscribeRequest) (Subscription, error)
}

// Subscription receives streaming config changes from the server.
type Subscription interface {
	Recv() (*ConfigChange, error)
}

// --- Request / Response types ---

// GetFieldRequest is the input for [Transport.GetField].
type GetFieldRequest struct {
	TenantID  string
	FieldPath string
	Version   *int32
}

// GetFieldResponse is the output of [Transport.GetField].
type GetFieldResponse struct {
	FieldPath string
	Value     *TypedValue // nil when the field is null
	Checksum  string
}

// GetConfigRequest is the input for [Transport.GetConfig].
type GetConfigRequest struct {
	TenantID string
	Version  *int32
}

// GetConfigResponse is the output of [Transport.GetConfig].
type GetConfigResponse struct {
	TenantID string
	Version  int32
	Values   []ConfigValue
}

// ConfigValue is a single field value within a config snapshot.
type ConfigValue struct {
	FieldPath string
	Value     *TypedValue // nil when the field is null
	Checksum  string
}

// GetFieldsRequest is the input for [Transport.GetFields].
type GetFieldsRequest struct {
	TenantID   string
	FieldPaths []string
	Version    *int32
}

// GetFieldsResponse is the output of [Transport.GetFields].
type GetFieldsResponse struct {
	Values []ConfigValue
}

// SetFieldRequest is the input for [Transport.SetField].
type SetFieldRequest struct {
	TenantID         string
	FieldPath        string
	Value            *TypedValue // nil sets the field to null
	ExpectedChecksum *string
	Description      string
}

// SetFieldResponse is the output of [Transport.SetField].
type SetFieldResponse struct{}

// FieldUpdate describes a single field change within a batch write.
type FieldUpdate struct {
	FieldPath        string
	Value            *TypedValue // nil sets the field to null
	ExpectedChecksum *string
}

// SetFieldsRequest is the input for [Transport.SetFields].
type SetFieldsRequest struct {
	TenantID    string
	Updates     []FieldUpdate
	Description string
}

// SetFieldsResponse is the output of [Transport.SetFields].
type SetFieldsResponse struct{}

// SubscribeRequest is the input for [Transport.Subscribe].
type SubscribeRequest struct {
	TenantID   string
	FieldPaths []string
}

// ConfigChange represents a single field value change from a subscription stream.
type ConfigChange struct {
	TenantID  string
	FieldPath string
	OldValue  *TypedValue
	NewValue  *TypedValue
}
