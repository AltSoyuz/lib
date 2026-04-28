package db

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// NullableFloat returns a valid sql.NullFloat64 when v is non-zero.
func NullableFloat(v float64) sql.NullFloat64 {
	return sql.NullFloat64{Float64: v, Valid: v != 0}
}

// BoolToInt returns 1 for true and 0 for false.
func BoolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// FloatOrZero returns the float64 value of a sql.NullFloat64 or zero if it's null.
func FloatOrZero(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}

// Int64OrZero returns the int64 value of a sql.NullInt64 or zero if it's null.
func Int64OrZero(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

// IsUniqueViolation returns true if the error is a SQLite UNIQUE constraint violation.
func IsUniqueViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}
	return false
}

// NullJoined returns a NullString by joining xs with commas.
// Returns an invalid NullString if xs is empty.
func NullJoined(xs []string) sql.NullString {
	if len(xs) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: strings.Join(xs, ","), Valid: true}
}
