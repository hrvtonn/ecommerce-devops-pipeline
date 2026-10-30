package repository

import (
	"errors"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// args accumulates positional query parameters.
type args []any

// add appends v and returns its placeholder ($1, $2, ...).
func (a *args) add(v any) string {
	*a = append(*a, v)
	return "$" + strconv.Itoa(len(*a))
}

// setClause builds the SET list of a partial UPDATE.
type setClause struct {
	args
	parts []string
}

func (s *setClause) set(column string, value any) {
	s.parts = append(s.parts, column+" = "+s.add(value))
}

func (s *setClause) empty() bool { return len(s.parts) == 0 }

func (s *setClause) String() string { return strings.Join(s.parts, ", ") }

func where(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(conditions, " AND ")
}

func offset(page, limit int) int { return (page - 1) * limit }

// one turns a single-row scan result into a pointer, mapping "no rows" to nil.
func one[T any](v T, err error) (*T, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// many scans every row; the result is never nil so it encodes as [] in JSON.
func many[T any](rows pgx.Rows, err error, scan func(pgx.Row) (T, error)) ([]T, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
