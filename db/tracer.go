package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/AltSoyuz/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/mattn/go-sqlite3"
)

// Tracer configures query tracing for the wrapped SQLite driver.
type Tracer struct {
	SlowThresh  time.Duration // 0 = off
	MaskArgs    bool
	SampleEvery int // 0/1 => all; N => 1/N
}

type tracedDriver struct {
	base driver.Driver
	t    *Tracer
}

func (d tracedDriver) Open(name string) (driver.Conn, error) {
	c, err := d.base.Open(name)
	if err != nil {
		return nil, err
	}
	return &tracedConn{base: c, t: d.t}, nil
}

type tracedConn struct {
	base driver.Conn
	t    *Tracer

	n uint64
}

func (c *tracedConn) Prepare(q string) (driver.Stmt, error) { return c.base.Prepare(q) }
func (c *tracedConn) Close() error                          { return c.base.Close() }

func (c *tracedConn) Begin() (driver.Tx, error) {
	return nil, driver.ErrSkip
}

func (c *tracedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	cbtx, ok := c.base.(driver.ConnBeginTx)
	if !ok {
		return nil, driver.ErrSkip
	}
	return cbtx.BeginTx(ctx, opts)
}

func (c *tracedConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	ce, ok := c.base.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	res, err := ce.ExecContext(ctx, q, args)
	c.log("exec", q, args, time.Since(start), err)
	return res, err
}

func (c *tracedConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	cq, ok := c.base.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	start := time.Now()
	rows, err := cq.QueryContext(ctx, q, args)
	c.log("query", q, args, time.Since(start), err)
	return rows, err
}

func (c *tracedConn) log(kind, sqlq string, args []driver.NamedValue, dur time.Duration, err error) {
	if c.t == nil {
		return
	}

	sqlq = compact(sqlq)

	metrics.GetOrCreateCounter(fmt.Sprintf(`app_db_queries_total{kind=%q}`, kind)).Inc()
	metrics.GetOrCreateHistogram(fmt.Sprintf(`app_db_query_duration_seconds{kind=%q}`, kind)).Update(dur.Seconds())

	if err != nil {
		metrics.GetOrCreateCounter(fmt.Sprintf(`app_db_query_errors_total{kind=%q}`, kind)).Inc()
		logger.ErrorSkipframes(2, "db.query.error",
			"kind", kind,
			"sql", sqlq,
			"args", maskArgs(args, c.t.MaskArgs),
			"dur", dur,
			"err", err.Error(),
		)
		return
	}

	if c.t.SlowThresh <= 0 || dur < c.t.SlowThresh {
		return
	}
	if !c.sampled() {
		return
	}

	logger.WarnSkipframes(2, "db.query.slow",
		"kind", kind,
		"sql", sqlq,
		"dur", dur,
	)
}

func (c *tracedConn) sampled() bool {
	n := c.t.SampleEvery
	if n <= 1 {
		return true
	}
	c.n++
	return c.n%uint64(n) == 0
}

func compact(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func maskArgs(args []driver.NamedValue, mask bool) any {
	if mask {
		return "[masked]"
	}
	return args
}

// TracedDriverName is the database/sql driver name registered by RegisterTracedDriver.
const TracedDriverName = "sqlite3-traced"

var registerOnce sync.Once

// RegisterTracedDriver registers the traced SQLite driver once per process.
func RegisterTracedDriver(tr *Tracer) {
	registerOnce.Do(func() {
		sql.Register(TracedDriverName, tracedDriver{
			base: &sqlite3.SQLiteDriver{},
			t:    tr,
		})
	})
}
