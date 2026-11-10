#!/usr/bin/env bash
#
# 停止生产环境。
#
# 只处理 deploy/ 生成的生产栈（项目名 simple）。开发环境请用 end-dev.sh。
# 部署脚本在清理源码时会同时删除本文件，清理后请使用 deploy/stop.sh。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ -f deploy/docker-compose.yml ]]; then
    cd deploy
    docker compose down
    echo
    echo "生产环境已停止。"
    exit 0
fi

echo "未找到 deploy/docker-compose.yml，没有可停止的生产环境。" >&2
echo "开发环境请使用 scripts/shell/end-dev.sh。" >&2
