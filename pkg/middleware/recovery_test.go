package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecover(t *testing.T) {
	t.Run("normal request", func(t *testing.T) {
		logs := captureLogs(t)
		recorder := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("done"))
		})

		Recover(handler).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

		require.Equal(t, http.StatusAccepted, recorder.Code)
		require.Equal(t, "done", recorder.Body.String())
		require.Empty(t, logs.String())
	})

	tests := []struct {
		name  string
		value any
		err   string
	}{
		{
			name:  "string panic",
			value: "boom",
			err:   "boom",
		},
		{
			name:  "error panic",
			value: errors.New("broken"),
			err:   "broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := captureLogs(t)
			recorder := httptest.NewRecorder()
			handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				panic(tt.value)
			})

			require.NotPanics(t, func() {
				Recover(handler).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			})

			require.Equal(t, http.StatusInternalServerError, recorder.Code)
			require.Equal(t, "internal server error\n", recorder.Body.String())
			require.Equal(t, "text/plain; charset=utf-8", recorder.Header().Get("Content-Type"))

			entry := decodeLog(t, logs)
			require.Equal(t, "ERROR", entry["level"])
			require.Equal(t, "panic recovered", entry["msg"])
			require.Contains(t, entry["err"], tt.err)
			require.Contains(t, entry["stack"], "TestRecover")
		})
	}
}
