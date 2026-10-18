package postgres

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrInvalid  = errors.New("invalid posting row")
	ErrNotFound = errors.New("posting not found")
)

const sqlStateCheckViolation = "23514"

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlStateCheckViolation {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	return err
}
