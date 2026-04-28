package db

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrate(t *testing.T) {
	f := func(fs fstest.MapFS, expectErr bool, checkDB func(*testing.T, *sql.DB)) {
		t.Helper()

		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				t.Fatalf("close db: %v", err)
			}
		}()

		err = Migrate(context.Background(), db, fs)
		if expectErr && err == nil {
			t.Fatal("expected error but got nil")
		}
		if !expectErr && err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if checkDB != nil {
			checkDB(t, db)
		}
	}

	t.Run("successful migrations", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/001_init.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE users(id INTEGER PRIMARY KEY);`),
			},
			"migrations/002_add_posts.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE posts(id INTEGER PRIMARY KEY, user_id INTEGER);`),
			},
		}

		f(fs, false, func(t *testing.T, db *sql.DB) {
			var count int
			err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
			if err != nil {
				t.Fatalf("query migrations: %v", err)
			}
			if count != 2 {
				t.Errorf("expected 2 migrations, got %d", count)
			}

			var version int
			err = db.QueryRow(`SELECT MAX(version) FROM schema_migrations`).Scan(&version)
			if err != nil {
				t.Fatalf("query max version: %v", err)
			}
			if version != 2 {
				t.Errorf("expected version 2, got %d", version)
			}
		})
	})

	t.Run("idempotent migrations", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/001_init.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE users(id INTEGER PRIMARY KEY);`),
			},
		}

		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				t.Fatalf("close db: %v", err)
			}
		}()

		ctx := context.Background()

		// Run first time
		if err := Migrate(ctx, db, fs); err != nil {
			t.Fatalf("first migrate: %v", err)
		}

		// Run second time - should not fail
		if err := Migrate(ctx, db, fs); err != nil {
			t.Fatalf("second migrate: %v", err)
		}

		var count int
		err = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
		if err != nil {
			t.Fatalf("query migrations: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 migration record, got %d", count)
		}
	})

	t.Run("duplicate version error", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/001_init.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE users(id INTEGER PRIMARY KEY);`),
			},
			"migrations/001_duplicate.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE posts(id INTEGER PRIMARY KEY);`),
			},
		}

		f(fs, true, nil)
	})

	t.Run("invalid migration name", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/abc_invalid.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE users(id INTEGER PRIMARY KEY);`),
			},
		}

		f(fs, true, nil)
	})

	t.Run("sql execution error", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/001_init.sql": &fstest.MapFile{
				Data: []byte(`CREATE TABLE users(id INTEGER PRIMARY KEY);`),
			},
			"migrations/002_bad.sql": &fstest.MapFile{
				Data: []byte(`INVALID SQL SYNTAX HERE;`),
			},
		}

		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				t.Fatalf("close db: %v", err)
			}
		}()

		err = Migrate(context.Background(), db, fs)
		if err == nil {
			t.Fatal("expected error but got nil")
		}

		// Verify rollback - schema_migrations table should exist but be empty
		var exists int
		err = db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&exists)
		if err != nil {
			t.Fatalf("query table existence: %v", err)
		}
		if exists == 1 {
			t.Error("expected transaction rollback, but schema_migrations table exists")
		}
	})

	t.Run("empty migrations directory", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/": &fstest.MapFile{
				Mode: 0755 | fs.ModeDir,
			},
		}

		f(fs, false, func(t *testing.T, db *sql.DB) {
			var count int
			err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
			if err != nil {
				t.Fatalf("query migrations: %v", err)
			}
			if count != 0 {
				t.Errorf("expected 0 migrations, got %d", count)
			}
		})
	})

	t.Run("skip already applied migrations", func(t *testing.T) {
		fs := fstest.MapFS{
			"migrations/001_init.sql":         &fstest.MapFile{Data: []byte(`CREATE TABLE users(id INTEGER PRIMARY KEY);`)},
			"migrations/002_add_posts.sql":    &fstest.MapFile{Data: []byte(`CREATE TABLE posts(id INTEGER PRIMARY KEY);`)},
			"migrations/003_add_comments.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE comments(id INTEGER PRIMARY KEY);`)},
		}

		db, err := sql.Open("sqlite3", ":memory:")
		if err != nil {
			t.Fatalf("open db: %v", err)
		}
		defer func() {
			if err := db.Close(); err != nil {
				t.Fatalf("close db: %v", err)
			}
		}()

		ctx := context.Background()

		// Apply first two migrations
		fsPartial := fstest.MapFS{
			"migrations/001_init.sql":      fs["migrations/001_init.sql"],
			"migrations/002_add_posts.sql": fs["migrations/002_add_posts.sql"],
		}
		if err := Migrate(ctx, db, fsPartial); err != nil {
			t.Fatalf("first migrate: %v", err)
		}

		// Apply all three - should only apply the third
		if err := Migrate(ctx, db, fs); err != nil {
			t.Fatalf("second migrate: %v", err)
		}

		var count int
		err = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
		if err != nil {
			t.Fatalf("query migrations: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 migrations, got %d", count)
		}
	})
}
