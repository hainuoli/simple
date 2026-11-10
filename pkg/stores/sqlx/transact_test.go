package sqlx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/require"
)

func openTxDB(t *testing.T, tx driver.Tx) (*sql.DB, *fakeConn) {
	t.Helper()

	conn := &fakeConn{
		beginFn: func(context.Context, driver.TxOptions) (driver.Tx, error) {
			return tx, nil
		},
	}
	return openFakeDB(t, conn), conn
}

func TestTransactCommits(t *testing.T) {
	tx := &fakeTx{}
	db, conn := openTxDB(t, tx)

	var got *sql.Tx
	err := Transact(context.Background(), db, func(tx *sql.Tx) error {
		got = tx
		return nil
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, conn.beginCalled)
	require.Equal(t, 1, tx.commitCalls)
}

func TestTransactRollsBackOnError(t *testing.T) {
	tx := &fakeTx{}
	db, _ := openTxDB(t, tx)

	err := Transact(context.Background(), db, func(*sql.Tx) error {
		return errFn
	})
	require.ErrorIs(t, err, errFn)
	require.Equal(t, 0, tx.commitCalls)
	require.Equal(t, 1, tx.rollbackCalls)
}

func TestTransactBeginError(t *testing.T) {
	db := openFakeDB(t, &fakeConn{
		beginFn: func(context.Context, driver.TxOptions) (driver.Tx, error) {
			return nil, errBegin
		},
	})

	err := Transact(context.Background(), db, func(*sql.Tx) error {
		return nil
	})
	require.ErrorIs(t, err, errBegin)
}

func TestTransactCommitError(t *testing.T) {
	tx := &fakeTx{commitErr: errCommit}
	db, _ := openTxDB(t, tx)

	err := Transact(context.Background(), db, func(*sql.Tx) error {
		return nil
	})
	require.ErrorIs(t, err, errCommit)
	require.Equal(t, 1, tx.commitCalls)
	// database/sql marks the transaction done on Commit, so the deferred
	// Rollback returns ErrTxDone without reaching the driver.
	require.Zero(t, tx.rollbackCalls)
}

func TestTransactOptions(t *testing.T) {
	tx := &fakeTx{}
	var got driver.TxOptions
	db := openFakeDB(t, &fakeConn{
		beginFn: func(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
			got = opts
			return tx, nil
		},
	})

	err := TransactOptions(
		context.Background(),
		db,
		&sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true},
		func(*sql.Tx) error { return nil },
	)
	require.NoError(t, err)
	require.Equal(t, driver.IsolationLevel(sql.LevelSerializable), got.Isolation)
	require.True(t, got.ReadOnly)
}

func TestTransactPanicsAfterRollback(t *testing.T) {
	tx := &fakeTx{}
	db, _ := openTxDB(t, tx)

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic to propagate")
		}
		require.Equal(t, "boom", recovered)
		require.Equal(t, 0, tx.commitCalls)
		require.Equal(t, 1, tx.rollbackCalls)
	}()

	_ = Transact(context.Background(), db, func(*sql.Tx) error {
		panic("boom")
	})
}

func TestTransactWithSqlConn(t *testing.T) {
	tx := &fakeTx{}
	db, _ := openTxDB(t, tx)

	err := Transact(context.Background(), &SqlConn{DB: db}, func(*sql.Tx) error {
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, tx.commitCalls)
}
