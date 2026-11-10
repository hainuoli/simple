package sqlx

import (
	"context"
	"database/sql"
)

// beginner is the subset of *sql.DB and *SqlConn needed to start a transaction.
type beginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

// Compile-time guarantee: both pool handles can start transactions.
var (
	_ beginner = (*sql.DB)(nil)
	_ beginner = (*SqlConn)(nil)
)

// Transact runs fn inside a transaction and commits when fn returns nil.
//
// A deferred Rollback runs on every path, so an error or panic does not leak a
// transaction. BeginTx errors are returned unchanged; otherwise the returned
// error is fn's or Commit's, and the standard library's ErrTxDone from the
// deferred Rollback is ignored.
func Transact(ctx context.Context, db beginner, fn func(*sql.Tx) error) error {
	return TransactOptions(ctx, db, nil, fn)
}

// TransactOptions is Transact with explicit transaction options.
//
// opts is passed to BeginTx unchanged; nil keeps the driver defaults.
func TransactOptions(
	ctx context.Context,
	db beginner,
	opts *sql.TxOptions,
	fn func(*sql.Tx) error,
) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
