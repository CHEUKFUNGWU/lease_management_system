#!/usr/bin/env bash
# 起一个一次性 PostgreSQL、灌入空库基线、跑集成测试、无论成败都销毁。
#
# 为什么需要它：跨法人隔离（底线 1）与幂等（底线 4）的证据大多在集成测试里，
# 而这些测试在 TEST_DATABASE_URL 未设置时静默 skip —— skip 掉的测试证明不了
# 任何事，却看起来是绿的。开发机上 5432 常被宿主机原生 postgres 或既有的
# lease-postgres 卷占着（后者曾用另一套凭据初始化，.env 里的 lease/lease_secret
# 连过去会得到 role "lease" does not exist），于是每个人各自手起容器绕过。
# 这个脚本把那套手工步骤收敛成一条命令。
#
# 用法：
#   scripts/test_with_db.sh                      # 跑全部
#   scripts/test_with_db.sh ./internal/handlers/ # 跑指定包
#   scripts/test_with_db.sh ./... -run TestFoo   # 透传 go test 参数
#
# 它不碰既有的 lease-postgres 容器与卷。容器名与端口都是独立的。

set -euo pipefail

CONTAINER="${TEST_DB_CONTAINER:-lease-testdb}"
PORT="${TEST_DB_PORT:-5544}"
IMAGE="postgres:16-alpine"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if ! docker info >/dev/null 2>&1; then
  echo "错误：Docker 未运行。先启动 Docker Desktop。" >&2
  exit 1
fi

if lsof -nP -iTCP:"$PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  echo "错误：端口 $PORT 已被占用。用 TEST_DB_PORT=<其他端口> 重试。" >&2
  exit 1
fi

cleanup() {
  local code=$?
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  exit $code
}
trap cleanup EXIT INT TERM

docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
echo "→ 起一次性数据库 ${CONTAINER}（端口 ${PORT}）"
docker run -d --name "$CONTAINER" \
  -e POSTGRES_USER=lease -e POSTGRES_PASSWORD=lease_secret -e POSTGRES_DB=lease \
  -p "$PORT":5432 "$IMAGE" >/dev/null

for _ in $(seq 1 40); do
  docker exec "$CONTAINER" pg_isready -U lease >/dev/null 2>&1 && break
  sleep 2
done
docker exec "$CONTAINER" pg_isready -U lease >/dev/null 2>&1 || {
  echo "错误：数据库未在预期时间内就绪。" >&2
  exit 1
}

echo "→ 加载 db/init/01_init.sql"
# init 脚本自带 schema_migrations 基线，空库会被标记为全部迁移已应用，
# 与 db/migrations/ 增量迁移的关系见 AGENTS.md「关键设计决策」。
if ! docker exec -i "$CONTAINER" psql -U lease -d lease -q -v ON_ERROR_STOP=1 \
    < "$ROOT/db/init/01_init.sql" 2>&1 | grep -iE '^(ERROR|FATAL)' ; then
  : # grep 无命中即无错误
fi

TABLES="$(docker exec "$CONTAINER" psql -U lease -d lease -tAc \
  "select count(*) from information_schema.tables where table_schema='public'")"
echo "→ 已加载 $TABLES 张表"

export TEST_DATABASE_URL="postgres://lease:lease_secret@localhost:$PORT/lease?sslmode=disable"
echo "→ TEST_DATABASE_URL 已导出，开始跑测试"
echo

cd "$ROOT/core-service"
GOCACHE="$(pwd)/.gocache" go test "${@:-./...}"
