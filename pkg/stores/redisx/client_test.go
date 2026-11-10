package redisx

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	mr := miniredis.RunT(t)

	client, err := New(context.Background(), Conf{
		Addr:        mr.Addr(),
		PingTimeout: time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	require.NoError(t, client.Set(ctx, "greeting", "hello", 0).Err())

	value, err := client.Get(ctx, "greeting").Result()
	require.NoError(t, err)
	assert.Equal(t, "hello", value)

	stats := client.PoolStats()
	require.NotNil(t, stats)
}

func TestNewEmptyAddr(t *testing.T) {
	client, err := New(context.Background(), Conf{})
	require.ErrorIs(t, err, ErrEmptyAddr)
	assert.Nil(t, client)
}

func TestNewPingFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	client, err := New(context.Background(), Conf{
		Addr:        addr,
		DialTimeout: 50 * time.Millisecond,
		PingTimeout: 100 * time.Millisecond,
		MaxRetries:  -1,
	})
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "redisx: ping")
}

func TestNewCanceledContext(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := New(ctx, Conf{Addr: mr.Addr()})
	require.Error(t, err)
	assert.Nil(t, client)
}

func TestMustNew(t *testing.T) {
	mr := miniredis.RunT(t)

	client := MustNew(context.Background(), Conf{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	assert.NotNil(t, client)

	assert.Panics(t, func() {
		MustNew(context.Background(), Conf{})
	})
}

func TestIsMiss(t *testing.T) {
	assert.True(t, IsMiss(redis.Nil))
	assert.False(t, IsMiss(errors.New("boom")))
	assert.False(t, IsMiss(nil))
}
