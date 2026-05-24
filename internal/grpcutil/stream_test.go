package grpcutil_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/opendecree/decree/internal/grpcutil"
)

type fakeStream struct{ grpc.ServerStream }

func TestWrappedStream_Context(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "sentinel")
	ws := grpcutil.NewWrappedStream(fakeStream{}, ctx)
	assert.Equal(t, ctx, ws.Context())
}
