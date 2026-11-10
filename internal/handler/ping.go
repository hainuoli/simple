package handler

import (
	"net/http"

	"simple/internal/logic"
	"simple/internal/svc"
	"simple/pkg/httpx"
)

func Ping(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewPingLogic(r.Context(), svc)
		httpx.Ok(w, l.Ping())
	}
}
