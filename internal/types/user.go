// Package types defines the request and response shapes of the HTTP API.
//
// Field tags are read by httpx.Parse: json for request bodies, query for URL
// query parameters, and path for route parameters. Keep these structs free of
// behavior so they can be generated later without hand edits.
package types

import "time"

type (
	User struct {
		ID        int64     `json:"id"`
		Name      string    `json:"name"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
	}

	CreateUserReq struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	GetUserReq struct {
		ID int64 `path:"id"`
	}

	UpdateUserReq struct {
		ID    int64  `path:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	DeleteUserReq struct {
		ID int64 `path:"id"`
	}

	ListUsersReq struct {
		Page int `query:"page"`
		Size int `query:"size"`
	}

	ListUsersResp struct {
		Items []User `json:"items"`
		Total int    `json:"total"`
	}
)
