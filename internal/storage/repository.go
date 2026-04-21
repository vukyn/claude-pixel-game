package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Entity interface {
	GetID() string
}

type Scanner interface {
	Scan(dest ...any) error
}

type Mapper[T Entity] interface {
	Table() string
	Columns() []string
	Scan(row Scanner) (T, error)
	Values(t T) []any
}

type Repository[T Entity] struct {
	db     *sql.DB
	mapper Mapper[T]
}

func NewRepository[T Entity](db *sql.DB, m Mapper[T]) *Repository[T] {
	return &Repository[T]{db: db, mapper: m}
}

func (r *Repository[T]) Get(ctx context.Context, id string) (T, error) {
	var zero T
	cols := r.mapper.Columns()
	q := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", strings.Join(cols, ", "), r.mapper.Table(), cols[0])
	row := r.db.QueryRowContext(ctx, q, id)
	t, err := r.mapper.Scan(row)
	if err != nil {
		return zero, err
	}
	return t, nil
}

func (r *Repository[T]) List(ctx context.Context) ([]T, error) {
	cols := r.mapper.Columns()
	q := fmt.Sprintf("SELECT %s FROM %s", strings.Join(cols, ", "), r.mapper.Table())
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		t, err := r.mapper.Scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (r *Repository[T]) Upsert(ctx context.Context, t T) error {
	cols := r.mapper.Columns()
	placeholders := strings.Repeat("?,", len(cols))
	placeholders = placeholders[:len(placeholders)-1]

	updateParts := make([]string, 0, len(cols)-1)
	for _, c := range cols[1:] {
		updateParts = append(updateParts, fmt.Sprintf("%s=excluded.%s", c, c))
	}

	q := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT(%s) DO UPDATE SET %s",
		r.mapper.Table(),
		strings.Join(cols, ", "),
		placeholders,
		cols[0],
		strings.Join(updateParts, ", "),
	)
	_, err := r.db.ExecContext(ctx, q, r.mapper.Values(t)...)
	return err
}

func (r *Repository[T]) Delete(ctx context.Context, id string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", r.mapper.Table(), r.mapper.Columns()[0])
	_, err := r.db.ExecContext(ctx, q, id)
	return err
}
