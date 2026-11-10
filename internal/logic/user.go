package logic

import (
	"context"
	"errors"
	"strings"

	"simple/internal/repo"
	"simple/internal/svc"
	"simple/internal/types"
	"simple/pkg/errorx"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxNameLength   = 64
)

// UserLogic is the reference implementation of a CRUD flow. It validates
// input, maps repo errors to business errors, and keeps HTTP details out.
type UserLogic struct {
	ctx context.Context
	svc *svc.ServiceContext
}

func NewUserLogic(ctx context.Context, svc *svc.ServiceContext) *UserLogic {
	return &UserLogic{
		ctx: ctx,
		svc: svc,
	}
}

func (l *UserLogic) Create(req *types.CreateUserReq) (*types.User, error) {
	name, email, err := validateUser(req.Name, req.Email)
	if err != nil {
		return nil, err
	}

	u, err := l.svc.Users.Create(l.ctx, name, email)
	if err != nil {
		return nil, mapUserError(err)
	}
	return toUser(u), nil
}

func (l *UserLogic) Get(req *types.GetUserReq) (*types.User, error) {
	u, err := l.svc.Users.Get(l.ctx, req.ID)
	if err != nil {
		return nil, mapUserError(err)
	}
	return toUser(u), nil
}

func (l *UserLogic) List(req *types.ListUsersReq) (*types.ListUsersResp, error) {
	page, size := req.Page, req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}

	users, total, err := l.svc.Users.List(l.ctx, (page-1)*size, size)
	if err != nil {
		return nil, mapUserError(err)
	}

	items := make([]types.User, 0, len(users))
	for i := range users {
		items = append(items, *toUser(&users[i]))
	}
	return &types.ListUsersResp{Items: items, Total: total}, nil
}

func (l *UserLogic) Update(req *types.UpdateUserReq) (*types.User, error) {
	name, email, err := validateUser(req.Name, req.Email)
	if err != nil {
		return nil, err
	}

	u, err := l.svc.Users.Update(l.ctx, req.ID, name, email)
	if err != nil {
		return nil, mapUserError(err)
	}
	return toUser(u), nil
}

func (l *UserLogic) Delete(req *types.DeleteUserReq) error {
	if err := l.svc.Users.Delete(l.ctx, req.ID); err != nil {
		return mapUserError(err)
	}
	return nil
}

func validateUser(name, email string) (string, string, error) {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)

	if name == "" {
		return "", "", errorx.InvalidParam("name is required")
	}
	if len(name) > maxNameLength {
		return "", "", errorx.InvalidParam("name is too long")
	}
	if !strings.Contains(email, "@") {
		return "", "", errorx.InvalidParam("email is invalid")
	}
	return name, email, nil
}

func mapUserError(err error) error {
	switch {
	case errors.Is(err, repo.ErrNotFound):
		return errorx.NotFound("user not found")
	case errors.Is(err, repo.ErrEmailExists):
		return errorx.Conflict("email already exists")
	default:
		return errorx.Internal(err)
	}
}

func toUser(u *repo.User) *types.User {
	return &types.User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
	}
}
