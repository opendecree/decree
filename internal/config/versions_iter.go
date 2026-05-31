package config

import (
	"context"
	"iter"

	"github.com/opendecree/decree/internal/pagination"
	"github.com/opendecree/decree/internal/storage/domain"
)

const allVersionsPageSize = int32(500)

// allVersions returns a server-side iterator over every config version for
// tenantID, newest first.  Uses [pagination.Iter]; see that function for the
// usage pattern.
//
// Pilot for the iter.Seq list pattern — apply the same shape to other list
// endpoints (schema.ListSchemas, audit.QueryWriteLog, etc.) as needed:
//
//  1. Add a private *Service method that calls pagination.Iter with a fetch
//     closure wrapping the relevant store query.
//  2. The fetch closure decodes the incoming token, calls the store with
//     Limit = pageSize+1, computes the next token via pagination.NextPageToken,
//     trims the slice, and returns.
//  3. Tests must cover full iteration, early break, and store error.
//
// Server module only (Go 1.25).  Do not replicate in SDK packages.
func (s *Service) allVersions(ctx context.Context, tenantID string) iter.Seq2[domain.ConfigVersion, error] {
	return pagination.Iter(ctx, func(ctx context.Context, pageToken string) ([]domain.ConfigVersion, string, error) {
		offset, err := pagination.DecodePageToken(pageToken)
		if err != nil {
			return nil, "", err
		}
		versions, err := s.store.ListConfigVersions(ctx, ListConfigVersionsParams{
			TenantID: tenantID,
			Limit:    allVersionsPageSize + 1,
			Offset:   offset,
		})
		if err != nil {
			return nil, "", err
		}
		next := pagination.NextPageToken(allVersionsPageSize, int32(len(versions)), offset)
		if int32(len(versions)) > allVersionsPageSize {
			versions = versions[:allVersionsPageSize]
		}
		return versions, next, nil
	})
}
