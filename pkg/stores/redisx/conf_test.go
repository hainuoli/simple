package redisx

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfValidate(t *testing.T) {
	tests := []struct {
		name  string
		conf  Conf
		valid bool
		err   error
	}{
		{name: "single node", conf: Conf{Addr: "127.0.0.1:6379"}, valid: true},
		{name: "cluster", conf: Conf{Addr: "a:6379, b:6379"}, valid: true},
		{name: "sentinel", conf: Conf{Addr: "a:26379", MasterName: "mymaster"}, valid: true},
		{name: "empty addr", conf: Conf{}, err: ErrEmptyAddr},
		{name: "empty entry", conf: Conf{Addr: "a:6379,,b:6379"}, err: ErrEmptyAddr},
		{name: "whitespace entry", conf: Conf{Addr: "a:6379,   "}, err: ErrEmptyAddr},
		{name: "negative db", conf: Conf{Addr: "a:6379", DB: -1}},
		{name: "negative pool", conf: Conf{Addr: "a:6379", PoolSize: -1}},
		{name: "negative idle", conf: Conf{Addr: "a:6379", MinIdleConns: -1}},
		{name: "negative dial timeout", conf: Conf{Addr: "a:6379", DialTimeout: -time.Second}},
		{name: "negative read timeout", conf: Conf{Addr: "a:6379", ReadTimeout: -time.Second}},
		{name: "negative write timeout", conf: Conf{Addr: "a:6379", WriteTimeout: -time.Second}},
		{name: "negative ping timeout", conf: Conf{Addr: "a:6379", PingTimeout: -time.Second}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conf.validate()
			assert.Equal(t, tt.valid, tt.conf.Valid())

			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			if tt.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestConfAddrs(t *testing.T) {
	assert.Nil(t, Conf{}.addrs())
	assert.Equal(t, []string{"a:6379", "b:6379"}, Conf{Addr: " a:6379 , b:6379 "}.addrs())
}

func TestConfOptions(t *testing.T) {
	c := Conf{
		Addr:         " a:6379 , b:6379 ",
		MasterName:   "mymaster",
		Username:     "user",
		Password:     "pass",
		DB:           2,
		TLS:          true,
		DialTimeout:  time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     4,
		MinIdleConns: 1,
		MaxRetries:   -1,
	}

	o := c.options()
	assert.Equal(t, []string{"a:6379", "b:6379"}, o.Addrs)
	assert.Equal(t, "mymaster", o.MasterName)
	assert.Equal(t, "user", o.Username)
	assert.Equal(t, "pass", o.Password)
	assert.Equal(t, 2, o.DB)
	assert.Equal(t, time.Second, o.DialTimeout)
	assert.Equal(t, 2*time.Second, o.ReadTimeout)
	assert.Equal(t, 3*time.Second, o.WriteTimeout)
	assert.Equal(t, 4, o.PoolSize)
	assert.Equal(t, 1, o.MinIdleConns)
	assert.Equal(t, -1, o.MaxRetries)
	require.NotNil(t, o.TLSConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), o.TLSConfig.MinVersion)

	assert.Nil(t, Conf{Addr: "a:6379"}.options().TLSConfig)
}

func TestConfPingTimeout(t *testing.T) {
	assert.Equal(t, defaultPingTimeout, Conf{}.pingTimeout())
	assert.Equal(t, time.Second, Conf{PingTimeout: time.Second}.pingTimeout())
}
