package sqlx

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	rows := &fakeRows{values: [][]driver.Value{{int64(7), "alice"}}}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	got, err := Get[testUser](
		context.Background(),
		db,
		"SELECT id, name FROM users WHERE id = ?",
		7,
	)
	require.NoError(t, err)
	require.Equal(t, testUser{ID: 7, Name: "alice"}, got)
	require.True(t, rows.closed)
}

func TestGetWithinTransaction(t *testing.T) {
	rows := &fakeRows{values: [][]driver.Value{{int64(9), "tx"}}}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	got, err := Get[testUser](context.Background(), tx, "SELECT id, name FROM users")
	require.NoError(t, err)
	require.Equal(t, testUser{ID: 9, Name: "tx"}, got)
	require.NoError(t, tx.Rollback())
}

func TestGetQueryError(t *testing.T) {
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return nil, errQuery
		},
	})

	got, err := Get[testUser](context.Background(), db, "SELECT id, name FROM users")
	require.ErrorIs(t, err, errQuery)
	require.Equal(t, testUser{}, got)
}

func TestGetScanError(t *testing.T) {
	rows := &fakeRows{values: [][]driver.Value{{int64(1), "alice"}}}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	got, err := Get[failingModel](context.Background(), db, "SELECT id, name FROM users")
	require.ErrorIs(t, err, errScan)
	require.Equal(t, failingModel{}, got)
}

func TestSelect(t *testing.T) {
	rows := &fakeRows{values: [][]driver.Value{
		{int64(1), "alice"},
		{int64(2), "bob"},
	}}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	got, err := Select[testUser](context.Background(), db, "SELECT id, name FROM users")
	require.NoError(t, err)
	require.Equal(t, []testUser{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
	}, got)
	require.True(t, rows.closed)
}

func TestSelectEmptyReturnsNonNilSlice(t *testing.T) {
	rows := &fakeRows{}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	got, err := Select[testUser](context.Background(), db, "SELECT id, name FROM users")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Empty(t, got)
	require.True(t, rows.closed)
}

func TestSelectQueryError(t *testing.T) {
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return nil, errQuery
		},
	})

	got, err := Select[testUser](context.Background(), db, "SELECT id, name FROM users")
	require.ErrorIs(t, err, errQuery)
	require.Nil(t, got)
}

func TestSelectScanError(t *testing.T) {
	rows := &fakeRows{values: [][]driver.Value{{int64(1), "alice"}}}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	got, err := Select[failingModel](context.Background(), db, "SELECT id, name FROM users")
	require.ErrorIs(t, err, errScan)
	require.Nil(t, got)
	require.True(t, rows.closed)
}

func TestSelectRowsError(t *testing.T) {
	rows := &fakeRows{
		values: [][]driver.Value{{int64(1), "alice"}},
		err:    errRows,
	}
	db := openFakeDB(t, &fakeConn{
		queryFn: func(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
			return rows, nil
		},
	})

	got, err := Select[testUser](context.Background(), db, "SELECT id, name FROM users")
	require.ErrorIs(t, err, errRows)
	require.Nil(t, got)
	require.True(t, rows.closed)
}
