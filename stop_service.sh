#!/usr/bin/env bash
# stop_service.sh — 停止农趣村后端服务
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CYAN='\033[0;36m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'; RESET='\033[0m'
log()  { echo -e "${CYAN}[stop]${RESET} $*"; }
warn() { echo -e "${YELLOW}[stop]${RESET} $*"; }
ok()   { echo -e "${GREEN}[stop]${RESET} $*"; }

# ── 1. 通过 PID 文件停止后端进程 ─────────────────────────────────────────────
LOGS_DIR="$ROOT_DIR/logs"
PIDFILE="$LOGS_DIR/server.pid"
if [ -f "$PIDFILE" ]; then
    PID=$(cat "$PIDFILE")
    if kill -0 "$PID" 2>/dev/null; then
        warn "停止后端进程 (PID $PID)…"
        kill "$PID" 2>/dev/null || true
        for _ in 1 2 3; do kill -0 "$PID" 2>/dev/null || break; sleep 1; done
        kill -9 "$PID" 2>/dev/null || true
    fi
    rm -f "$PIDFILE"
fi

# ── 2. 释放端口（兜底） ───────────────────────────────────────────────────────
PIDS=$(lsof -ti tcp:9080 2>/dev/null || true)
if [ -n "$PIDS" ]; then
    warn "端口 9080 仍被占用，强制终止…"
    echo "$PIDS" | xargs kill -9 2>/dev/null || true
fi

# ── 3. 停止 Docker 容器 ───────────────────────────────────────────────────────
cd "$ROOT_DIR"
if docker compose ps -q 2>/dev/null | grep -q .; then
    log "停止 Docker 容器…"
    docker compose down --remove-orphans
else
    log "Docker 容器未运行，跳过"
fi

ok "服务已停止 ✓"
