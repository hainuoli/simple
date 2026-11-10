# sqlx：薄封装数据访问层

`pkg/stores/sqlx` 是 `database/sql` 之上的薄封装，目标是消除连接配置、行扫描和事务生命周期中的重复代码，同时保留标准库的全部能力。

## 1. 设计原则

- 标准库优先：`SqlConn` 直接嵌入 `*sql.DB`，不重新实现 SQL 方法。
- 零运行时反射：结构体映射由 `ScanFrom` 显式完成，后续可由代码生成器维护。
- 按需组合：`Get`、`Select`、`Transact` 可以单独使用，不要求全部采用。
- 不重复封装：`ExecContext`、`QueryContext`、`QueryRowContext` 继续使用标准库。
- 零第三方核心依赖：数据库驱动由应用按需引入。

## 2. 核心组件

| 组件 | 作用 |
| --- | --- |
| `SqlConf` | 驱动名、数据源、连接池配置 |
| `PoolConf` | 最大连接数、最大空闲连接数、连接生命周期 |
| `SqlConn` | 持有 `*sql.DB` 的连接池句柄 |
| `DBTX` | 统一 `*sql.DB`、`*sql.Tx`、`*SqlConn` |
| `RowScanner` | 统一 `*sql.Row`、`*sql.Rows` 的 `Scan` |
| `Get[T]` | 查询单条并调用 `ScanFrom` |
| `Select[T]` | 查询多条并处理 Rows 生命周期 |
| `Transact` | 自动提交与回滚 |
| `TransactOptions` | 带事务选项的 `Transact` |

## 3. 初始化连接

```go
import (
	"context"
	"time"

	"simple/pkg/stores/sqlx"
)

func newDB(ctx context.Context, dsn string) (*sqlx.SqlConn, error) {
	return sqlx.NewConn(ctx, sqlx.SqlConf{
		DriverName: "mysql",
		DataSource: dsn,
		Pool: &sqlx.PoolConf{
			MaxOpenConns:    20,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 10 * time.Minute,
		},
	})
}
```

`Pool` 为 `nil` 时完全使用 `database/sql` 的默认设置。

`NewConn` 会执行一次 `PingContext`，让连接错误在启动阶段暴露。返回的 `SqlConn` 由调用方负责 `Close`。

数据库驱动需要应用自行注册：

```go
import _ "github.com/go-sql-driver/mysql"
```

PostgreSQL 可以使用 `github.com/jackc/pgx/v5/stdlib`，并把 `DriverName` 设为 `pgx`。

## 4. Model 写法

Model 属于 repo 的数据库实现包（例如 `internal/repo/sqldb`），不是 `svc` 的一部分，也不暴露给 logic。

每个 Model 只需要实现 `ScanFrom`，扫描顺序必须与 SQL 返回的列顺序一致。

```go
type User struct {
	ID   int64
	Name string
}

func (u *User) ScanFrom(s sqlx.RowScanner) error {
	return s.Scan(&u.ID, &u.Name)
}

type UserModel struct {
	db sqlx.DBTX
}

func NewUserModel(db sqlx.DBTX) *UserModel {
	return &UserModel{db: db}
}

func (m *UserModel) WithTx(tx *sql.Tx) *UserModel {
	return &UserModel{db: tx}
}

func (m *UserModel) FindByID(ctx context.Context, id int64) (User, error) {
	return sqlx.Get[User](ctx, m.db,
		"SELECT id, name FROM users WHERE id = ?", id)
}

func (m *UserModel) List(ctx context.Context) ([]User, error) {
	return sqlx.Select[User](ctx, m.db,
		"SELECT id, name FROM users")
}
```

`Get` 保留 `sql.ErrNoRows` 语义；`Select` 没有结果时返回非 nil 的空切片。

SQL 占位符由驱动决定：MySQL 使用 `?`，PostgreSQL 使用 `$1`、`$2`。

## 5. 事务

事务边界属于 repository，而不是 logic。把 `sqlx.Transact` 收在 repo 方法内部，logic 只看到普通的领域方法：

```go
func (r *UserRepo) UpdateName(ctx context.Context, id int64, name string) error {
	return sqlx.Transact(ctx, r.db, func(tx *sql.Tx) error {
		return r.WithTx(tx).updateName(ctx, id, name)
	})
}
```

需要指定隔离级别或只读事务时，同样只出现在 repo 实现内部：

```go
return sqlx.TransactOptions(ctx, r.db, &sql.TxOptions{
	Isolation: sql.LevelSerializable,
	ReadOnly:  true,
}, func(tx *sql.Tx) error {
	return doSomething(ctx, tx)
})
```

`Transact` 的行为：

- `fn` 返回 `nil` 时执行 `Commit`。
- `fn` 返回错误时执行 `Rollback`，并返回该错误。
- `fn` 发生 panic 时执行 `Rollback`，然后继续向上抛出 panic。
- `Commit` 的错误原样返回，不隐藏。
- 延迟 `Rollback` 产生的 `sql.ErrTxDone` 不会被当作业务错误返回。

如果一个业务动作必须跨多个 repo 方法原子提交，就在接口上增加事务方法，而不是把 `*sql.Tx` 暴露给 logic：

```go
type UserRepo interface {
	// ...
	Transaction(ctx context.Context, fn func(context.Context, UserRepo) error) error
}
```

这样换用 gorm 时只需要重新实现接口，logic 中的调用代码不变。

## 6. 分层与替换实现

`svc` 是依赖容器：字段声明为接口，`New` 内部创建实现。这样 logic 永远看不到具体存储或 ORM 类型，而「用哪个实现」集中在 `svc.New` 一处：

```go
type ServiceContext struct {
	Conf  config.Conf
	Users repo.UserRepo
}

func New(c config.Conf) *ServiceContext {
	return &ServiceContext{
		Conf:  c,
		Users: memory.NewStore(), // 换成 sqldb.NewUserRepo(conn) 即可
	}
}
```

启动层只负责调用：

```go
svc := svc.New(cfg)
```

换成 sqlx 时，只改 `svc.New`。连接池也在这里打开，并由 `ServiceContext` 持有和关闭：

```go
func New(c config.Conf, ctx context.Context) (*ServiceContext, error) {
	conn, err := sqlx.NewConn(ctx, sqlx.SqlConf{
		DriverName: "mysql",
		DataSource: c.DB.DataSource,
	})
	if err != nil {
		return nil, err
	}

	return &ServiceContext{
		Conf:  c,
		DB:    conn,
		Users: sqldb.NewUserRepo(conn),
	}, nil
}
```

换成 gorm 时，同样只改 `svc.New`：

```go
db, err := gorm.Open(mysql.Open(c.DB.DataSource), &gorm.Config{})
if err != nil {
	return nil, err
}

return &ServiceContext{Conf: c, Users: gormrepo.NewUserRepo(db)}, nil
```

`logic`、`handler`、`router` 都不需要修改，因为它们只依赖 `repo.UserRepo`。

连接池由创建它的 `ServiceContext` 持有，因此需要配一个 `Close` 方法，让启动层在退出时调用；数据库的关闭责任不落在 logic 上。

这种写法和 go-zero 的 `NewServiceContext` 一致：依赖在容器里组装，业务代码只拿到接口。本项目的差别是把接口和实现拆到 `internal/repo` 与 `internal/repo/memory`（将来可以是 `internal/repo/sqldb`、`internal/repo/gormrepo`），让领域包不引入任何基础设施依赖。

三条约束：

- 接口定义在使用方边界，即 `internal/repo`，保持领域语言，不出现 `*sql.DB`、`*sql.Tx`。
- 实现放在子包，构造函数返回接口或具体类型均可，但只允许组合根 import 实现包。
- `ServiceContext` 不保存数据库句柄；句柄归组合根所有，由它负责打开与关闭。

## 7. 非目标

`sqlx` v1 不提供以下能力：

- ORM 和运行时字段映射
- SQL 构建器
- 读写分离
- 自动重试
- 熔断
- 数据库缓存
- 内置 Metrics

这些能力可以在不破坏 `DBTX` 的前提下，作为独立层逐步增加。

## 8. 测试

`pkg/stores/sqlx` 使用自定义的 `database/sql/driver` 假实现，不依赖真实数据库，覆盖：

- 连接创建、Ping 失败、连接池校验
- 单行查询、多行查询、空结果、扫描错误、Rows 错误
- 事务提交、回滚、Begin 错误、Commit 错误、panic 回滚
- 事务内查询与 `*SqlConn` 兼容性

当前函数覆盖率为 100%，包含在 CI 的 `pkg/` 覆盖门禁中。
