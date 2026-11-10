#!/usr/bin/env bash
#
# simple 生产部署脚本（交互式引导）。
#
# 每一项配置单独提问，直接回车使用方括号中的默认值。
# 部署完成后生成自包含的 deploy/ 运行目录，源码可以全部删除。
#
# 用法:
#   ./scripts/shell/start-prod.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DEPLOY_DIR="${DEPLOY_DIR:-$ROOT/deploy}"
IMAGE="${IMAGE:-simple:prod}"

DB=""
WITH_REDIS=0
NGINX_PORT=""
SERVER_NAME=""
ACCESS_URL=""
LOCAL_URL=""

MYSQL_DATABASE=""
MYSQL_USER=""
MYSQL_PASSWORD=""
MYSQL_ROOT_PASSWORD=""

POSTGRES_DB=""
POSTGRES_USER=""
POSTGRES_PASSWORD=""

REDIS_PASSWORD=""

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
simple 生产部署脚本（交互式引导）

用法:
  ./scripts/shell/start-prod.sh

部署过程会：
  1. 逐项询问数据库类型、密码、端口、域名等配置
  2. 构建 simple:prod 镜像
  3. 生成自包含的 deploy/ 运行目录并启动服务
  4. 验证 /api/v1/ping 后，询问是否删除源码只保留 deploy/
EOF
}

ask() {
    local prompt="$1" default="${2:-}" answer
    if [[ -n "$default" ]]; then
        read -r -p "$prompt [$default]: " answer
    else
        read -r -p "$prompt: " answer
    fi
    printf '%s' "${answer:-${default}}"
}

valid_password() {
    local value="$1"
    if (( ${#value} < 8 )); then
        return 1
    fi
    if [[ "$value" == *[[:space:]]* ]]; then
        return 1
    fi
    if [[ "$value" == *'$'* || "$value" == *'#'* || "$value" == *'"'* \
        || "$value" == *"'"* || "$value" == *'\'* || "$value" == *'`'* ]]; then
        return 1
    fi
    return 0
}

ask_secret() {
    local prompt="$1" value confirm
    while true; do
        read -r -s -p "$prompt: " value
        printf '\n' >&2
        if [[ -z "$value" ]]; then
            warn "不能为空，请重新输入"
            continue
        fi
        if ! valid_password "$value"; then
            warn "至少 8 位，且不能包含空格、\$、#、引号、反斜杠或反引号"
            continue
        fi
        read -r -s -p "$prompt（再输入一次）: " confirm
        printf '\n' >&2
        if [[ "$value" != "$confirm" ]]; then
            warn "两次输入不一致，请重试"
            continue
        fi
        printf '%s' "$value"
        return 0
    done
}

confirm() {
    local prompt="$1" default="${2:-N}" hint answer
    if [[ "$default" == "Y" ]]; then
        hint="Y/n"
    else
        hint="y/N"
    fi
    read -r -p "$prompt [$hint]: " answer
    answer="${answer:-$default}"
    [[ "$answer" == "y" || "$answer" == "Y" ]]
}

valid_port() {
    local value="$1"
    if [[ ! "$value" =~ ^[0-9]+$ ]]; then
        return 1
    fi
    if (( 10#$value < 1 || 10#$value > 65535 )); then
        return 1
    fi
    return 0
}

valid_name() {
    [[ "$1" =~ ^[A-Za-z_][A-Za-z0-9_]{0,31}$ ]]
}

valid_server_name() {
    [[ "$1" =~ ^[A-Za-z0-9_.*-]+$ ]]
}

ask_port() {
    local prompt="$1" default="$2" value
    while true; do
        value="$(ask "$prompt" "$default")"
        if valid_port "$value"; then
            printf '%s' "$value"
            return 0
        fi
        warn "端口必须是 1-65535 之间的数字"
    done
}

ask_name() {
    local prompt="$1" default="$2" value
    while true; do
        value="$(ask "$prompt" "$default")"
        if valid_name "$value"; then
            printf '%s' "$value"
            return 0
        fi
        warn "只能使用字母、数字和下划线，且不能以数字开头，长度 1-32"
    done
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

need_docker() {
    command -v docker >/dev/null 2>&1 || die "未找到 docker，请先安装 Docker Engine"
    docker compose version >/dev/null 2>&1 || die "未找到 docker compose 插件，请安装 Docker Compose v2"
    docker info >/dev/null 2>&1 || die "无法连接 Docker 守护进程，请确认 Docker 正在运行，且当前用户有权限"
}

http_ping_ok() {
    local url="$1"
    if command -v curl >/dev/null 2>&1; then
        curl -fsS --max-time 2 "$url" >/dev/null 2>&1
        return $?
    fi
    if command -v wget >/dev/null 2>&1; then
        wget -qO- --timeout=2 "$url" >/dev/null 2>&1
        return $?
    fi
    return 1
}

generate_deploy() {
    step "生成运行目录 $DEPLOY_DIR"

    [[ -f "$ROOT/nginx/conf.d/simple.conf" ]] || die "缺少 nginx/conf.d/simple.conf，无法生成运行目录"

    mkdir -p "$DEPLOY_DIR/nginx/conf.d" "$DEPLOY_DIR/data"

    if [[ -f "$DEPLOY_DIR/.env" ]]; then
        local env_backup
        env_backup="$DEPLOY_DIR/.env.bak.$(date +%Y%m%d%H%M%S)"
        cp "$DEPLOY_DIR/.env" "$env_backup"
        chmod 600 "$env_backup"
        warn "已备份旧配置到 $env_backup"
    fi

    local old_umask
    old_umask="$(umask)"
    umask 077
    {
        echo "# 由 scripts/shell/start-prod.sh 生成于 $(date '+%Y-%m-%d %H:%M:%S')"
        echo "# 修改后执行 ./restart.sh 生效"
        echo "APP_ENV=prod"
        echo "HTTP_ADDR=:8888"
        echo "NGINX_PORT=$NGINX_PORT"
        echo "SERVER_NAME=$SERVER_NAME"
        echo "DB_ENGINE=$DB"
        echo "WITH_REDIS=$WITH_REDIS"
        case "$DB" in
            mysql)
                echo "MYSQL_HOST=mysql"
                echo "MYSQL_PORT=3306"
                echo "MYSQL_DATABASE=$MYSQL_DATABASE"
                echo "MYSQL_USER=$MYSQL_USER"
                echo "MYSQL_PASSWORD=$MYSQL_PASSWORD"
                echo "MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD"
                ;;
            postgres)
                echo "POSTGRES_HOST=postgres"
                echo "POSTGRES_PORT=5432"
                echo "POSTGRES_DB=$POSTGRES_DB"
                echo "POSTGRES_USER=$POSTGRES_USER"
                echo "POSTGRES_PASSWORD=$POSTGRES_PASSWORD"
                ;;
        esac
        if (( WITH_REDIS )); then
            echo "REDIS_HOST=redis"
            echo "REDIS_PORT=6379"
            echo "REDIS_PASSWORD=$REDIS_PASSWORD"
        fi
    } > "$DEPLOY_DIR/.env"
    chmod 600 "$DEPLOY_DIR/.env"
    umask "$old_umask"

    sed "s/server_name _;/server_name ${SERVER_NAME};/" \
        "$ROOT/nginx/conf.d/simple.conf" > "$DEPLOY_DIR/nginx/conf.d/simple.conf"

    if [[ "$DB" == "mysql" && -d "$ROOT/scripts/sql" ]]; then
        mkdir -p "$DEPLOY_DIR/scripts/sql"
        cp -R "$ROOT/scripts/sql/." "$DEPLOY_DIR/scripts/sql/"
    fi

    local db_env_block="" db_service_block="" depends_block=""
    local redis_env_block="" redis_service_block=""

    case "$DB" in
        mysql)
            db_env_block=$(cat <<'YAML'
      MYSQL_HOST: mysql
      MYSQL_PORT: "3306"
      MYSQL_DATABASE: ${MYSQL_DATABASE}
      MYSQL_USER: ${MYSQL_USER}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
YAML
)
            db_service_block=$(cat <<'YAML'

  mysql:
    image: mysql:8.4
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: ${MYSQL_ROOT_PASSWORD}
      MYSQL_DATABASE: ${MYSQL_DATABASE}
      MYSQL_USER: ${MYSQL_USER}
      MYSQL_PASSWORD: ${MYSQL_PASSWORD}
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_0900_ai_ci
    volumes:
      - ./data/mysql:/var/lib/mysql
      - ./scripts/sql:/docker-entrypoint-initdb.d:ro
    healthcheck:
      test: ["CMD-SHELL", "mysqladmin ping -h 127.0.0.1 -u$$MYSQL_USER -p$$MYSQL_PASSWORD --silent"]
      interval: 10s
      timeout: 5s
      retries: 12
      start_period: 40s
YAML
)
            depends_block=$(cat <<'YAML'
    depends_on:
      mysql:
        condition: service_healthy
YAML
)
            ;;
        postgres)
            db_env_block=$(cat <<'YAML'
      POSTGRES_HOST: postgres
      POSTGRES_PORT: "5432"
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
YAML
)
            db_service_block=$(cat <<'YAML'

  postgres:
    image: postgres:16-alpine
    restart: always
    environment:
      POSTGRES_DB: ${POSTGRES_DB}
      POSTGRES_USER: ${POSTGRES_USER}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 10s
      timeout: 5s
      retries: 12
      start_period: 20s
YAML
)
            depends_block=$(cat <<'YAML'
    depends_on:
      postgres:
        condition: service_healthy
YAML
)
            ;;
    esac

    if (( WITH_REDIS )); then
        redis_env_block=$(cat <<'YAML'
      REDIS_HOST: redis
      REDIS_PORT: "6379"
      REDIS_PASSWORD: ${REDIS_PASSWORD}
YAML
)
        redis_service_block=$(cat <<'YAML'

  redis:
    image: redis:7-alpine
    restart: always
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "${REDIS_PASSWORD}"]
    volumes:
      - ./data/redis:/data
    healthcheck:
      test: ["CMD-SHELL", "redis-cli -a $$REDIS_PASSWORD ping | grep -q PONG"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 10s
YAML
)
        depends_block+=$'\n      redis:\n        condition: service_healthy'
    fi

    cat > "$DEPLOY_DIR/docker-compose.yml" <<YAML
name: simple

services:
  simple:
    image: ${IMAGE}
    restart: always
    stop_grace_period: 15s
    environment:
      APP_ENV: prod
      HTTP_ADDR: ":8888"
      DB_ENGINE: \${DB_ENGINE}
      WITH_REDIS: "\${WITH_REDIS}"
${db_env_block}
${redis_env_block}
${depends_block}
    expose:
      - "8888"

  nginx:
    image: nginx:1.27-alpine
    restart: always
    depends_on:
      simple:
        condition: service_healthy
    ports:
      - "\${NGINX_PORT}:80"
    volumes:
      - ./nginx/conf.d:/etc/nginx/conf.d:ro
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://127.0.0.1/api/v1/ping >/dev/null 2>&1 || exit 1"]
      interval: 10s
      timeout: 3s
      retries: 5
      start_period: 10s
${db_service_block}
${redis_service_block}
YAML

    cat > "$DEPLOY_DIR/start.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
docker compose up -d
docker compose ps
SH

    cat > "$DEPLOY_DIR/stop.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
docker compose down
SH

    cat > "$DEPLOY_DIR/restart.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
docker compose down
docker compose up -d
docker compose ps
SH

    cat > "$DEPLOY_DIR/status.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
docker compose ps
SH

    cat > "$DEPLOY_DIR/logs.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
docker compose logs -f --tail=200 "$@"
SH

    local redis_text="未启用"
    if (( WITH_REDIS )); then
        redis_text="已启用"
    fi

    cat > "$DEPLOY_DIR/README.md" <<EOF
# simple 生产运行目录

- 部署时间：$(date '+%Y-%m-%d %H:%M:%S')
- 访问地址：${ACCESS_URL}
- 对外域名：${SERVER_NAME}
- Nginx 端口：${NGINX_PORT}
- 数据库：${DB}
- Redis：${redis_text}

## 常用命令

    ./start.sh        启动
    ./stop.sh         停止
    ./restart.sh      重启
    ./status.sh       查看状态
    ./logs.sh         跟随全部日志
    ./logs.sh simple  只看应用日志

## 文件说明

- docker-compose.yml：生产编排文件
- .env：环境变量和密码，权限 600，请勿外传
- nginx/conf.d/simple.conf：Nginx 反向代理配置
- data/：数据库和 Redis 的持久化数据
- scripts/sql/：MySQL 部署时存在，首次启动执行的初始化 SQL

## 注意

- 应用镜像 ${IMAGE} 已构建在本机 Docker 中。
- 修改 .env 后执行 ./restart.sh 生效。
- 备份时请同时备份本目录和数据库数据。
EOF

    chmod +x \
        "$DEPLOY_DIR/start.sh" \
        "$DEPLOY_DIR/stop.sh" \
        "$DEPLOY_DIR/restart.sh" \
        "$DEPLOY_DIR/status.sh" \
        "$DEPLOY_DIR/logs.sh"

    say "运行目录已生成:"
    (cd "$DEPLOY_DIR" && find . -maxdepth 3 -type f | sort | sed 's|^\./|  |')
}

start_and_verify() {
    step "启动服务"
    if ! (cd "$DEPLOY_DIR" && docker compose down --remove-orphans); then
        warn "停止旧容器时出现问题，继续尝试启动"
    fi

    (cd "$DEPLOY_DIR" && docker compose up -d)
    (cd "$DEPLOY_DIR" && docker compose ps)

    step "等待服务就绪（最多 3 分钟）"
    local ready=0 i id status
    if command -v curl >/dev/null 2>&1 || command -v wget >/dev/null 2>&1; then
        for i in $(seq 1 90); do
            if http_ping_ok "$LOCAL_URL"; then
                ready=1
                break
            fi
            sleep 2
        done
    else
        warn "未找到 curl/wget，改为检查 Nginx 容器健康状态"
        for i in $(seq 1 90); do
            id="$(cd "$DEPLOY_DIR" && docker compose ps -q nginx 2>/dev/null | head -n 1 || true)"
            if [[ -n "$id" ]]; then
                status="$(docker inspect \
                    -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' \
                    "$id" 2>/dev/null || true)"
                if [[ "$status" == "healthy" ]]; then
                    ready=1
                    break
                fi
            fi
            sleep 2
        done
    fi

    if (( ! ready )); then
        warn "服务没有在预期时间内就绪，下面是当前状态和日志："
        (cd "$DEPLOY_DIR" && docker compose ps) || true
        (cd "$DEPLOY_DIR" && docker compose logs --tail=80) || true
        die "部署失败，已保留源码，请根据日志排查"
    fi
}

cleanup_source() {
    step "清理源码"
    say "deploy/ 已经包含运行所需的全部文件。"
    say "清理会删除根目录下除 deploy/ 以外的一切，包括："
    say "  cmd/ internal/ pkg/ scripts/ nginx/ .github/ docs/ mcp/"
    say "  main.go go.mod go.sum Dockerfile docker-compose.yml .env .git"
    say "  以及开发用的 data/ 和 logs/"

    if ! confirm "现在清理源码，只保留 deploy/ 吗" Y; then
        say "已保留源码。以后仍可再次运行本脚本重新部署。"
        return 0
    fi

    if confirm "是否先把源码打包备份到 /tmp" Y; then
        local backup
        backup="/tmp/simple-source-backup-$(date +%Y%m%d-%H%M%S).tar.gz"
        if tar -czf "$backup" \
            --exclude='./deploy' \
            --exclude='./data' \
            --exclude='./logs' \
            --exclude='./.tmp' \
            -C "$ROOT" .; then
            say "源码备份: $backup"
        else
            warn "备份失败"
            if ! confirm "仍然继续清理" N; then
                say "已取消清理，源码保留。"
                return 0
            fi
        fi
    fi

    if [[ "$(dirname "$DEPLOY_DIR")" != "$ROOT" || ! -d "$DEPLOY_DIR" ]]; then
        die "DEPLOY_DIR 必须位于 $ROOT 下，拒绝清理"
    fi

    if [[ ! -f "$DEPLOY_DIR/docker-compose.yml" || ! -f "$DEPLOY_DIR/.env" ]]; then
        die "运行目录不完整，拒绝清理"
    fi

    case "$ROOT" in
        "/" | "$HOME")
            die "当前目录是 $ROOT，拒绝执行清理"
            ;;
    esac

    local answer
    read -r -p "清理不可恢复，请输入 DELETE 确认: " answer
    if [[ "$answer" != "DELETE" ]]; then
        say "输入不匹配，已取消清理，源码保留。"
        return 0
    fi

    find "$ROOT" -mindepth 1 -maxdepth 1 ! -name "$(basename "$DEPLOY_DIR")" -exec rm -rf {} \;

    step "清理完成"
    say "现在只保留 $DEPLOY_DIR"
    (cd "$DEPLOY_DIR" && ls -la)
    say "服务仍在运行，后续用 deploy/start.sh、deploy/stop.sh 管理。"
}

main() {
    case "${1:-}" in
        -h | --help)
            usage
            exit 0
            ;;
        "")
            ;;
        *)
            die "未知参数: $1"
            ;;
    esac

    if [[ ! -t 0 ]]; then
        die "这是交互式脚本，请在终端中直接运行"
    fi

    cd "$ROOT"
    if [[ "$(dirname "$DEPLOY_DIR")" != "$ROOT" ]]; then
        die "DEPLOY_DIR 必须是项目根目录下的子目录"
    fi
    need_docker

    step "simple 生产部署"
    say "接下来逐项询问配置，直接回车使用方括号中的默认值。"
    say "部署成功之前不会修改或删除任何源码。"

    step "第 1 步：选择数据库"
    say "  1) MySQL"
    say "  2) PostgreSQL"
    while true; do
        read -r -p "请选择 [1]: " db_choice
        case "${db_choice:-1}" in
            1)
                DB="mysql"
                break
                ;;
            2)
                DB="postgres"
                break
                ;;
            *)
                warn "请输入 1 或 2"
                ;;
        esac
    done

    step "第 2 步：Redis"
    if confirm "是否启用 Redis" N; then
        WITH_REDIS=1
    fi

    step "第 3 步：Nginx"
    while true; do
        NGINX_PORT="$(ask_port "对外 HTTP 端口（80 表示直接用域名访问）" "80")"
        if port_in_use "$NGINX_PORT"; then
            warn "端口 $NGINX_PORT 已被占用"
            if ! confirm "仍然使用这个端口" N; then
                continue
            fi
        fi
        break
    done

    SERVER_NAME="$(ask "对外域名，没有域名直接回车" "_")"
    while ! valid_server_name "$SERVER_NAME"; do
        warn "域名只能包含字母、数字、点、横杠和下划线"
        SERVER_NAME="$(ask "对外域名，没有域名直接回车" "_")"
    done

    step "第 4 步：数据库配置"
    case "$DB" in
        mysql)
            MYSQL_DATABASE="$(ask_name "数据库名" "simple")"
            while true; do
                MYSQL_USER="$(ask_name "数据库用户" "simple")"
                if [[ "$MYSQL_USER" != "root" ]]; then
                    break
                fi
                warn "MySQL 不能使用 root 作为业务用户"
            done
            MYSQL_PASSWORD="$(ask_secret "数据库密码（至少 8 位，输入时不显示）")"
            MYSQL_ROOT_PASSWORD="$(ask_secret "MySQL root 密码（至少 8 位，输入时不显示）")"
            ;;
        postgres)
            POSTGRES_DB="$(ask_name "数据库名" "simple")"
            POSTGRES_USER="$(ask_name "数据库用户" "simple")"
            POSTGRES_PASSWORD="$(ask_secret "数据库密码（至少 8 位，输入时不显示）")"
            ;;
    esac

    if (( WITH_REDIS )); then
        step "第 5 步：Redis 密码"
        REDIS_PASSWORD="$(ask_secret "Redis 密码（至少 8 位，输入时不显示）")"
    fi

    if [[ "$NGINX_PORT" == "80" ]]; then
        ACCESS_URL="http://<服务器IP>/api/v1/ping"
    else
        ACCESS_URL="http://<服务器IP>:${NGINX_PORT}/api/v1/ping"
    fi
    LOCAL_URL="http://127.0.0.1:${NGINX_PORT}/api/v1/ping"

    step "部署摘要"
    say "数据库     : $DB"
    case "$DB" in
        mysql)
            say "数据库名   : $MYSQL_DATABASE"
            say "数据库用户 : $MYSQL_USER"
            ;;
        postgres)
            say "数据库名   : $POSTGRES_DB"
            say "数据库用户 : $POSTGRES_USER"
            ;;
    esac
    if (( WITH_REDIS )); then
        say "Redis      : 启用"
    else
        say "Redis      : 未启用"
    fi
    say "Nginx 端口 : $NGINX_PORT"
    say "对外域名   : $SERVER_NAME"
    say "访问地址   : $ACCESS_URL"
    say "运行目录   : $DEPLOY_DIR"
    say "镜像       : $IMAGE"
    say "密码不会回显，请自行妥善保存。"
    confirm "确认以上配置，开始部署" Y || die "已取消"

    local existing
    existing="$(docker ps -q --filter label=com.docker.compose.project=simple || true)"
    if [[ -n "$existing" ]]; then
        warn "检测到 simple 项目已有生产容器（可能是上一次部署）。"
        confirm "部署会先停止并移除这些容器，继续吗" Y || die "已取消"
    fi

    if command -v go >/dev/null 2>&1; then
        if confirm "构建前是否运行 go vet / go test" Y; then
            step "运行 go vet ./..."
            (cd "$ROOT" && go vet ./...)
            step "运行 go test ./..."
            (cd "$ROOT" && go test ./...)
        fi
    else
        warn "未找到 go，跳过本地检查（Docker 构建阶段仍会编译）"
    fi

    step "构建镜像 $IMAGE"
    if ! (cd "$ROOT" && docker build -t "$IMAGE" .); then
        die "镜像构建失败，运行目录未修改"
    fi

    generate_deploy
    start_and_verify

    step "部署成功"
    say "访问地址 : $ACCESS_URL"
    say "本机验证 : $LOCAL_URL"
    say "运行目录 : $DEPLOY_DIR"
    say "启动     : $DEPLOY_DIR/start.sh"
    say "停止     : $DEPLOY_DIR/stop.sh"
    say "状态     : $DEPLOY_DIR/status.sh"
    say "日志     : $DEPLOY_DIR/logs.sh"

    cleanup_source
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
