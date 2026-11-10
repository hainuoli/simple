package sqlx

import (
	"context"
	"database/sql"
)

// DBTX is the shared data access surface of *sql.DB and *sql.Tx, so a Model can
// run inside or outside a transaction without knowing which one it holds.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// RowScanner is satisfied by *sql.Row and *sql.Rows.
type RowScanner interface {
	Scan(dest ...any) error
}

// scannable binds a value type to its pointer scanner. Keeping the constraint
// unexported means Model code only needs to implement ScanFrom.
type scannable[T any] interface {
	*T
	ScanFrom(RowScanner) error
}

// Compile-time guarantee: the pool handle, a transaction, and the standard
// library pool are all usable wherever a Model expects DBTX.
var (
	_ DBTX = (*sql.DB)(nil)
	_ DBTX = (*sql.Tx)(nil)
	_ DBTX = (*SqlConn)(nil)
)

// Get queries and scans a single row.
//
// It keeps database/sql semantics: no rows returns sql.ErrNoRows, and the SQL
// placeholder style is whatever the driver expects (? for MySQL, $1 for
// PostgreSQL).
func Get[T any, P scannable[T]](
	ctx context.Context,
	db DBTX,
	query string,
	args ...any,
) (T, error) {
	var item T
	err := P(&item).ScanFrom(db.QueryRowContext(ctx, query, args...))
	return item, err
}

// Select queries and scans multiple rows.
//
// It handles rows.Next, rows.Close, and rows.Err. No rows returns a non-nil
// empty slice, so callers never need a nil check before iterating.
func Select[T any, P scannable[T]](
	ctx context.Context,
	db DBTX,
	query string,
	args ...any,
) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]T, 0)
	for rows.Next() {
		var item T
		if err := P(&item).ScanFrom(rows); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
