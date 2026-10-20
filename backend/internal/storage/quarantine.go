package storage

import (
	"context"
	"fmt"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/storage/sqlc"
)

func (s *Store) Quarantine(ctx context.Context, sourceID string, q raw.Quarantined) (inserted bool, err error) {
	n, err := sqlc.New(s.pool).InsertQuarantine(ctx, sqlc.InsertQuarantineParams{
		SourceID:     sourceID,
		ElementIndex: int64(q.Index),
		Raw:          q.Raw,
		Error:        q.Err.Error(),
	})
	if err != nil {
		return false, fmt.Errorf("insert quarantine: %w", mapErr(err))
	}
	return n == 1, nil
}
