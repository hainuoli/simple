package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"simple/pkg/errorx"
)

// Body is the JSON envelope shared by every response. Code carries the
// business result, Msg is human-readable text, and Data is omitted when nil.
type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// Ok writes data with HTTP 200, business code 0, and message "ok".
func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Body{Code: errorx.CodeOK, Msg: "ok", Data: data})
}

// Err writes err as the JSON error response. *errorx.Error values keep their
// status, code, and message. Any other error is logged and reported as a
// generic 500, so internal details never reach the client.
func Err(w http.ResponseWriter, err error) {
	if err == nil {
		JSON(w, http.StatusInternalServerError, Body{Code: errorx.CodeInternal, Msg: "internal server error"})
		return
	}

	var e *errorx.Error
	if errors.As(err, &e) && e != nil {
		if e.Status >= http.StatusInternalServerError && e.Err != nil {
			slog.Error("request failed", "err", e.Err)
		}
		JSON(w, e.Status, Body{Code: e.Code, Msg: e.Msg})
		return
	}

	slog.Error("request failed", "err", err)
	JSON(w, http.StatusInternalServerError, Body{Code: errorx.CodeInternal, Msg: "internal server error"})
}

// JSON writes body as JSON with the given status. Encoding errors are not
// reported because the status line has already been sent.
func JSON(w http.ResponseWriter, status int, body Body) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
