package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"simple/pkg/errorx"
	"simple/pkg/httpx"
)

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"err", err,
					"stack", string(debug.Stack()),
				)
				httpx.Err(w, errorx.NewError(http.StatusInternalServerError, errorx.CodeInternal, "internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
