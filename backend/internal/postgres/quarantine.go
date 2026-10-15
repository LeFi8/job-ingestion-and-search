package postgres

import (
	"context"
	"fmt"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/ingestion/raw"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres/sqlc"
)

func (s *Store) Quarantine(ctx context.Context, sourceID string, q raw.Quarantined) error {
	err := sqlc.New(s.pool).InsertQuarantine(ctx, sqlc.InsertQuarantineParams{
		SourceID:     sourceID,
		ElementIndex: int32(q.Index),
		Raw:          q.Raw,
		Error:        q.Err.Error(),
	})
	if err != nil {
		return fmt.Errorf("insert quarantine: %w", mapErr(err))
	}
	return nil
}
