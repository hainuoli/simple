package sqlx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// SqlConn holds a database/sql connection pool.
//
// It embeds *sql.DB, so ExecContext, QueryContext, QueryRowContext, BeginTx,
// PingContext, Stats, Close, and every other standard library method remain
// available without re-wrapping.
type SqlConn struct {
	*sql.DB
}

// NewConn opens and verifies a database connection pool.
//
// sql.Open is lazy: it validates the driver name but does not connect. NewConn
// therefore pings the database once so configuration errors surface at startup
// instead of on the first request.
//
// The caller owns the returned connection and must close it.
func NewConn(ctx context.Context, c SqlConf) (*SqlConn, error) {
	if c.DriverName == "" {
		return nil, errors.New("sqlx: driver name is required")
	}

	if c.Pool != nil {
		p := c.Pool
		if p.MaxOpenConns < 0 || p.MaxIdleConns < 0 ||
			p.ConnMaxLifetime < 0 || p.ConnMaxIdleTime < 0 {
			return nil, errors.New("sqlx: invalid pool configuration")
		}
		if p.MaxOpenConns > 0 && p.MaxIdleConns > p.MaxOpenConns {
			return nil, errors.New("sqlx: max idle connections exceeds max open connections")
		}
	}

	db, err := sql.Open(c.DriverName, c.DataSource)
	if err != nil {
		return nil, fmt.Errorf("sqlx: open: %w", err)
	}

	if c.Pool != nil {
		p := c.Pool
		db.SetMaxOpenConns(p.MaxOpenConns)
		db.SetMaxIdleConns(p.MaxIdleConns)
		db.SetConnMaxLifetime(p.ConnMaxLifetime)
		db.SetConnMaxIdleTime(p.ConnMaxIdleTime)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlx: ping: %w", err)
	}

	return &SqlConn{DB: db}, nil
}
