package infrastructure

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Helpers
func nullableInt(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableBool(p *bool) any {
	if p == nil {
		return nil
	}
	return *p
}

func parseYM(yyyymmdd string) int {
	if len(yyyymmdd) >= 6 {
		var y, m int
		fmt.Sscanf(yyyymmdd[:6], "%04d%02d", &y, &m)
		return y*100 + m
	}
	return 0
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "already exists")
}

func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	// Check for pgx PgError with code 23503 (foreign_key_violation)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	// Fallback to string matching
	msg := err.Error()
	return strings.Contains(msg, "23503") || strings.Contains(msg, "foreign key") || strings.Contains(msg, "violates foreign key")
}
