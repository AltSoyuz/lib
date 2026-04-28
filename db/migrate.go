package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/AltSoyuz/lib/logger"
)

type migration struct {
	v    int
	name string
	sql  string
}

func Migrate(ctx context.Context, db *sql.DB, dir fs.FS) error {
	start := time.Now()

	entries, err := fs.ReadDir(dir, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: readdir: %w", err)
	}

	var ms []migration
	seen := map[int]string{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		name := e.Name() // ex: 001_init.sql
		if strings.HasPrefix(name, ".") {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("migrate: bad name %q: %w", name, err)
		}
		if prev, ok := seen[v]; ok {
			return fmt.Errorf("migrate: duplicate version %d: %q and %q", v, prev, name)
		}
		seen[v] = name

		f, err := dir.Open("migrations/" + name)
		if err != nil {
			return fmt.Errorf("migrate: read %q: %w", name, err)
		}

		b, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("migrate: read %q: %w", name, err)
		}

		ms = append(ms, migration{
			v:    v,
			name: name,
			sql:  string(b),
		})
	}

	sort.Slice(ms, func(i, j int) bool { return ms[i].v < ms[j].v })

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate: begin: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
			logger.Error("db.migrate.rollback", "error", err)
		}
	}()

	if _, err := tx.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY);`,
	); err != nil {
		return fmt.Errorf("migrate: create table: %w", err)
	}

	var cur int
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`,
	).Scan(&cur); err != nil {
		return fmt.Errorf("migrate: current version: %w", err)
	}

	applied := 0
	for _, m := range ms {
		if m.v <= cur {
			continue
		}

		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("migrate: exec %d (%s): %w", m.v, m.name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version) VALUES (?)`, m.v,
		); err != nil {
			return fmt.Errorf("migrate: record %d: %w", m.v, err)
		}

		applied++
		logger.Info("db.migrate.applied", "version", m.v, "name", m.name)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate: commit: %w", err)
	}

	if applied > 0 {
		logger.Info("db.migrate.done",
			"applied", applied,
			"dur", time.Since(start),
		)
	}

	return nil
}
