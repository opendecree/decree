package schema

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/opendecree/decree/internal/storage/domain"
)

func TestErrToStatus(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		notFoundMsg string
		failedMsg   string
		wantCode    codes.Code
		wantMsgPart string
	}{
		{
			name:        "ErrNotFound maps to NotFound",
			err:         domain.ErrNotFound,
			notFoundMsg: "thing not found",
			failedMsg:   "failed to get thing",
			wantCode:    codes.NotFound,
			wantMsgPart: "thing not found",
		},
		{
			name:        "wrapped ErrNotFound maps to NotFound",
			err:         fmt.Errorf("db: %w", domain.ErrNotFound),
			notFoundMsg: "record not found",
			failedMsg:   "internal error",
			wantCode:    codes.NotFound,
			wantMsgPart: "record not found",
		},
		{
			name:        "ErrAlreadyExists maps to AlreadyExists",
			err:         domain.ErrAlreadyExists,
			notFoundMsg: "not found",
			failedMsg:   "failed",
			wantCode:    codes.AlreadyExists,
			wantMsgPart: "already exists",
		},
		{
			name:        "wrapped ErrAlreadyExists maps to AlreadyExists",
			err:         fmt.Errorf("db: %w", domain.ErrAlreadyExists),
			notFoundMsg: "not found",
			failedMsg:   "failed",
			wantCode:    codes.AlreadyExists,
			wantMsgPart: "already exists",
		},
		{
			name:        "ErrReferencedByOther maps to FailedPrecondition",
			err:         domain.ErrReferencedByOther,
			notFoundMsg: "not found",
			failedMsg:   "failed",
			wantCode:    codes.FailedPrecondition,
			wantMsgPart: "referenced by active config versions",
		},
		{
			name:        "wrapped ErrReferencedByOther maps to FailedPrecondition",
			err:         fmt.Errorf("db: %w", domain.ErrReferencedByOther),
			notFoundMsg: "not found",
			failedMsg:   "failed",
			wantCode:    codes.FailedPrecondition,
			wantMsgPart: "referenced by active config versions",
		},
		{
			name:        "other error maps to Internal",
			err:         errors.New("connection reset"),
			notFoundMsg: "not found",
			failedMsg:   "operation failed",
			wantCode:    codes.Internal,
			wantMsgPart: "operation failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := errToStatus(tc.err, tc.notFoundMsg, tc.failedMsg)
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tc.wantCode, st.Code())
			assert.Contains(t, st.Message(), tc.wantMsgPart)
		})
	}
}
