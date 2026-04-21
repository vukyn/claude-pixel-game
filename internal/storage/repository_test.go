package storage

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

type toyEntity struct {
	ID   string
	Name string
	Qty  int
}

func (t toyEntity) GetID() string { return t.ID }

type toyMapper struct{}

func (toyMapper) Table() string     { return "toys" }
func (toyMapper) Columns() []string { return []string{"id", "name", "qty"} }
func (toyMapper) Scan(row Scanner) (toyEntity, error) {
	var t toyEntity
	return t, row.Scan(&t.ID, &t.Name, &t.Qty)
}
func (toyMapper) Values(t toyEntity) []any { return []any{t.ID, t.Name, t.Qty} }

func openMemDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE toys (id TEXT PRIMARY KEY, name TEXT NOT NULL, qty INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	db := openMemDB(t)
	repo := NewRepository[toyEntity](db, toyMapper{})

	if err := repo.Upsert(ctx, toyEntity{ID: "a", Name: "Apple", Qty: 3}); err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, "a")
	if err != nil || got.Name != "Apple" || got.Qty != 3 {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	if err := repo.Upsert(ctx, toyEntity{ID: "a", Name: "Apple", Qty: 9}); err != nil {
		t.Fatal(err)
	}
	got, _ = repo.Get(ctx, "a")
	if got.Qty != 9 {
		t.Fatalf("upsert did not update: %+v", got)
	}

	if err := repo.Upsert(ctx, toyEntity{ID: "b", Name: "Banana", Qty: 1}); err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("list=%v err=%v", list, err)
	}

	if err := repo.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, "a"); err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows, got %v", err)
	}
}
