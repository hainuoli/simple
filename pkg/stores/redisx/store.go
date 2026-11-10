package redisx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

// Store is a typed JSON view over one Redis key namespace.
//
// NewStore binds a prefix such as "user" to a Go type, so callers never build
// keys or encode values by hand:
//
//	users := redisx.NewStore[User](rdb, "user")
//
//	u, ok, err := users.Get(ctx, "42")        // key: user:42
//	err = users.Set(ctx, "42", u, time.Hour)  // JSON encoded
//
// Values are encoded with encoding/json. A Store is safe for concurrent use.
// Take additionally collapses concurrent cache misses for the same key.
type Store[T any] struct {
	cmd    redis.Cmdable
	prefix string
	group  singleflight.Group
}

// NewStore returns a typed store scoped to prefix.
//
// prefix is trimmed of surrounding spaces and colons, so "user", "user:" and
// " user: " are equivalent. An empty prefix means keys are used unchanged.
// cmd is typically a *Client, but any redis.Cmdable works, including a
// transaction. cmd must be non-nil.
func NewStore[T any](cmd redis.Cmdable, prefix string) *Store[T] {
	return &Store[T]{
		cmd:    cmd,
		prefix: strings.Trim(strings.TrimSpace(prefix), ":"),
	}
}

// Key returns the full Redis key for key.
//
// Use it when a raw command needs the same namespace as the typed helpers:
//
//	rdb.Expire(ctx, users.Key("42"), time.Hour)
func (s *Store[T]) Key(key string) string {
	if s.prefix == "" {
		return key
	}
	if key == "" {
		return s.prefix
	}
	return s.prefix + ":" + key
}

// Get returns the decoded value and whether the key exists.
//
// A missing key is (zero, false, nil), so callers do not need to compare
// against redis.Nil. Decode and connection errors are returned as errors.
func (s *Store[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T

	fullKey := s.Key(key)
	data, err := s.cmd.Get(ctx, fullKey).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, false, nil
	}
	if err != nil {
		return zero, false, fmt.Errorf("redisx: get %q: %w", fullKey, err)
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return zero, false, fmt.Errorf("redisx: decode %q: %w", fullKey, err)
	}
	return value, true, nil
}

// Set JSON-encodes value and stores it under the full key.
//
// ttl == 0 stores the value without expiration; use Del to remove it. Negative
// TTLs are rejected by Redis and returned as an error.
func (s *Store[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	fullKey := s.Key(key)

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redisx: encode %q: %w", fullKey, err)
	}

	if err := s.cmd.Set(ctx, fullKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("redisx: set %q: %w", fullKey, err)
	}
	return nil
}

// Del removes the given keys from the namespace. No keys is a no-op.
func (s *Store[T]) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	fullKeys := make([]string, len(keys))
	for i, key := range keys {
		fullKeys[i] = s.Key(key)
	}

	if err := s.cmd.Del(ctx, fullKeys...).Err(); err != nil {
		return fmt.Errorf("redisx: del %s: %w", strings.Join(fullKeys, ","), err)
	}
	return nil
}

// Take returns the cached value, or calls load on a miss and caches the result.
//
// Concurrent misses for the same key are collapsed: load runs once and every
// caller receives its result. A cache write failure is ignored because the
// cache is an optimization; the loaded value is still returned. load runs with
// the context of the caller that owns the flight.
//
// Use Take for read-through caches; use Get/Set directly when the caller needs
// to control when a value is written.
func (s *Store[T]) Take(
	ctx context.Context,
	key string,
	ttl time.Duration,
	load func(context.Context) (T, error),
) (T, error) {
	var zero T

	if value, ok, err := s.Get(ctx, key); err != nil {
		return zero, err
	} else if ok {
		return value, nil
	}

	value, err, _ := s.group.Do(s.Key(key), func() (any, error) {
		// Another goroutine may have filled the cache while this one waited.
		if value, ok, err := s.Get(ctx, key); err != nil {
			return zero, err
		} else if ok {
			return value, nil
		}

		value, err := load(ctx)
		if err != nil {
			return zero, err
		}

		_ = s.Set(ctx, key, value, ttl)
		return value, nil
	})
	if err != nil {
		return zero, err
	}
	return value.(T), nil
}
