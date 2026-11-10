// Package errorx defines business errors shared by logic and the transport
// layer. It deliberately does not import any transport package: httpx maps
// these values to JSON responses, never the other way around.
package errorx

import "net/http"

// Business codes carried in Body.Code. The HTTP status describes the
// transport-level result; Code describes the business result and stays stable
// even if the transport changes.
const (
	CodeOK           = 0
	CodeInvalidParam = 40000
	CodeNotFound     = 40400
	CodeConflict     = 40900
	CodeInternal     = 50000
)

// Error is a business error. Logic returns it and httpx.Err turns it into a
// response, so the error contract lives in one place instead of every handler.
type Error struct {
	Status int
	Code   int
	Msg    string
	Err    error
}

// Error returns the client-facing message, with the internal cause appended
// when one is present. The cause is for logs, not for responses.
func (e *Error) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

// Unwrap exposes the cause so errors.Is and errors.As keep working through
// business errors.
func (e *Error) Unwrap() error { return e.Err }

// NewError builds an error with an explicit status and business code. Prefer
// the helpers below unless a new category is needed.
func NewError(status, code int, msg string) *Error {
	return &Error{Status: status, Code: code, Msg: msg}
}

// InvalidParam reports a malformed or missing request value.
func InvalidParam(msg string) *Error {
	return NewError(http.StatusBadRequest, CodeInvalidParam, msg)
}

// NotFound reports a missing resource.
func NotFound(msg string) *Error {
	return NewError(http.StatusNotFound, CodeNotFound, msg)
}

// Conflict reports a state conflict, such as a duplicate unique value.
func Conflict(msg string) *Error {
	return NewError(http.StatusConflict, CodeConflict, msg)
}

// Internal wraps an unexpected error. The cause is logged but never sent to
// the client, so internal details do not leak.
func Internal(err error) *Error {
	return &Error{
		Status: http.StatusInternalServerError,
		Code:   CodeInternal,
		Msg:    "internal server error",
		Err:    err,
	}
}
