package redisx

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type payload struct {
	N int
}

type badPayload struct {
	F float64
}

// fakeCmdable implements only the three commands Store uses. Embedding the
// interface keeps the fake small and makes unexpected calls panic.
type fakeCmdable struct {
	redis.Cmdable
	get func(context.Context, string) *redis.StringCmd
	set func(context.Context, string, any, time.Duration) *redis.StatusCmd
	del func(context.Context, ...string) *redis.IntCmd
}

func (f *fakeCmdable) Get(ctx context.Context, key string) *redis.StringCmd {
	return f.get(ctx, key)
}

func (f *fakeCmdable) Set(
	ctx context.Context,
	key string,
	value any,
	ttl time.Duration,
) *redis.StatusCmd {
	return f.set(ctx, key, value, ttl)
}

func (f *fakeCmdable) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return f.del(ctx, keys...)
}

func stringCmd(ctx context.Context, value string, err error) *redis.StringCmd {
	cmd := redis.NewStringCmd(ctx)
	cmd.SetVal(value)
	cmd.SetErr(err)
	return cmd
}

func statusCmd(ctx context.Context, err error) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if err == nil {
		cmd.SetVal("OK")
	}
	cmd.SetErr(err)
	return cmd
}

func intCmd(ctx context.Context, value int64, err error) *redis.IntCmd {
	cmd := redis.NewIntCmd(ctx)
	cmd.SetVal(value)
	cmd.SetErr(err)
	return cmd
}

func TestNewStoreKey(t *testing.T) {
	store := NewStore[payload](nil, " user: ")
	assert.Equal(t, "user", store.prefix)
	assert.Equal(t, "user", store.Key(""))
	assert.Equal(t, "user:42", store.Key("42"))

	plain := NewStore[payload](nil, "")
	assert.Equal(t, "42", plain.Key("42"))
}

func TestStoreGet(t *testing.T) {
	ctx := context.Background()

	t.Run("hit", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, `{"N":1}`, nil)
			},
		}, "user")

		value, ok, err := store.Get(ctx, "42")
		require.NoError(t, err)
		assert.True(t, ok)
		assert.Equal(t, payload{N: 1}, value)
	})

	t.Run("miss", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "", redis.Nil)
			},
		}, "user")

		value, ok, err := store.Get(ctx, "42")
		require.NoError(t, err)
		assert.False(t, ok)
		assert.Equal(t, payload{}, value)
	})

	t.Run("backend error", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "", errors.New("boom"))
			},
		}, "user")

		_, _, err := store.Get(ctx, "42")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `redisx: get "user:42"`)
	})

	t.Run("decode error", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "{", nil)
			},
		}, "user")

		_, _, err := store.Get(ctx, "42")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `redisx: decode "user:42"`)
	})
}

func TestStoreSet(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		var gotKey string
		var gotValue any
		var gotTTL time.Duration

		store := NewStore[payload](&fakeCmdable{
			set: func(_ context.Context, key string, value any, ttl time.Duration) *redis.StatusCmd {
				gotKey, gotValue, gotTTL = key, value, ttl
				return statusCmd(ctx, nil)
			},
		}, "user")

		require.NoError(t, store.Set(ctx, "42", payload{N: 1}, time.Minute))
		assert.Equal(t, "user:42", gotKey)
		assert.Equal(t, time.Minute, gotTTL)
		assert.JSONEq(t, `{"N":1}`, string(gotValue.([]byte)))
	})

	t.Run("encode error", func(t *testing.T) {
		called := false
		store := NewStore[badPayload](&fakeCmdable{
			set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
				called = true
				return statusCmd(ctx, nil)
			},
		}, "user")

		err := store.Set(ctx, "42", badPayload{F: math.Inf(1)}, time.Minute)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `redisx: encode "user:42"`)
		assert.False(t, called)
	})

	t.Run("backend error", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
				return statusCmd(ctx, errors.New("boom"))
			},
		}, "user")

		err := store.Set(ctx, "42", payload{}, time.Minute)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `redisx: set "user:42"`)
	})
}

func TestStoreDel(t *testing.T) {
	ctx := context.Background()

	t.Run("no keys", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{}, "user")
		require.NoError(t, store.Del(ctx))
	})

	t.Run("success", func(t *testing.T) {
		var gotKeys []string
		store := NewStore[payload](&fakeCmdable{
			del: func(_ context.Context, keys ...string) *redis.IntCmd {
				gotKeys = keys
				return intCmd(ctx, int64(len(keys)), nil)
			},
		}, "user")

		require.NoError(t, store.Del(ctx, "1", "2"))
		assert.Equal(t, []string{"user:1", "user:2"}, gotKeys)
	})

	t.Run("backend error", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			del: func(context.Context, ...string) *redis.IntCmd {
				return intCmd(ctx, 0, errors.New("boom"))
			},
		}, "user")

		err := store.Del(ctx, "1", "2")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redisx: del user:1,user:2")
	})
}

func TestStoreTake(t *testing.T) {
	ctx := context.Background()

	t.Run("cache hit", func(t *testing.T) {
		loaded := false
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, `{"N":1}`, nil)
			},
		}, "user")

		value, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			loaded = true
			return payload{}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, payload{N: 1}, value)
		assert.False(t, loaded)
	})

	t.Run("miss loads and caches", func(t *testing.T) {
		var gotKey string
		var gotTTL time.Duration
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "", redis.Nil)
			},
			set: func(_ context.Context, key string, _ any, ttl time.Duration) *redis.StatusCmd {
				gotKey, gotTTL = key, ttl
				return statusCmd(ctx, nil)
			},
		}, "user")

		value, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			return payload{N: 2}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, payload{N: 2}, value)
		assert.Equal(t, "user:42", gotKey)
		assert.Equal(t, time.Minute, gotTTL)
	})

	t.Run("initial get error", func(t *testing.T) {
		loaded := false
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "", errors.New("boom"))
			},
		}, "user")

		_, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			loaded = true
			return payload{}, nil
		})
		require.Error(t, err)
		assert.False(t, loaded)
	})

	t.Run("recheck error", func(t *testing.T) {
		calls := 0
		loaded := false
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				calls++
				if calls == 1 {
					return stringCmd(ctx, "", redis.Nil)
				}
				return stringCmd(ctx, "", errors.New("boom"))
			},
		}, "user")

		_, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			loaded = true
			return payload{}, nil
		})
		require.Error(t, err)
		assert.False(t, loaded)
	})

	t.Run("recheck hit", func(t *testing.T) {
		calls := 0
		loaded := false
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				calls++
				if calls == 1 {
					return stringCmd(ctx, "", redis.Nil)
				}
				return stringCmd(ctx, `{"N":3}`, nil)
			},
		}, "user")

		value, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			loaded = true
			return payload{}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, payload{N: 3}, value)
		assert.False(t, loaded)
	})

	t.Run("load error", func(t *testing.T) {
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "", redis.Nil)
			},
		}, "user")

		_, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			return payload{}, errors.New("load failed")
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "load failed")
	})

	t.Run("cache write error is ignored", func(t *testing.T) {
		setCalled := false
		store := NewStore[payload](&fakeCmdable{
			get: func(context.Context, string) *redis.StringCmd {
				return stringCmd(ctx, "", redis.Nil)
			},
			set: func(context.Context, string, any, time.Duration) *redis.StatusCmd {
				setCalled = true
				return statusCmd(ctx, errors.New("boom"))
			},
		}, "user")

		value, err := store.Take(ctx, "42", time.Minute, func(context.Context) (payload, error) {
			return payload{N: 4}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, payload{N: 4}, value)
		assert.True(t, setCalled)
	})
}

func TestStoreWithMiniredis(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), DisableIdentity: true})
	t.Cleanup(func() { _ = client.Close() })

	ctx := context.Background()
	store := NewStore[payload](client, "user")

	require.NoError(t, store.Set(ctx, "42", payload{N: 7}, time.Minute))
	assert.Equal(t, time.Minute, mr.TTL(store.Key("42")))

	value, ok, err := store.Get(ctx, "42")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, payload{N: 7}, value)

	raw, err := client.Get(ctx, store.Key("42")).Result()
	require.NoError(t, err)
	assert.JSONEq(t, `{"N":7}`, raw)

	require.NoError(t, store.Del(ctx, "42"))
	_, ok, err = store.Get(ctx, "42")
	require.NoError(t, err)
	assert.False(t, ok)
}
