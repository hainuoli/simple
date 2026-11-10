package handler

import (
	"net/http"

	"simple/internal/logic"
	"simple/internal/svc"
	"simple/internal/types"
	"simple/pkg/errorx"
	"simple/pkg/httpx"
)

// The handlers below are the reference shape for every endpoint: parse the
// request, call logic, then return with Ok or Err. Business decisions
// live in logic, not here.

func CreateUser(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Err(w, errorx.InvalidParam(err.Error()))
			return
		}

		l := logic.NewUserLogic(r.Context(), svc)
		user, err := l.Create(&req)
		if err != nil {
			httpx.Err(w, err)
			return
		}
		httpx.Ok(w, user)
	}
}

func GetUser(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Err(w, errorx.InvalidParam(err.Error()))
			return
		}

		l := logic.NewUserLogic(r.Context(), svc)
		user, err := l.Get(&req)
		if err != nil {
			httpx.Err(w, err)
			return
		}
		httpx.Ok(w, user)
	}
}

func ListUsers(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListUsersReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Err(w, errorx.InvalidParam(err.Error()))
			return
		}

		l := logic.NewUserLogic(r.Context(), svc)
		resp, err := l.List(&req)
		if err != nil {
			httpx.Err(w, err)
			return
		}
		httpx.Ok(w, resp)
	}
}

func UpdateUser(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Err(w, errorx.InvalidParam(err.Error()))
			return
		}

		l := logic.NewUserLogic(r.Context(), svc)
		user, err := l.Update(&req)
		if err != nil {
			httpx.Err(w, err)
			return
		}
		httpx.Ok(w, user)
	}
}

func DeleteUser(svc *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeleteUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Err(w, errorx.InvalidParam(err.Error()))
			return
		}

		l := logic.NewUserLogic(r.Context(), svc)
		if err := l.Delete(&req); err != nil {
			httpx.Err(w, err)
			return
		}
		httpx.Ok(w, nil)
	}
}
