package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		status int
		handle http.HandlerFunc
		body   string
	}{
		{
			name:   "implicit OK",
			method: http.MethodGet,
			path:   "/users?active=true",
			status: http.StatusOK,
			handle: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("ok"))
			},
			body: "ok",
		},
		{
			name:   "explicit status",
			method: http.MethodPost,
			path:   "/users",
			status: http.StatusCreated,
			handle: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
		},
		{
			name:   "no response body",
			method: http.MethodDelete,
			path:   "/users/1",
			status: http.StatusOK,
			handle: func(http.ResponseWriter, *http.Request) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)

			Logger(tt.handle).ServeHTTP(recorder, request)

			require.Equal(t, tt.status, recorder.Code)
			require.Equal(t, tt.body, recorder.Body.String())

			entry := decodeLog(t, logs)
			require.Equal(t, "INFO", entry["level"])
			require.Equal(t, "http", entry["msg"])
			require.Equal(t, tt.method, entry["method"])
			require.Equal(t, request.URL.Path, entry["path"])
			require.Equal(t, float64(tt.status), entry["status"])
			require.Contains(t, entry, "cost")
		})
	}
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	return &buf
}

func decodeLog(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()

	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))

	return entry
}
