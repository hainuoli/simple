# 开发与部署指南

本文档对应 `scripts/shell/` 下的四个脚本，说明本地开发、生产部署、日常运维和排查方式。

## 1. 环境要求

| 场景 | 需要安装 |
| --- | --- |
| 本地开发（直接运行 Go） | Go 1.23+ |
| 本地开发（`--docker`） | Docker + Docker Compose v2 |
| 生产部署 | Docker + Docker Compose v2 |

脚本使用 Docker Compose v2（`docker compose` 命令），不兼容旧的 `docker-compose` v1。

---

## 2. 本地开发

### 2.1 快速开始

```bash
./scripts/shell/start-dev.sh               # 只运行 Go 应用
./scripts/shell/start-dev.sh --mysql       # 先启动 MySQL，再运行应用
./scripts/shell/start-dev.sh --postgres    # 先启动 PostgreSQL，再运行应用
./scripts/shell/start-dev.sh --redis       # 先启动 Redis，再运行应用
./scripts/shell/start-dev.sh --docker      # 用 Docker 启动应用 + Nginx
./scripts/shell/start-dev.sh --port 9000   # 覆盖本地监听端口
```

第一次运行时会自动根据 `.env.example` 创建 `.env`。

### 2.2 参数说明

| 参数 | 作用 |
| --- | --- |
| `--mysql` | 启动 MySQL 容器，等待健康检查通过后再启动应用 |
| `--postgres` | 启动 PostgreSQL 容器 |
| `--redis` | 启动 Redis 容器 |
| `--docker` | 构建并启动完整开发栈（应用 + Nginx），可与其他数据库参数组合 |
| `--port <端口>` | 覆盖本地应用监听端口，同时覆盖 Docker 模式下的宿主机映射端口 |
| `-h` / `--help` | 查看帮助 |

### 2.3 默认模式（直接运行 Go）

```bash
./scripts/shell/start-dev.sh --mysql
```

执行流程：

1. 读取 `.env` 中的端口和数据库配置。
2. 通过 `docker compose --profile mysql up -d mysql` 启动 MySQL。
3. 轮询容器健康状态，最多等待 4 分钟。
4. 检查应用端口是否被占用。
5. 执行 `go run .`，Ctrl+C 停止应用。

数据库和 Redis 容器会常驻后台，不会随应用退出而停止。

### 2.4 Docker 模式

```bash
./scripts/shell/start-dev.sh --docker --postgres --redis
```

该模式会：

1. 构建 `simple:latest` 镜像。
2. 启动应用、Nginx，以及指定的数据库/Redis。
3. 等待应用和 Nginx 健康检查通过。
4. 跟随应用日志，Ctrl+C 只退出日志，容器继续运行。

开发环境地址：

- 应用直连：`http://127.0.0.1:${HTTP_PORT}`，默认 `8888`
- Nginx 代理：`http://127.0.0.1:${NGINX_PORT}`，默认 `8080`

### 2.5 停止开发环境

```bash
./scripts/shell/end-dev.sh
```

只停止开发栈，不删除数据。MySQL、PostgreSQL、Redis 的数据分别保存在：

```text
data/mysql/
data/postgres/
data/redis/
```

开发 Compose 的项目名是 `simple-dev`，生产栈是 `simple`，停止其中一方不会影响另一方；但两者使用的宿主机端口不能重复占用。

---

## 3. 生产部署

### 3.1 一键引导部署

```bash
./scripts/shell/start-prod.sh
```

脚本必须在交互式终端中运行。整个过程中所有配置逐项询问，直接回车使用方括号中的默认值。

### 3.2 交互步骤

1. **选择数据库**：`1` 为 MySQL，`2` 为 PostgreSQL，默认 MySQL。
2. **是否启用 Redis**：默认不启用。
3. **Nginx 对外端口**：默认 `80`；如果端口已被占用，会提示是否继续或重新输入。
4. **对外域名**：没有域名直接回车，使用 `_`。
5. **数据库配置**：
   - MySQL：数据库名、业务用户、业务密码、root 密码。
   - PostgreSQL：数据库名、用户、密码。
6. **Redis 密码**：仅在启用 Redis 时询问。
7. **部署摘要确认**：确认后开始构建镜像。
8. **可选检查**：如果本机安装了 Go，会询问是否执行 `go vet ./...` 和 `go test ./...`。
9. **构建镜像**：生成 `simple:prod`。
10. **启动并验证**：等待 `/api/v1/ping` 返回成功，最多 3 分钟。
11. **清理源码**：部署成功后询问是否只保留 `deploy/`。

### 3.3 密码规则

密码至少 8 位，且不能包含：

- 空格
- `$`
- `#`
- 双引号 `"`
- 单引号 `'`
- 反斜杠 `\`
- 反引号 `` ` ``

这些限制是为了保证密码可以安全地写入 `.env`，并被 Docker Compose 正确解析。密码输入时不回显，需要输入两次确认。

### 3.4 生成的运行目录

部署完成后会生成自包含的 `deploy/` 目录：

```text
deploy/
├── .env                       # 环境变量和密码，权限 600
├── docker-compose.yml         # 生产编排文件
├── README.md                  # 本目录的运维说明
├── start.sh                   # 启动
├── stop.sh                    # 停止
├── restart.sh                 # 重启
├── status.sh                  # 查看容器状态
├── logs.sh                    # 跟随日志
├── nginx/conf.d/simple.conf   # 反向代理配置
├── scripts/sql/               # MySQL 首次启动执行的初始化 SQL
└── data/                      # 数据库和 Redis 持久化数据
```

`deploy/` 不再依赖源码目录，移动或保留它即可继续运行。

### 3.5 验证地址

部署脚本会在本机请求：

```text
http://127.0.0.1:<NGINX_PORT>/api/v1/ping
```

成功后对外访问地址为：

```text
http://<服务器IP>/api/v1/ping
http://<服务器IP>:<NGINX_PORT>/api/v1/ping
```

端口为 `80` 时不需要写端口号。

### 3.6 清理源码

部署成功后会询问：

```text
现在清理源码，只保留 deploy/ 吗 [Y/n]:
```

选择清理后：

1. 默认先把源码打包到 `/tmp/simple-source-backup-<时间戳>.tar.gz`，备份不包含 `deploy/`、`data/`、`logs/`、`.tmp/`。
2. 要求输入 `DELETE` 进行二次确认。
3. 删除项目根目录下除 `deploy/` 以外的所有内容，包括源码、`.git`、`.env`、脚本和开发数据。

清理完成后，项目根目录只剩下 `deploy/`。此时无法再从源码重新部署，需要重新获取源码。

### 3.7 日常运维

```bash
cd deploy

./start.sh        # 启动
./stop.sh         # 停止
./restart.sh      # 重启
./status.sh       # 查看状态
./logs.sh         # 跟随全部日志
./logs.sh simple  # 只看应用日志
```

停止生产栈时也可以从源码目录执行：

```bash
./scripts/shell/end-prod.sh
```

如果已经清理源码，该脚本会被删除，请使用 `deploy/stop.sh`。

### 3.8 重新部署

清理源码后，需要重新获取源码，再执行：

```bash
./scripts/shell/start-prod.sh
```

部署脚本检测到已有的 `simple` 项目容器时，会先停止并移除旧容器，再启动新容器；数据库数据保存在 `deploy/data/`，不会因为重新部署而丢失。

---

## 4. 配置参考

### 4.1 本地开发 `.env`

`.env.example` 中的配置默认全部是注释状态。不设置时，脚本和 Compose 会使用下表中的默认值；取消注释并修改后即可覆盖。

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_ENV` | `dev` | 运行环境：`dev` / `test` / `prod` |
| `HTTP_ADDR` | `:8888` | Go 进程监听地址 |
| `HTTP_PORT` | `8888` | Docker 模式下的宿主机端口 |
| `NGINX_PORT` | `8080` | 开发 Nginx 端口 |
| `MYSQL_PORT` | `3306` | MySQL 宿主机端口 |
| `MYSQL_DATABASE` | `simple` | 数据库名 |
| `MYSQL_USER` | `simple` | 数据库用户 |
| `MYSQL_PASSWORD` | `simple` | 数据库密码 |
| `MYSQL_ROOT_PASSWORD` | `simple-root` | MySQL root 密码 |
| `POSTGRES_PORT` | `5432` | PostgreSQL 宿主机端口 |
| `POSTGRES_DB` | `simple` | 数据库名 |
| `POSTGRES_USER` | `simple` | 数据库用户 |
| `POSTGRES_PASSWORD` | `simple` | 数据库密码 |
| `REDIS_PORT` | `6379` | Redis 宿主机端口 |
| `REDIS_PASSWORD` | 空 | 开发环境默认不设置密码 |

开发环境的默认值只用于本地调试，不要用于生产。

### 4.2 生产 `deploy/.env`

生产环境由脚本生成，主要变量包括：

| 变量 | 说明 |
| --- | --- |
| `APP_ENV` | 固定为 `prod` |
| `HTTP_ADDR` | 固定为 `:8888` |
| `NGINX_PORT` | 对外端口 |
| `SERVER_NAME` | Nginx `server_name` |
| `DB_ENGINE` | `mysql` 或 `postgres` |
| `WITH_REDIS` | `0` 或 `1` |
| `MYSQL_*` | MySQL 连接信息，仅 MySQL 部署时生成 |
| `POSTGRES_*` | PostgreSQL 连接信息，仅 PostgreSQL 部署时生成 |
| `REDIS_*` | Redis 连接信息，仅启用 Redis 时生成 |

`.env` 权限为 `600`，包含密码，不要提交到 Git，也不要直接发送给他人。

---

## 5. 故障排查

### Docker 未运行或没有权限

```text
无法连接 Docker 守护进程，请确认 Docker 正在运行，且当前用户有权限
```

Linux 下可以把当前用户加入 `docker` 组，或使用有权限的账号执行：

```bash
sudo usermod -aG docker "$USER"
```

重新登录后生效。

### 端口被占用

```bash
lsof -iTCP:80 -sTCP:LISTEN
```

开发环境也可以直接用 `--port` 换端口：

```bash
./scripts/shell/start-dev.sh --port 9000
```

生产环境在部署时会提示端口占用，可以重新输入。

### 健康检查失败

```bash
cd deploy
./status.sh
./logs.sh
```

常见原因：

- 镜像构建失败或启动参数错误。
- 数据库首次初始化时间较长。
- 端口被其他程序占用。
- 服务器无法拉取 Docker 镜像。

### 修改生产配置

编辑 `deploy/.env` 后执行：

```bash
cd deploy
./restart.sh
```

不要直接修改 `deploy/docker-compose.yml` 中的密码，密码统一放在 `.env`。

### 恢复源码备份

如果清理源码时选择了备份，备份文件在：

```text
/tmp/simple-source-backup-<时间戳>.tar.gz
```

恢复：

```bash
mkdir -p /path/to/restore
tar -xzf /tmp/simple-source-backup-<时间戳>.tar.gz -C /path/to/restore
```

备份不包含 `deploy/`、`data/`、`logs/` 和 `.tmp/`。

---

## 6. 注意事项

- 当前应用的数据访问仍使用内存仓库，MySQL/PostgreSQL/Redis 容器和环境变量已经准备好，但 Go 代码尚未真正读写数据库。
- MySQL 的 `scripts/sql/` 只会在数据目录为空的首次启动时执行；已有数据后修改 SQL 不会自动重新执行。
- 生产环境的应用端口 `8888` 只在 Docker 内部暴露，对外统一经过 Nginx。
- 开发环境使用项目名 `simple-dev`，生产环境使用 `simple`，避免误停对方的容器。
- 部署脚本默认会备份源码，但 `/tmp` 可能在重启后清空；需要长期保留时请手动转存备份。
