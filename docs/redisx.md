# redisx：Redis 连接与类型化缓存

`pkg/stores/redisx` 是 go-redis 的薄封装，提供连接初始化、启动检查和泛型 JSON 缓存。

## 初始化

```go
rdb, err := redisx.New(ctx, redisx.Conf{
	Addr:     "127.0.0.1:6379",
	Password: os.Getenv("REDIS_PASSWORD"),
})
if err != nil {
	return err
}
defer rdb.Close()
```

启动阶段可以直接使用 `MustNew`，失败时 panic：

```go
rdb := redisx.MustNew(ctx, redisx.Conf{Addr: "127.0.0.1:6379"})
```

常用配置：

| 字段 | 说明 |
| --- | --- |
| `Addr` | 单个 `host:port`；逗号分隔多个地址启用 cluster |
| `MasterName` | 设置后启用 sentinel |
| `DB` | 单机或 sentinel 的数据库编号 |
| `PoolSize` / `MinIdleConns` | 连接池大小与最小空闲连接 |
| `MaxRetries` | 默认 3；设为 `-1` 关闭重试 |
| `PingTimeout` | 启动检查超时，默认 5 秒 |

零值使用 go-redis 默认值。`New` 会 Ping 一次，`Client` 由调用方 `Close`。

## 原生命令

`Client` 嵌入 `redis.UniversalClient`，go-redis 的方法可以直接调用：

```go
rdb.Set(ctx, "greeting", "hello", time.Minute)

rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
	pipe.Incr(ctx, "a")
	pipe.Incr(ctx, "b")
	return nil
})

rdb.AddHook(hook)
```

`redis.Pipeliner` 等参数类型来自 go-redis，需要按需 import。

## 类型化缓存

`Store[T]` 绑定一个类型和一个 key 前缀，值使用 JSON 编码：

```go
type User struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

users := redisx.NewStore[User](rdb, "user")

u, ok, err := users.Get(ctx, "42") // key: user:42
if err != nil {
	return err
}
if !ok {
	// 未命中
}

err = users.Set(ctx, "42", u, 10*time.Minute)
err = users.Del(ctx, "42")
```

前缀首尾的空格和冒号会被去掉。原生命令需要使用完整 key 时调用 `Key`：

```go
rdb.Expire(ctx, users.Key("42"), time.Hour)
```

`Get` 未命中返回 `(zero, false, nil)`，不会返回 `redis.Nil`。

## Take

`Take` 封装「查缓存 → 未命中回源 → 写缓存」：

```go
u, err := users.Take(ctx, "42", 10*time.Minute, func(ctx context.Context) (User, error) {
	return userRepo.FindByID(ctx, 42)
})
```

- 命中缓存时不会调用 `load`。
- 同一个 key 的并发未命中会合并，`load` 只执行一次。
- 写缓存失败会被忽略，仍返回加载结果。

## 接入 ServiceContext

```go
type ServiceContext struct {
	Conf  config.Conf
	Redis *redisx.Client
	Users repo.UserRepo
}

func New(ctx context.Context, c config.Conf) (*ServiceContext, error) {
	rdb, err := redisx.New(ctx, redisx.Conf{
		Addr:     c.RedisAddr,
		Password: c.RedisPassword,
	})
	if err != nil {
		return nil, err
	}

	return &ServiceContext{Conf: c, Redis: rdb, Users: memory.NewStore()}, nil
}

func (s *ServiceContext) Close() error {
	return s.Redis.Close()
}
```

需要在 `config.Conf` 中增加 `RedisAddr`、`RedisPassword` 字段。
