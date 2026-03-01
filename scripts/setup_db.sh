#!/usr/bin/env bash
# setup_db.sh — 数据库初始化脚本
#
# 用法：
#   ./scripts/setup_db.sh          # 仅迁移（表不存在则创建）
#   ./scripts/setup_db.sh --reset  # 先删库重建，再迁移（清空所有数据）
#
# 依赖：Docker Compose 已启动（postgres 容器在运行）
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ── 颜色 ──────────────────────────────────────────────────────────────────────
RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
CYAN='\033[0;36m'; BOLD='\033[1m'; RESET='\033[0m'

log()  { echo -e "${CYAN}[db]${RESET} $*"; }
ok()   { echo -e "${GREEN}[db]${RESET} $*"; }
warn() { echo -e "${YELLOW}[db]${RESET} $*"; }
err()  { echo -e "${RED}[db]${RESET} $*" >&2; exit 1; }

RESET_MODE=false
[[ "${1:-}" == "--reset" ]] && RESET_MODE=true

# ── 读取 .env ─────────────────────────────────────────────────────────────────
ENV_FILE="$ROOT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
    set -a
    # shellcheck source=/dev/null
    source <(grep -v '^\s*#' "$ENV_FILE" | grep -v '^\s*$')
    set +a
fi

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5433}"
DB_USER="${DB_USER:-farmer}"
DB_PASSWORD="${DB_PASSWORD:-farmer_secret}"
DB_NAME="${DB_NAME:-farmer_dev}"
DATABASE_DSN="${DATABASE_DSN:-postgres://$DB_USER:$DB_PASSWORD@$DB_HOST:$DB_PORT/$DB_NAME?sslmode=disable}"

# ── 1. 等待 PostgreSQL 就绪 ───────────────────────────────────────────────────
log "等待 PostgreSQL 就绪 ($DB_HOST:$DB_PORT)…"
MAX_WAIT=30
WAITED=0
until docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T postgres \
    pg_isready -U "$DB_USER" -d postgres &>/dev/null; do
    WAITED=$((WAITED + 1))
    [ "$WAITED" -ge "$MAX_WAIT" ] && err "PostgreSQL ${MAX_WAIT}s 内未就绪，请先运行 docker compose up -d"
    printf '.'
    sleep 1
done
echo
ok "PostgreSQL 就绪 ✓"

# ── 2. --reset 模式：删除并重建数据库 ────────────────────────────────────────
psql() {
    docker compose -f "$ROOT_DIR/docker-compose.yml" exec -T postgres \
        psql -U "$DB_USER" "$@"
}

if $RESET_MODE; then
    warn "⚠ RESET 模式：即将删除数据库 '$DB_NAME'，所有数据将丢失！"
    read -r -p "确认继续？[y/N] " confirm
    [[ "$confirm" =~ ^[Yy]$ ]] || { log "已取消"; exit 0; }

    log "断开所有连接…"
    psql -d postgres -c \
        "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();" \
        > /dev/null

    log "删除数据库 $DB_NAME…"
    psql -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;"
    ok "已删除 ✓"
fi

# ── 3. 创建数据库（不存在时） ─────────────────────────────────────────────────
DB_EXISTS=$(psql -d postgres -tAc \
    "SELECT 1 FROM pg_database WHERE datname = '$DB_NAME';" 2>/dev/null || echo "")

if [ "$DB_EXISTS" != "1" ]; then
    log "创建数据库 $DB_NAME…"
    psql -d postgres -c "CREATE DATABASE $DB_NAME OWNER $DB_USER ENCODING 'UTF8';"
    ok "数据库已创建 ✓"
else
    log "数据库 $DB_NAME 已存在，跳过创建"
fi

# ── 4. 运行 Ent 迁移（建表 / 更新表结构） ─────────────────────────────────────
log "运行 Ent Schema 迁移（28 张表）…"
cd "$ROOT_DIR/server"
DATABASE_DSN="$DATABASE_DSN" go run ./cmd/migrate
ok "迁移完成 ✓"

# ── 5. 打印表清单 ─────────────────────────────────────────────────────────────
log "当前数据库表清单："
psql -d "$DB_NAME" -c "\dt" 2>/dev/null | grep -E "^\s(public|farmer)" || true

echo
echo -e "${BOLD}${GREEN}数据库初始化完成 ✓${RESET}"
echo -e "  DSN: ${CYAN}$DATABASE_DSN${RESET}"
echo -e "  用 DBeaver 连接: ${CYAN}$DB_HOST:$DB_PORT / $DB_NAME${RESET}"
