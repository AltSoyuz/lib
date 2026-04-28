package db

import (
	"database/sql"
	"testing"
)

func TestNullableFloat(t *testing.T) {
	f := func(input float64, wantValid bool, wantValue float64) {
		t.Helper()
		got := NullableFloat(input)
		if got.Valid != wantValid {
			t.Fatalf("NullableFloat(%v): Valid = %v, want %v", input, got.Valid, wantValid)
		}
		if got.Float64 != wantValue {
			t.Fatalf("NullableFloat(%v): Float64 = %v, want %v", input, got.Float64, wantValue)
		}
	}
	t.Run("non-zero", func(t *testing.T) { f(1.23, true, 1.23) })
	t.Run("zero", func(t *testing.T) { f(0, false, 0) })
}

func TestBoolToInt(t *testing.T) {
	f := func(input bool, want int64) {
		t.Helper()
		got := BoolToInt(input)
		if got != want {
			t.Fatalf("BoolToInt(%v) = %v, want %v", input, got, want)
		}
	}
	t.Run("true", func(t *testing.T) { f(true, 1) })
	t.Run("false", func(t *testing.T) { f(false, 0) })
}

func TestFloatOrZero(t *testing.T) {
	f := func(input sql.NullFloat64, want float64) {
		t.Helper()
		got := FloatOrZero(input)
		if got != want {
			t.Fatalf("FloatOrZero(%v) = %v, want %v", input, got, want)
		}
	}
	t.Run("valid", func(t *testing.T) { f(sql.NullFloat64{Float64: 2.5, Valid: true}, 2.5) })
	t.Run("invalid", func(t *testing.T) { f(sql.NullFloat64{Float64: 7.7, Valid: false}, 0) })
}

func TestInt64OrZero(t *testing.T) {
	f := func(input sql.NullInt64, want int64) {
		t.Helper()
		got := Int64OrZero(input)
		if got != want {
			t.Fatalf("Int64OrZero(%v) = %v, want %v", input, got, want)
		}
	}
	t.Run("valid", func(t *testing.T) { f(sql.NullInt64{Int64: 42, Valid: true}, 42) })
	t.Run("invalid", func(t *testing.T) { f(sql.NullInt64{Int64: 99, Valid: false}, 0) })
}
