package svc

import (
	"testing"

	"github.com/stretchr/testify/require"

	"simple/internal/config"
)

func TestNew(t *testing.T) {
	conf := config.Conf{Env: "test", Addr: ":9999"}

	s := New(conf)

	require.Equal(t, conf, s.Conf)
	require.NotNil(t, s.Users)
}
