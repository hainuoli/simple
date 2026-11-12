<h1 align="center">Simple</h1>

<p align="center"><strong>Everything is simple.</strong></p>

### 快速开始

#### 本地开发

```bash
./scripts/shell/start-dev.sh            # 只启动 Go 应用
./scripts/shell/start-dev.sh --mysql    # 同时启动 MySQL
./scripts/shell/start-dev.sh --postgres # 同时启动 PostgreSQL
./scripts/shell/start-dev.sh --redis    # 同时启动 Redis
./scripts/shell/start-dev.sh --docker   # 用 Docker 启动应用和 Nginx
./scripts/shell/end-dev.sh              # 停止开发环境
```

#### 生产部署

```bash
./scripts/shell/start-prod.sh
```

部署完成后，日常运维使用: 
- `deploy/start.sh`
- `deploy/stop.sh`
- `deploy/restart.sh`
- `deploy/status.sh`
- `deploy/logs.sh`

### 文档

- [开发与部署指南](docs/deployment.md)
- [chain 中间件链](docs/chain.md)
- [errorx 业务错误](docs/errorx.md)
- [httpx 请求与响应](docs/httpx.md)
- [stores/sqlx 数据访问层](docs/sqlx.md)
- [stores/redisx Redis 与类型化缓存](docs/redisx.md)

### 说明

当前业务数据使用内存仓库；仓库通过 `repo.UserRepo` 接口注入，替换为 sqlx 或 gorm
实现时只需修改 `svc.New` 的组装代码，业务代码不需要改动。`pkg/stores/sqlx` 已提供数据库访问基础能力，
`pkg/stores/redisx` 已提供 Redis 连接与类型化缓存能力，但应用代码尚未接入数据库和缓存。
