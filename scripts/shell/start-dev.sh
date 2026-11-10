#!/usr/bin/env bash
#
# 本地开发启动脚本。
#
# 用法:
#   ./scripts/shell/start-dev.sh               # 只运行 Go 应用
#   ./scripts/shell/start-dev.sh --mysql       # 先启动 MySQL，再运行应用
#   ./scripts/shell/start-dev.sh --postgres    # 先启动 PostgreSQL，再运行应用
#   ./scripts/shell/start-dev.sh --redis       # 先启动 Redis，再运行应用
#   ./scripts/shell/start-dev.sh --docker      # 用 Docker 启动应用 + Nginx
#   ./scripts/shell/start-dev.sh --port 9000   # 覆盖本地监听端口
#
# 数据库/Redis 容器会常驻后台，用 ./scripts/shell/end-dev.sh 停止。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

DB=""
WITH_REDIS=0
USE_DOCKER=0
PORT_OVERRIDE=""

if [[ -t 1 ]]; then
    BOLD=$'\033[1m'
    RED=$'\033[31m'
    GREEN=$'\033[32m'
    YELLOW=$'\033[33m'
    RESET=$'\033[0m'
else
    BOLD=""
    RED=""
    GREEN=""
    YELLOW=""
    RESET=""
fi

step() { printf '\n%s==> %s%s\n' "$BOLD" "$*" "$RESET"; }
say() { printf '%s\n' "$*"; }
warn() { printf '%s%s%s\n' "$YELLOW" "$*" "$RESET" >&2; }
die() {
    printf '%s%s%s\n' "$RED" "$*" "$RESET" >&2
    exit 1
}

usage() {
    cat <<'EOF'
本地开发启动脚本

用法:
  ./scripts/shell/start-dev.sh               # 只运行 Go 应用
  ./scripts/shell/start-dev.sh --mysql       # 先启动 MySQL，再运行应用
  ./scripts/shell/start-dev.sh --postgres    # 先启动 PostgreSQL，再运行应用
  ./scripts/shell/start-dev.sh --redis       # 先启动 Redis，再运行应用
  ./scripts/shell/start-dev.sh --docker      # 用 Docker 启动应用 + Nginx
  ./scripts/shell/start-dev.sh --port 9000   # 覆盖本地监听端口

数据库/Redis 容器会常驻后台，用 ./scripts/shell/end-dev.sh 停止。
EOF
}

while (( $# > 0 )); do
    case "$1" in
        --mysql | --postgres)
            DB="${1#--}"
            ;;
        --redis)
            WITH_REDIS=1
            ;;
        --docker)
            USE_DOCKER=1
            ;;
        --port)
            if (( $# < 2 )); then
                die "--port 后面需要端口号"
            fi
            PORT_OVERRIDE="$2"
            shift
            ;;
        -h | --help)
            usage
            exit 0
            ;;
        *)
            die "未知参数: $1（用 --help 查看用法）"
            ;;
    esac
    shift
done

if [[ -n "$PORT_OVERRIDE" ]]; then
    if [[ ! "$PORT_OVERRIDE" =~ ^[0-9]+$ ]] \
        || (( 10#$PORT_OVERRIDE < 1 || 10#$PORT_OVERRIDE > 65535 )); then
        die "--port 必须是 1-65535 之间的数字"
    fi
fi

if [[ ! -f .env ]]; then
    cp .env.example .env
    say "已根据 .env.example 创建 .env（开发默认值，请勿用于生产）"
fi

env_value() {
    local key="$1" line
    line="$(grep -E "^[[:space:]]*${key}[[:space:]]*=" .env 2>/dev/null | tail -n 1 || true)"
    line="${line#*=}"
    line="${line%$'\r'}"
    line="$(printf '%s' "$line" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//')"
    case "$line" in
        \"*\")
            line="${line#\"}"
            line="${line%\"}"
            ;;
    esac
    printf '%s' "$line"
}

HTTP_ADDR="$(env_value HTTP_ADDR)"
HTTP_ADDR="${HTTP_ADDR:-:8888}"
HTTP_PORT="$(env_value HTTP_PORT)"
HTTP_PORT="${HTTP_PORT:-8888}"
NGINX_PORT="$(env_value NGINX_PORT)"
NGINX_PORT="${NGINX_PORT:-8080}"
MYSQL_PORT="$(env_value MYSQL_PORT)"
MYSQL_PORT="${MYSQL_PORT:-3306}"
MYSQL_DATABASE="$(env_value MYSQL_DATABASE)"
MYSQL_DATABASE="${MYSQL_DATABASE:-simple}"
MYSQL_USER="$(env_value MYSQL_USER)"
MYSQL_USER="${MYSQL_USER:-simple}"
POSTGRES_PORT="$(env_value POSTGRES_PORT)"
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
POSTGRES_DB="$(env_value POSTGRES_DB)"
POSTGRES_DB="${POSTGRES_DB:-simple}"
POSTGRES_USER="$(env_value POSTGRES_USER)"
POSTGRES_USER="${POSTGRES_USER:-simple}"
APP_PORT="${HTTP_ADDR##*:}"

if [[ -n "$PORT_OVERRIDE" ]]; then
    HTTP_ADDR=":$PORT_OVERRIDE"
    APP_PORT="$PORT_OVERRIDE"
    HTTP_PORT="$PORT_OVERRIDE"
fi

if [[ ! "$APP_PORT" =~ ^[0-9]+$ ]]; then
    die "HTTP_ADDR 必须是 :端口 或 host:端口 的形式（当前: $HTTP_ADDR）"
fi
if [[ ! "$HTTP_PORT" =~ ^[0-9]+$ ]]; then
    die ".env 中的 HTTP_PORT 不是有效端口: $HTTP_PORT"
fi
if [[ ! "$NGINX_PORT" =~ ^[0-9]+$ ]]; then
    die ".env 中的 NGINX_PORT 不是有效端口: $NGINX_PORT"
fi

export HTTP_ADDR HTTP_PORT

need_docker() {
    command -v docker >/dev/null 2>&1 || die "未找到 docker，请先安装 Docker"
    docker compose version >/dev/null 2>&1 || die "未找到 docker compose 插件，请安装 Docker Compose v2"
    docker info >/dev/null 2>&1 || die "无法连接 Docker 守护进程，请确认 Docker 正在运行"
}

port_in_use() {
    local port="$1"
    if command -v lsof >/dev/null 2>&1; then
        if lsof -iTCP:"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            return 0
        fi
        return 1
    fi
    if command -v ss >/dev/null 2>&1; then
        if ss -ltn 2>/dev/null | grep -q ":$port "; then
            return 0
        fi
        return 1
    fi
    return 1
}

profiles=()
services=()
if [[ -n "$DB" ]]; then
    profiles+=(--profile "$DB")
    services+=("$DB")
fi
if (( WITH_REDIS )); then
    profiles+=(--profile redis)
    services+=(redis)
fi

# 始终是一个非空数组，避免 bash 3.2 在 set -u 下展开空数组报错。
compose=(docker compose)
if (( ${#profiles[@]} > 0 )); then
    compose+=("${profiles[@]}")
fi

wait_service_healthy() {
    local service="$1" timeout="${2:-120}" elapsed=0 id status
    while (( elapsed < timeout )); do
        id="$("${compose[@]}" ps -q "$service" 2>/dev/null | head -n 1 || true)"
        if [[ -n "$id" ]]; then
            status="$(docker inspect \
                -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \
                "$id" 2>/dev/null || true)"
            case "$status" in
                healthy | running)
                    return 0
                    ;;
                unhealthy)
                    warn "$service 状态为 unhealthy"
                    return 1
                    ;;
            esac
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done
    return 1
}

if (( USE_DOCKER )); then
    need_docker
    step "构建并启动 Docker 开发栈"
    "${compose[@]}" up -d --build

    step "等待容器就绪"
    if ! wait_service_healthy simple 180; then
        "${compose[@]}" logs --tail=80 simple || true
        die "应用容器没有在 3 分钟内就绪"
    fi
    if ! wait_service_healthy nginx 60; then
        "${compose[@]}" logs --tail=80 nginx || true
        die "Nginx 容器没有就绪"
    fi

    say ""
    say "API 直连 : http://127.0.0.1:${HTTP_PORT}/api/v1/ping"
    say "Nginx    : http://127.0.0.1:${NGINX_PORT}/api/v1/ping"
    say "查看日志 : docker compose logs -f simple"
    say "停止环境 : scripts/shell/end-dev.sh"
    say ""
    say "下面跟随应用日志，Ctrl+C 只退出日志，容器继续在后台运行。"
    exec "${compose[@]}" logs -f --tail=50 simple
fi

if (( ${#services[@]} > 0 )); then
    need_docker
    step "启动基础设施: ${services[*]}"
    "${compose[@]}" up -d "${services[@]}"

    for service in "${services[@]}"; do
        if ! wait_service_healthy "$service" 240; then
            "${compose[@]}" logs --tail=80 "$service" || true
            die "$service 没有在 4 分钟内就绪"
        fi
        say "$service 已就绪"
    done

    case "$DB" in
        mysql)
            say "MySQL    : 127.0.0.1:${MYSQL_PORT}  库 ${MYSQL_DATABASE}  用户 ${MYSQL_USER}"
            ;;
        postgres)
            say "Postgres : 127.0.0.1:${POSTGRES_PORT}  库 ${POSTGRES_DB}  用户 ${POSTGRES_USER}"
            ;;
    esac
    if (( WITH_REDIS )); then
        say "Redis    : 127.0.0.1:6379"
    fi
    say "基础设施已常驻后台，停止用 scripts/shell/end-dev.sh"
fi

command -v go >/dev/null 2>&1 || die "未找到 go，请先安装 Go 1.23 或更高版本"

if port_in_use "$APP_PORT"; then
    die "端口 ${APP_PORT} 已被占用。若是本项目的 Docker 栈，请先运行 scripts/shell/end-dev.sh，或用 --port 指定其它端口"
fi

step "启动 Go 应用"
APP_ENV_SHOW="$(env_value APP_ENV)"
APP_ENV_SHOW="${APP_ENV_SHOW:-dev}"
say "监听地址 : http://127.0.0.1:${APP_PORT}"
say "运行环境 : APP_ENV=${APP_ENV_SHOW}"
say "Ctrl+C 停止应用（基础设施不会自动停止）"
say ""

exec go run .
