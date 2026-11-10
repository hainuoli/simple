package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		content string
		env     string
		addr    string
		want    Conf
		wantErr string
	}{
		{
			name: "defaults",
			want: Conf{Env: "dev", Addr: ":8888"},
		},
		{
			name:    "dotenv",
			content: "APP_ENV=test\nHTTP_ADDR=:9000\n",
			want:    Conf{Env: "test", Addr: ":9000"},
		},
		{
			name:    "existing environment takes precedence",
			content: "APP_ENV=test\nHTTP_ADDR=:9000\n",
			env:     "prod",
			addr:    ":7000",
			want:    Conf{Env: "prod", Addr: ":7000"},
		},
		{
			name:    "invalid environment",
			env:     "staging",
			wantErr: `unknown APP_ENV: "staging"`,
		},
		{
			name:    "dotenv read failure",
			content: strings.Repeat("x", 70<<10),
			wantErr: "scan .env:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changeDir(t, t.TempDir())
			unsetEnv(t, "APP_ENV")
			unsetEnv(t, "HTTP_ADDR")

			if tt.content != "" {
				require.NoError(t, os.WriteFile(".env", []byte(tt.content), 0600))
			}
			if tt.env != "" {
				t.Setenv("APP_ENV", tt.env)
			}
			if tt.addr != "" {
				t.Setenv("HTTP_ADDR", tt.addr)
			}

			got, err := Load()
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Equal(t, Conf{}, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnv(t *testing.T) {
	const key = "CONFIG_TEST_FALLBACK"

	unsetEnv(t, key)
	require.Equal(t, "fallback", getEnv(key, "fallback"))

	t.Setenv(key, "")
	require.Equal(t, "fallback", getEnv(key, "fallback"))

	t.Setenv(key, "value")
	require.Equal(t, "value", getEnv(key, "fallback"))
}

func TestLoadDotEnv(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".env")
		require.NoError(t, loadDotEnv(path))
	})

	t.Run("open failure", func(t *testing.T) {
		require.ErrorContains(t, loadDotEnv("\x00"), "open")
	})

	t.Run("scanner failure", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), ".env")
		require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("x", 70<<10)), 0600))
		require.ErrorContains(t, loadDotEnv(path), "scan")
	})

	t.Run("parsing and precedence", func(t *testing.T) {
		const (
			newKey      = "CONFIG_TEST_NEW_KEY"
			existingKey = "CONFIG_TEST_EXISTING_KEY"
			quotedKey   = "CONFIG_TEST_QUOTED_KEY"
			equalsKey   = "CONFIG_TEST_EQUALS_KEY"
		)

		unsetEnv(t, newKey)
		unsetEnv(t, quotedKey)
		unsetEnv(t, equalsKey)
		t.Setenv(existingKey, "from-process")

		path := filepath.Join(t.TempDir(), ".env")
		content := strings.Join([]string{
			"",
			"   # comment",
			"not-an-assignment",
			" " + newKey + " =  hello  ",
			existingKey + "=from-file",
			quotedKey + `="hello world"`,
			equalsKey + "=a=b=c",
		}, "\n")

		require.NoError(t, os.WriteFile(path, []byte(content), 0600))
		require.NoError(t, loadDotEnv(path))

		require.Equal(t, "hello", os.Getenv(newKey))
		require.Equal(t, "from-process", os.Getenv(existingKey))
		require.Equal(t, "hello world", os.Getenv(quotedKey))
		require.Equal(t, "a=b=c", os.Getenv(equalsKey))
	})
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()

	previous, exists := os.LookupEnv(key)
	require.NoError(t, os.Unsetenv(key))

	t.Cleanup(func() {
		if exists {
			require.NoError(t, os.Setenv(key, previous))
		} else {
			require.NoError(t, os.Unsetenv(key))
		}
	})
}

func changeDir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))

	t.Cleanup(func() {
		require.NoError(t, os.Chdir(previous))
	})
}
