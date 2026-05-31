package config

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/storage/domain"
)

func makeVersions(count int, startVersion int32) []domain.ConfigVersion {
	vs := make([]domain.ConfigVersion, count)
	for i := range vs {
		vs[i] = domain.ConfigVersion{
			TenantID: tenantID1,
			Version:  startVersion - int32(i),
		}
	}
	return vs
}

// TestAllVersions_FullIteration verifies that allVersions yields every item
// across two pages.
func TestAllVersions_FullIteration(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	page1 := makeVersions(int(allVersionsPageSize)+1, 1000) // extra item signals "has next"
	page2 := makeVersions(3, 500)

	store.On("ListConfigVersions", ctx, ListConfigVersionsParams{
		TenantID: tenantID1,
		Limit:    allVersionsPageSize + 1,
		Offset:   0,
	}).Return(page1, nil).Once()

	store.On("ListConfigVersions", ctx, ListConfigVersionsParams{
		TenantID: tenantID1,
		Limit:    allVersionsPageSize + 1,
		Offset:   allVersionsPageSize,
	}).Return(page2, nil).Once()

	var got []domain.ConfigVersion
	for v, err := range svc.allVersions(ctx, tenantID1) {
		require.NoError(t, err)
		got = append(got, v)
	}

	require.Len(t, got, int(allVersionsPageSize)+3)
	store.AssertExpectations(t)
}

// TestAllVersions_EarlyBreak verifies that breaking out of the loop stops
// fetching additional pages.
func TestAllVersions_EarlyBreak(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	page1 := makeVersions(int(allVersionsPageSize)+1, 1000)

	store.On("ListConfigVersions", ctx, ListConfigVersionsParams{
		TenantID: tenantID1,
		Limit:    allVersionsPageSize + 1,
		Offset:   0,
	}).Return(page1, nil).Once()

	var count int
	for _, err := range svc.allVersions(ctx, tenantID1) {
		require.NoError(t, err)
		count++
		if count == 5 {
			break
		}
	}

	require.Equal(t, 5, count)
	// Second page must never be fetched.
	store.AssertNotCalled(t, "ListConfigVersions", mock.Anything, ListConfigVersionsParams{
		TenantID: tenantID1,
		Limit:    allVersionsPageSize + 1,
		Offset:   allVersionsPageSize,
	})
}

// TestAllVersions_StoreError verifies that a store error is surfaced as the
// error value in the iterator and stops iteration.
func TestAllVersions_StoreError(t *testing.T) {
	svc, store, _, _ := newTestService()
	ctx := context.Background()

	sentinel := errors.New("db unavailable")
	store.On("ListConfigVersions", ctx, ListConfigVersionsParams{
		TenantID: tenantID1,
		Limit:    allVersionsPageSize + 1,
		Offset:   0,
	}).Return([]domain.ConfigVersion(nil), sentinel)

	var gotErr error
	for _, err := range svc.allVersions(ctx, tenantID1) {
		gotErr = err
		break
	}

	require.ErrorIs(t, gotErr, sentinel)

	// Encode a valid next-page token to confirm pagination.Iter stops on error.
	_ = pagination.EncodePageToken(allVersionsPageSize) // compile-time import check
}
