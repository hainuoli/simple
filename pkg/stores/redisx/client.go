package redisx

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

// Client is a Redis connection handle.
//
// It embeds redis.UniversalClient, so every go-redis method is available
// unchanged: Get, Set, pipelines, Lua scripts, pub/sub, transactions, AddHook,
// PoolStats, and commands added by future go-redis releases. Client adds no
// command wrappers; typed cache helpers live in Store.
//
// A Client is safe for concurrent use and owns its connection pool. Close it
// when the process or component shuts down.
type Client struct {
	redis.UniversalClient
}

// Compile-time guarantee: a Client can be used anywhere a cache command set is
// expected, including redisx.NewStore.
var _ redis.Cmdable = (*Client)(nil)

// New validates the configuration, builds the client, and pings the server so
// misconfiguration surfaces at startup instead of on the first request.
//
// The caller owns the returned Client and must close it.
func New(ctx context.Context, c Conf) (*Client, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}

	client := &Client{UniversalClient: redis.NewUniversalClient(c.options())}

	pingCtx, cancel := context.WithTimeout(ctx, c.pingTimeout())
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redisx: ping %s: %w", strings.Join(c.addrs(), ","), err)
	}

	return client, nil
}

// MustNew is New for startup code that treats a Redis failure as fatal.
// It panics with the error returned by New.
func MustNew(ctx context.Context, c Conf) *Client {
	client, err := New(ctx, c)
	if err != nil {
		panic(err)
	}
	return client
}

// IsMiss reports whether err means the key does not exist.
//
// It is a convenience for raw go-redis calls; Store.Get already turns a miss
// into (zero, false, nil), so most application code never needs this.
func IsMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}
