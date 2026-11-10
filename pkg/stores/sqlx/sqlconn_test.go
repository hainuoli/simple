package sqlx

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewConnRequiresDriverName(t *testing.T) {
	_, err := NewConn(context.Background(), SqlConf{})
	require.EqualError(t, err, "sqlx: driver name is required")
}

func TestNewConnInvalidPool(t *testing.T) {
	tests := []struct {
		name string
		pool PoolConf
		want string
	}{
		{
			name: "negative max open",
			pool: PoolConf{MaxOpenConns: -1},
			want: "sqlx: invalid pool configuration",
		},
		{
			name: "negative max idle",
			pool: PoolConf{MaxIdleConns: -1},
			want: "sqlx: invalid pool configuration",
		},
		{
			name: "negative max lifetime",
			pool: PoolConf{ConnMaxLifetime: -time.Second},
			want: "sqlx: invalid pool configuration",
		},
		{
			name: "negative max idle time",
			pool: PoolConf{ConnMaxIdleTime: -time.Second},
			want: "sqlx: invalid pool configuration",
		},
		{
			name: "idle exceeds open",
			pool: PoolConf{MaxOpenConns: 1, MaxIdleConns: 2},
			want: "sqlx: max idle connections exceeds max open connections",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewConn(context.Background(), SqlConf{
				DriverName: "unused",
				Pool:       &tt.pool,
			})
			require.EqualError(t, err, tt.want)
		})
	}
}

func TestNewConnUnknownDriver(t *testing.T) {
	_, err := NewConn(context.Background(), SqlConf{
		DriverName: "sqlx_driver_that_does_not_exist",
		DataSource: "dsn",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "sqlx: open:")
}

func TestNewConnOpenError(t *testing.T) {
	name := registerFakeDriver(t, &fakeDriver{openErr: errOpen})

	_, err := NewConn(context.Background(), SqlConf{
		DriverName: name,
		DataSource: "dsn",
	})
	require.ErrorIs(t, err, errOpen)
}

func TestNewConnPingError(t *testing.T) {
	conn := &fakeConn{pingErr: errPing}

	_, err := NewConn(context.Background(), fakeConf(t, conn, nil))
	require.ErrorIs(t, err, errPing)
}

func TestNewConnWithNilPoolUsesDefaults(t *testing.T) {
	conn := &fakeConn{}

	got, err := NewConn(context.Background(), fakeConf(t, conn, nil))
	require.NoError(t, err)
	require.NotNil(t, got.DB)
	require.Zero(t, got.Stats().MaxOpenConnections)
	require.NoError(t, got.Close())
	require.True(t, conn.closed)
}

func TestNewConnAppliesPool(t *testing.T) {
	conn := &fakeConn{}
	pool := &PoolConf{
		MaxOpenConns:    5,
		MaxIdleConns:    3,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Second,
	}

	got, err := NewConn(context.Background(), fakeConf(t, conn, pool))
	require.NoError(t, err)
	require.Equal(t, 5, got.Stats().MaxOpenConnections)
	require.NoError(t, got.Close())
	require.True(t, conn.closed)
}
