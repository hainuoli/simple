package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"simple/pkg/errorx"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOk(t *testing.T) {
	r := httptest.NewRecorder()

	Ok(r, map[string]string{"name": "Ada"})

	requireResponse(t, r, http.StatusOK, Body{
		Code: 0,
		Msg:  "ok",
		Data: map[string]any{"name": "Ada"},
	})
}

func TestOkNilData(t *testing.T) {
	r := httptest.NewRecorder()

	Ok(r, nil)

	requireResponse(t, r, http.StatusOK, Body{Code: 0, Msg: "ok"})
	require.JSONEq(t, `{"code":0,"msg":"ok"}`, r.Body.String())
}

func TestErrBusinessError(t *testing.T) {
	recorder := httptest.NewRecorder()

	// Err must see through wrapping, which is how logic usually propagates.
	Err(recorder, fmt.Errorf("create user: %w", errorx.NotFound("user not found")))

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Equal(t, "application/json; charset=utf-8", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, `{"code":40400,"msg":"user not found"}`, recorder.Body.String())
}

func TestErrCustomStatus(t *testing.T) {
	recorder := httptest.NewRecorder()

	Err(recorder, errorx.NewError(http.StatusUnprocessableEntity, 1001, "invalid input"))

	require.Equal(t, http.StatusUnprocessableEntity, recorder.Code)
	require.JSONEq(t, `{"code":1001,"msg":"invalid input"}`, recorder.Body.String())
}

func TestErrUnknownErrorDoesNotLeak(t *testing.T) {
	recorder := httptest.NewRecorder()

	Err(recorder, errors.New("dial tcp 127.0.0.1:5432: connection refused"))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"code":50000,"msg":"internal server error"}`, recorder.Body.String())
	require.NotContains(t, recorder.Body.String(), "5432")
}

func TestErrInternalError(t *testing.T) {
	recorder := httptest.NewRecorder()

	Err(recorder, errorx.Internal(errors.New("boom")))

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"code":50000,"msg":"internal server error"}`, recorder.Body.String())
}

func TestErrNil(t *testing.T) {
	recorder := httptest.NewRecorder()

	Err(recorder, nil)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"code":50000,"msg":"internal server error"}`, recorder.Body.String())
}

func TestJSON(t *testing.T) {
	r := httptest.NewRecorder()

	JSON(r, http.StatusCreated, Body{
		Code: 7,
		Msg:  "created",
		Data: []string{"a", "b"},
	})

	requireResponse(t, r, http.StatusCreated, Body{
		Code: 7,
		Msg:  "created",
		Data: []any{"a", "b"},
	})
}

func TestJSONUnsupportedData(t *testing.T) {
	r := httptest.NewRecorder()

	require.NotPanics(t, func() {
		JSON(r, http.StatusAccepted, Body{Data: make(chan int)})
	})
	require.Equal(t, http.StatusAccepted, r.Code)
	require.Equal(t, "application/json; charset=utf-8", r.Header().Get("Content-Type"))
}

func requireResponse(t *testing.T, r *httptest.ResponseRecorder, status int, want Body) {
	t.Helper()

	require.Equal(t, status, r.Code)
	require.Equal(t, "application/json; charset=utf-8", r.Header().Get("Content-Type"))

	var got Body
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &got))
	require.Equal(t, want, got)
}
