package db

import (
	"context"
	"database/sql/driver"
	"errors"
	"testing"
	"time"
)

func TestCompact(t *testing.T) {
	f := func(input, want string) {
		t.Helper()
		got := compact(input)
		if got != want {
			t.Fatalf("compact(%q) = %q; want %q", input, got, want)
		}
	}

	t.Run("single line", func(t *testing.T) {
		f("SELECT * FROM users", "SELECT * FROM users")
	})
	t.Run("multiline", func(t *testing.T) {
		f("SELECT *\nFROM users\nWHERE id = 1", "SELECT * FROM users WHERE id = 1")
	})
	t.Run("extra spaces", func(t *testing.T) {
		f("SELECT   *   FROM    users", "SELECT * FROM users")
	})
	t.Run("tabs and newlines", func(t *testing.T) {
		f("SELECT\t*\n  FROM\n\tusers", "SELECT * FROM users")
	})
}

func TestMaskArgs(t *testing.T) {
	args := []driver.NamedValue{
		{Ordinal: 1, Value: "secret"},
		{Ordinal: 2, Value: 123},
	}

	f := func(mask bool, wantMasked bool) {
		t.Helper()
		got := maskArgs(args, mask)
		if wantMasked {
			if s, ok := got.(string); !ok || s != "[masked]" {
				t.Fatalf("maskArgs(args, %v) = %v; want [masked]", mask, got)
			}
		} else {
			if _, ok := got.([]driver.NamedValue); !ok {
				t.Fatalf("maskArgs(args, %v) returned wrong type", mask)
			}
		}
	}

	t.Run("masked", func(t *testing.T) { f(true, true) })
	t.Run("unmasked", func(t *testing.T) { f(false, false) })
}

func TestTracedConnSampled(t *testing.T) {
	f := func(sampleEvery int, calls int, expectedSampled int) {
		t.Helper()
		tr := &Tracer{SampleEvery: sampleEvery}
		conn := &tracedConn{t: tr}

		sampled := 0
		for i := 0; i < calls; i++ {
			if conn.sampled() {
				sampled++
			}
		}

		if sampled != expectedSampled {
			t.Fatalf("with SampleEvery=%d, %d calls: got %d sampled; want %d",
				sampleEvery, calls, sampled, expectedSampled)
		}
	}

	t.Run("all sampled when 0", func(t *testing.T) { f(0, 10, 10) })
	t.Run("all sampled when 1", func(t *testing.T) { f(1, 10, 10) })
	t.Run("every 2nd", func(t *testing.T) { f(2, 10, 5) })
	t.Run("every 3rd", func(t *testing.T) { f(3, 9, 3) })
	t.Run("every 5th", func(t *testing.T) { f(5, 15, 3) })
}

// fakeDriver implements driver.Driver
type fakeDriver struct {
	openErr error
}

func (d *fakeDriver) Open(name string) (driver.Conn, error) {
	if d.openErr != nil {
		return nil, d.openErr
	}
	return &fakeConn{}, nil
}

// fakeConn implements driver.Conn with ExecerContext and QueryerContext
type fakeConn struct {
	execErr  error
	queryErr error
	execDur  time.Duration
	queryDur time.Duration
}

func (c *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, driver.ErrSkip
}

func (c *fakeConn) Close() error {
	return nil
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c *fakeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if c.execDur > 0 {
		time.Sleep(c.execDur)
	}
	return nil, c.execErr
}

func (c *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.queryDur > 0 {
		time.Sleep(c.queryDur)
	}
	return nil, c.queryErr
}

func TestTracedDriver(t *testing.T) {
	t.Run("successful open", func(t *testing.T) {
		tr := &Tracer{}
		d := tracedDriver{base: &fakeDriver{}, t: tr}
		conn, err := d.Open("test.db")
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		if conn == nil {
			t.Fatal("expected non-nil connection")
		}
		tc, ok := conn.(*tracedConn)
		if !ok {
			t.Fatal("expected *tracedConn")
		}
		if tc.t != tr {
			t.Fatal("tracer not set correctly")
		}
	})

	t.Run("open error", func(t *testing.T) {
		wantErr := errors.New("open failed")
		d := tracedDriver{base: &fakeDriver{openErr: wantErr}, t: &Tracer{}}
		_, err := d.Open("test.db")
		if err != wantErr {
			t.Fatalf("got err %v; want %v", err, wantErr)
		}
	})
}

func TestTracedConnExecContext(t *testing.T) {
	t.Run("delegates to base", func(t *testing.T) {
		base := &fakeConn{}
		tr := &Tracer{}
		tc := &tracedConn{base: base, t: tr}

		_, err := tc.ExecContext(context.Background(), "INSERT INTO test VALUES (1)", nil)
		if err != nil {
			t.Fatalf("ExecContext failed: %v", err)
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		wantErr := errors.New("exec error")
		base := &fakeConn{execErr: wantErr}
		tc := &tracedConn{base: base, t: &Tracer{}}

		_, err := tc.ExecContext(context.Background(), "INSERT INTO test VALUES (1)", nil)
		if err != wantErr {
			t.Fatalf("got err %v; want %v", err, wantErr)
		}
	})
}

func TestTracedConnQueryContext(t *testing.T) {
	t.Run("delegates to base", func(t *testing.T) {
		base := &fakeConn{}
		tr := &Tracer{}
		tc := &tracedConn{base: base, t: tr}

		_, err := tc.QueryContext(context.Background(), "SELECT * FROM test", nil)
		if err != nil {
			t.Fatalf("QueryContext failed: %v", err)
		}
	})

	t.Run("propagates error", func(t *testing.T) {
		wantErr := errors.New("query error")
		base := &fakeConn{queryErr: wantErr}
		tc := &tracedConn{base: base, t: &Tracer{}}

		_, err := tc.QueryContext(context.Background(), "SELECT * FROM test", nil)
		if err != wantErr {
			t.Fatalf("got err %v; want %v", err, wantErr)
		}
	})
}

func TestTracedConnBeginTx(t *testing.T) {
	t.Run("returns skip when base doesn't support it", func(t *testing.T) {
		base := &fakeConn{}
		tc := &tracedConn{base: base, t: &Tracer{}}

		_, err := tc.BeginTx(context.Background(), driver.TxOptions{})
		if err != driver.ErrSkip {
			t.Fatalf("got err %v; want driver.ErrSkip", err)
		}
	})
}

func TestTracedConnPrepareClose(t *testing.T) {
	base := &fakeConn{}
	tc := &tracedConn{base: base, t: &Tracer{}}

	t.Run("prepare delegates", func(t *testing.T) {
		_, err := tc.Prepare("SELECT 1")
		if err != driver.ErrSkip {
			t.Fatalf("got err %v; want driver.ErrSkip", err)
		}
	})

	t.Run("close delegates", func(t *testing.T) {
		err := tc.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
}
