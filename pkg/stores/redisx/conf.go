// Package redisx is a thin layer over github.com/redis/go-redis/v9.
//
// It provides connection configuration, a startup health check, and a typed
// JSON cache that removes key composition and cache-aside boilerplate. It does
// not re-declare Redis commands: Client embeds redis.UniversalClient, so the
// full go-redis API (including future commands and hooks) stays available.
package redisx

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrEmptyAddr is returned when Conf.Addr has no usable host:port entry.
var ErrEmptyAddr = errors.New("redisx: empty addr")

const defaultPingTimeout = 5 * time.Second

// Conf defines how a Redis connection is created.
//
// Addr is a single host:port. A comma-separated list enables cluster mode, or
// sentinel mode when MasterName is set. Zero values keep go-redis defaults:
// timeouts, pool size, and retry policy are only overridden when non-zero.
type Conf struct {
	// Addr is "host:port" or a comma-separated seed list.
	Addr string
	// MasterName enables sentinel failover when non-empty.
	MasterName string
	// Username and Password are passed to Redis ACL authentication.
	Username string
	Password string
	// DB selects the logical database. Only single-node and sentinel clients
	// use it; cluster mode always uses DB 0.
	DB int
	// TLS enables TLS with a minimum version of TLS 1.2.
	TLS bool

	// DialTimeout, ReadTimeout, and WriteTimeout default to go-redis values.
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration

	// PoolSize defaults to 10 connections per CPU. MinIdleConns defaults to 0.
	PoolSize     int
	MinIdleConns int

	// MaxRetries defaults to 3. Set it to -1 to disable retries.
	MaxRetries int

	// PingTimeout bounds the startup health check. Zero means 5 seconds.
	PingTimeout time.Duration
}

// Valid reports whether the configuration can be passed to New.
//
// Validation is intentionally about obvious configuration mistakes, not about
// reaching the server; New performs that check with a real Ping.
func (c Conf) Valid() bool {
	return c.validate() == nil
}

func (c Conf) validate() error {
	addrs := c.addrs()
	if len(addrs) == 0 {
		return ErrEmptyAddr
	}
	for _, addr := range addrs {
		if addr == "" {
			return ErrEmptyAddr
		}
	}

	if c.DB < 0 {
		return fmt.Errorf("redisx: negative db %d", c.DB)
	}
	if c.PoolSize < 0 {
		return fmt.Errorf("redisx: negative pool size %d", c.PoolSize)
	}
	if c.MinIdleConns < 0 {
		return fmt.Errorf("redisx: negative min idle conns %d", c.MinIdleConns)
	}
	if c.DialTimeout < 0 || c.ReadTimeout < 0 || c.WriteTimeout < 0 {
		return errors.New("redisx: negative timeout")
	}
	if c.PingTimeout < 0 {
		return errors.New("redisx: negative ping timeout")
	}
	return nil
}

// addrs splits and trims Addr. Empty entries are preserved so validate can
// reject typos such as "a:6379,,b:6379" instead of silently skipping them.
func (c Conf) addrs() []string {
	if c.Addr == "" {
		return nil
	}

	parts := strings.Split(c.Addr, ",")
	addrs := make([]string, len(parts))
	for i, part := range parts {
		addrs[i] = strings.TrimSpace(part)
	}
	return addrs
}

func (c Conf) options() *redis.UniversalOptions {
	o := &redis.UniversalOptions{
		Addrs:        c.addrs(),
		MasterName:   c.MasterName,
		Username:     c.Username,
		Password:     c.Password,
		DB:           c.DB,
		MaxRetries:   c.MaxRetries,
		DialTimeout:  c.DialTimeout,
		ReadTimeout:  c.ReadTimeout,
		WriteTimeout: c.WriteTimeout,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
	}
	if c.TLS {
		o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return o
}

func (c Conf) pingTimeout() time.Duration {
	if c.PingTimeout > 0 {
		return c.PingTimeout
	}
	return defaultPingTimeout
}
