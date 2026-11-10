#!/usr/bin/env bash
#
# 停止开发环境。
#
# 会停止 start-dev.sh 启动的 API、Nginx、MySQL、PostgreSQL 和 Redis 容器。
# 数据库数据保留在 ./data，不会删除。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
    echo "未找到 docker compose，无需停止。" >&2
    exit 0
fi

docker compose --profile mysql --profile postgres --profile redis down --remove-orphans

echo
echo "开发环境已停止。"
echo "数据仍保留在 $ROOT/data"
