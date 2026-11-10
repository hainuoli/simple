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

	// users: reference CRUD flow
	v1.HandleFunc("POST /users", handler.CreateUser(svc))
	v1.HandleFunc("GET /users", handler.ListUsers(svc))
	v1.HandleFunc("GET /users/{id}", handler.GetUser(svc))
	v1.HandleFunc("PUT /users/{id}", handler.UpdateUser(svc))
	v1.HandleFunc("DELETE /users/{id}", handler.DeleteUser(svc))

	// other wirte here:
	// ...
	v1.HandleFunc("GET /panic", func(w http.ResponseWriter, r *http.Request) {
		panic("hi~")
	})

	mux.Handle(v1Prefix+"/", http.StripPrefix(v1Prefix, v1))
}
