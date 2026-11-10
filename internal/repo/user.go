// Package repo is the persistence boundary. It owns the storage model and the
// interfaces that logic depends on; concrete implementations live in
// subpackages such as repo/memory. Swapping the in-memory store for sqlx, gorm,
// or a remote service therefore changes only the wiring in svc.New, never
// handlers or logic.
package repo

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("not found")
	ErrEmailExists = errors.New("email already exists")
)

// User is the storage model. It is intentionally separate from types.User so
// persistence changes never leak into the HTTP contract.
type User struct {
	ID        int64
	Name      string
	Email     string
	CreatedAt time.Time
}

// UserRepo is the only data access surface that logic may use. Keep it in
// domain terms: no *sql.DB, *sql.Tx, or dialect-specific types, so any
// implementation can satisfy it.
//
// When an operation needs several writes to commit atomically, add a
// transaction method here, for example:
//
//	Transaction(ctx context.Context, fn func(context.Context, UserRepo) error) error
//
// Hiding the transaction inside the interface keeps logic free of sqlx or gorm
// APIs; leaking sqlx.Transact and *sql.Tx into logic would reintroduce the
// coupling this boundary exists to prevent.
type UserRepo interface {
	Create(ctx context.Context, name, email string) (*User, error)
	Get(ctx context.Context, id int64) (*User, error)
	List(ctx context.Context, offset, limit int) ([]User, int, error)
	Update(ctx context.Context, id int64, name, email string) (*User, error)
	Delete(ctx context.Context, id int64) error
}
