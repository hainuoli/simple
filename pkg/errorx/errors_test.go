package errorx

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBusinessErrorHelpers(t *testing.T) {
	tests := []struct {
		name   string
		err    *Error
		status int
		code   int
		msg    string
	}{
		{name: "invalid param", err: InvalidParam("bad input"), status: http.StatusBadRequest, code: CodeInvalidParam, msg: "bad input"},
		{name: "not found", err: NotFound("missing"), status: http.StatusNotFound, code: CodeNotFound, msg: "missing"},
		{name: "conflict", err: Conflict("duplicate"), status: http.StatusConflict, code: CodeConflict, msg: "duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.status, tt.err.Status)
			require.Equal(t, tt.code, tt.err.Code)
			require.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("cause")

	require.ErrorIs(t, Internal(cause), cause)
	require.ErrorIs(t, fmt.Errorf("wrap: %w", Internal(cause)), cause)
}
