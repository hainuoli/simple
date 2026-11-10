package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestErr(t *testing.T) {
	r := httptest.NewRecorder()

	Err(r, http.StatusUnprocessableEntity, 1001, "invalid input")

	requireResponse(t, r, http.StatusUnprocessableEntity, Body{
		Code: 1001,
		Msg:  "invalid input",
	})
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
