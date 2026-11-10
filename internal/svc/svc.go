package svc

import (
	"simple/internal/config"
	"simple/internal/repo"
	"simple/internal/repo/memory"
)

// ServiceContext is the dependency container. Fields are interfaces, so logic
// never sees a concrete store or ORM type; only New below knows how they are
// built.
type ServiceContext struct {
	Conf  config.Conf
	Users repo.UserRepo
}

// New builds the dependencies. This is the only place that imports an
// implementation package, so switching memory for sqlx or gorm is a change here
// and nowhere in logic, handler, or router.
//
// When a real database is added, open the pool here and keep it on the context:
//
//	conn, err := sqlx.NewConn(ctx, sqlx.SqlConf{...})
//	...
//	return &ServiceContext{Conf: c, DB: conn, Users: sqldb.NewUserRepo(conn)}
//
// The context then needs a Close method, and the caller that built it owns the
// shutdown, just like http.Server.
func New(c config.Conf) *ServiceContext {
	return &ServiceContext{
		Conf:  c,
		Users: memory.NewStore(),
	}
}
