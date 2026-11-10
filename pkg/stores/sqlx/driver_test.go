package sqlx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"testing"
)

var (
	errPing   = errors.New("ping failed")
	errQuery  = errors.New("query failed")
	errScan   = errors.New("scan failed")
	errBegin  = errors.New("begin failed")
	errCommit = errors.New("commit failed")
	errRows   = errors.New("rows failed")
	errOpen   = errors.New("open failed")
	errFn     = errors.New("fn failed")
)

var fakeDriverSeq int

// registerFakeDriver registers a uniquely named driver for one test. Tests in
// this package must not call t.Parallel because the sequence is not atomic.
func registerFakeDriver(t *testing.T, d driver.Driver) string {
	t.Helper()

	fakeDriverSeq++
	name := fmt.Sprintf("sqlx_fake_%d", fakeDriverSeq)
	sql.Register(name, d)
	return name
}

func openFakeDB(t *testing.T, conn driver.Conn) *sql.DB {
	t.Helper()

	name := registerFakeDriver(t, &fakeDriver{conn: conn})
	db, err := sql.Open(name, "dsn")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func fakeConf(t *testing.T, conn driver.Conn, pool *PoolConf) SqlConf {
	t.Helper()

	return SqlConf{
		DriverName: registerFakeDriver(t, &fakeDriver{conn: conn}),
		DataSource: "dsn",
		Pool:       pool,
	}
}

// fakeDriver returns a fixed connection, or a configured open error.
type fakeDriver struct {
	conn    driver.Conn
	openErr error
}

func (d *fakeDriver) Open(string) (driver.Conn, error) {
	if d.openErr != nil {
		return nil, d.openErr
	}
	if d.conn == nil {
		return &fakeConn{}, nil
	}
	return d.conn, nil
}

// fakeConn implements driver.Conn, driver.Pinger, driver.QueryerContext, and
// driver.ConnBeginTx.
type fakeConn struct {
	closed      bool
	pingErr     error
	queryFn     func(context.Context, string, []driver.NamedValue) (driver.Rows, error)
	beginFn     func(context.Context, driver.TxOptions) (driver.Tx, error)
	beginOpts   driver.TxOptions
	beginCalled bool
}

func (c *fakeConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fake: Prepare is not supported")
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

func (c *fakeConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *fakeConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.beginCalled = true
	c.beginOpts = opts
	if c.beginFn != nil {
		return c.beginFn(ctx, opts)
	}
	return &fakeTx{}, nil
}

func (c *fakeConn) Ping(context.Context) error {
	return c.pingErr
}

func (c *fakeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.queryFn != nil {
		return c.queryFn(ctx, query, args)
	}
	return &fakeRows{}, nil
}

// fakeTx records commit and rollback calls.
type fakeTx struct {
	commitErr     error
	rollbackErr   error
	commitCalls   int
	rollbackCalls int
}

func (t *fakeTx) Commit() error {
	t.commitCalls++
	return t.commitErr
}

func (t *fakeTx) Rollback() error {
	t.rollbackCalls++
	return t.rollbackErr
}

// fakeRows is an in-memory driver.Rows implementation.
type fakeRows struct {
	columns []string
	values  [][]driver.Value
	err     error
	index   int
	closed  bool
}

func (r *fakeRows) Columns() []string {
	if r.columns != nil {
		return r.columns
	}
	return []string{"id", "name"}
}

func (r *fakeRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.index < len(r.values) {
		copy(dest, r.values[r.index])
		r.index++
		return nil
	}
	if r.err != nil {
		return r.err
	}
	return io.EOF
}

// testUser is the reference ScanFrom implementation used by query tests.
type testUser struct {
	ID   int64
	Name string
}

func (u *testUser) ScanFrom(s RowScanner) error {
	return s.Scan(&u.ID, &u.Name)
}

// failingModel always fails while scanning.
type failingModel struct{}

func (*failingModel) ScanFrom(RowScanner) error {
	return errScan
}
