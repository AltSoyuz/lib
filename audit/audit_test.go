package audit

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schema string

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestWriteCommitsWithTheChangeOrNotAtAll(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(ctx, tx, "u1", RoleChanged, "u2", map[string]any{"from": "user", "to": "admin"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	events, _, err := List(ctx, db, Filter{Limit: 10})
	if err != nil || len(events) != 0 {
		t.Fatalf("expected no event after a rollback, got %d err=%v", len(events), err)
	}

	tx, _ = db.BeginTx(ctx, nil)
	if err := Write(ctx, tx, "u1", RoleChanged, "u2", map[string]any{"to": "admin"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	events, _, err = List(ctx, db, Filter{Limit: 10})
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event after a commit, got %d err=%v", len(events), err)
	}
	var d map[string]string
	if err := json.Unmarshal(events[0].Details, &d); err != nil || d["to"] != "admin" {
		t.Fatalf("expected details {to: admin}, got %s", events[0].Details)
	}
}

func TestListPagesNewestFirstAndFilters(t *testing.T) {
	ctx := context.Background()
	db := openDB(t)
	for i := 0; i < 5; i++ {
		After(ctx, db, "a", SignedIn, "a", nil)
	}
	After(ctx, db, "", SignedOut, "b", nil)

	page, next, err := List(ctx, db, Filter{Limit: 4})
	if err != nil || len(page) != 4 || page[0].Action != SignedOut || page[0].ActorID != "" {
		t.Fatalf("expected 4 events, the system's sign out first, got %+v err=%v", page, err)
	}
	rest, last, err := List(ctx, db, Filter{Limit: 4, Before: next})
	if err != nil || len(rest) != 2 || last != 0 {
		t.Fatalf("expected the 2 last events and no next page, got %d next=%d err=%v", len(rest), last, err)
	}
	mine, _, _ := List(ctx, db, Filter{Limit: 10, ActorID: "a", Action: SignedIn})
	if len(mine) != 5 {
		t.Fatalf("expected 5 sign-ins of a, got %d", len(mine))
	}
	since, err := Since(ctx, db)
	if err != nil || since == "" {
		t.Fatalf("expected the time of the first event, got %q err=%v", since, err)
	}
}
