package router

import (
	"net/http"

	"simple/internal/handler"
	"simple/internal/svc"
)

const v1Prefix = "/api/v1"

func RegisterV1(mux *http.ServeMux, svc *svc.ServiceContext) {
	v1 := http.NewServeMux()

	// example:
	v1.HandleFunc("GET /ping", handler.Ping(svc))
	// other wirte here:
	// ...
	v1.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("hi~")
	})

	mux.Handle(v1Prefix+"/", http.StripPrefix(v1Prefix, v1))
}
