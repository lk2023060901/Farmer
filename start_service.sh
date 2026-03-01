#!/usr/bin/env bash
# start_service.sh — 启动农趣村后端服务
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

CYAN='\033[0;36m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; BOLD='\033[1m'; RESET='\033[0m'
log()  { echo -e "${CYAN}[start]${RESET} $*"; }
ok()   { echo -e "${GREEN}[start]${RESET} $*"; }
warn() { echo -e "${YELLOW}[start]${RESET} $*"; }
err()  { echo -e "${RED}[start]${RESET} $*" >&2; exit 1; }

LOGS_DIR="$ROOT_DIR/logs"
mkdir -p "$LOGS_DIR"

# ── 1. 加载环境变量 ───────────────────────────────────────────────────────────
ENV_FILE="$ROOT_DIR/.env"
if [ ! -f "$ENV_FILE" ]; then
    warn ".env 不存在，从 .env.example 复制…"
    cp "$ROOT_DIR/.env.example" "$ENV_FILE"
fi
set -a
source <(grep -v '^\s*#' "$ENV_FILE" | grep -v '^\s*$')
set +a

# ── 2. 启动 Docker（PostgreSQL + Redis） ──────────────────────────────────────
log "启动 Docker 容器…"
cd "$ROOT_DIR"
docker compose up -d

log "等待 PostgreSQL 就绪…"
MAX_WAIT=30; WAITED=0
until docker compose exec -T postgres pg_isready -U farmer -d farmer_dev &>/dev/null; do
    WAITED=$((WAITED + 1))
    [ "$WAITED" -ge "$MAX_WAIT" ] && err "PostgreSQL 未就绪，请检查：docker compose logs postgres"
    printf '.'; sleep 1
done
echo
ok "PostgreSQL 就绪 ✓"

# ── 3. 数据库初始化 ───────────────────────────────────────────────────────────
"$ROOT_DIR/scripts/setup_db.sh"

# ── 4. 启动 Go 后端 ───────────────────────────────────────────────────────────
log "启动 Go 后端 (port ${SERVER_PORT:-9080})…"
cd "$ROOT_DIR/server"
nohup env \
    SERVER_PORT="${SERVER_PORT:-9080}" \
    SERVER_MODE="${SERVER_MODE:-debug}" \
    DATABASE_DSN="${DATABASE_DSN:-postgres://farmer:farmer_secret@localhost:5433/farmer_dev?sslmode=disable}" \
    REDIS_ADDR="${REDIS_ADDR:-localhost:6379}" \
    REDIS_PASSWORD="${REDIS_PASSWORD:-}" \
    JWT_SECRET="${JWT_SECRET:-change_me_in_production}" \
    go run ./cmd/server \
    > "$LOGS_DIR/server.log" 2>&1 &
SERVER_PID=$!
echo "$SERVER_PID" > "$LOGS_DIR/server.pid"

log "等待后端就绪…"
MAX_WAIT=15; WAITED=0
until curl -sf "http://localhost:${SERVER_PORT:-9080}/health" &>/dev/null; do
    WAITED=$((WAITED + 1))
    if [ "$WAITED" -ge "$MAX_WAIT" ]; then warn "后端未响应 /health，继续"; break; fi
    printf '.'; sleep 1
done
echo
ok "后端已启动 (PID $SERVER_PID) → logs/server.log ✓"

echo
echo -e "${BOLD}${GREEN}═══════════════════════════════${RESET}"
echo -e "${BOLD}${GREEN}  农趣村后端已启动 🌾${RESET}"
echo -e "${BOLD}${GREEN}═══════════════════════════════${RESET}"
echo -e "  后端 API   → ${CYAN}http://localhost:${SERVER_PORT:-9080}${RESET}"
echo -e "  数据库管理 → ${CYAN}http://localhost:9081${RESET}  (Adminer)"
echo -e "  后端日志   → ${YELLOW}tail -f logs/server.log${RESET}"
echo -e "  停止服务   → ${RED}./stop_service.sh${RESET}"
echo
