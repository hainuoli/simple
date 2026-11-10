package httpx

import (
	"encoding/json"
	"net/http"
)

type Body struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

func Ok(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, Body{Code: 0, Msg: "ok", Data: data})
}

func Err(w http.ResponseWriter, status, code int, msg string) {
	JSON(w, status, Body{Code: code, Msg: msg})
}

func JSON(w http.ResponseWriter, status int, body Body) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
